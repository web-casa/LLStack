package security

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/web-casa/llstack/internal/core/plan"
	"github.com/web-casa/llstack/internal/system"
)

// Manager manages security features (fail2ban, IP blocking, rate limiting, firewalld).
type Manager struct {
	exec system.Executor
}

// NewManager creates a security manager.
func NewManager(exec system.Executor) Manager {
	return Manager{exec: exec}
}

// --- fail2ban ---

// Fail2banEnable installs and configures fail2ban with LLStack default jails.
func (m Manager) Fail2banEnable(ctx context.Context, dryRun bool) (plan.Plan, error) {
	p := plan.New("security.fail2ban.enable", "Enable fail2ban with LLStack jails")
	p.DryRun = dryRun

	p.AddOperation(plan.Operation{ID: "install-fail2ban", Kind: "package.install", Target: "fail2ban"})
	p.AddOperation(plan.Operation{ID: "write-jail-config", Kind: "file.write", Target: "/etc/fail2ban/jail.d/llstack.conf"})
	p.AddOperation(plan.Operation{ID: "enable-fail2ban", Kind: "service.enable", Target: "fail2ban"})

	if dryRun {
		return p, nil
	}

	m.exec.Run(ctx, system.Command{Name: "dnf", Args: []string{"-y", "install", "fail2ban"}})

	jailConfig := `# Managed by LLStack
[sshd]
enabled = true
maxretry = 5
bantime = 3600
findtime = 600

[llstack-http-auth]
enabled = true
port = http,https
filter = apache-auth
logpath = /var/log/llstack/sites/*.error.log
maxretry = 5
bantime = 3600
findtime = 600
`
	if err := writeFileWithDir("/etc/fail2ban/jail.d/llstack.conf", jailConfig); err != nil {
		return p, err
	}

	m.exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"enable", "--now", "fail2ban"}})
	return p, nil
}

// Fail2banStatus returns current fail2ban status.
func (m Manager) Fail2banStatus(ctx context.Context) (string, error) {
	result, err := m.exec.Run(ctx, system.Command{Name: "fail2ban-client", Args: []string{"status"}})
	if err != nil {
		return "", fmt.Errorf("fail2ban-client: %w", err)
	}
	return result.Stdout, nil
}

// Fail2banUnban unbans an IP address.
func (m Manager) Fail2banUnban(ctx context.Context, ip string) error {
	_, err := m.exec.Run(ctx, system.Command{Name: "fail2ban-client", Args: []string{"unban", ip}})
	return err
}

// --- IP Blocking ---

// BlockIP blocks an IP via firewalld rich rule.
func (m Manager) BlockIP(ctx context.Context, ip string, dryRun bool) (plan.Plan, error) {
	// Validate IP format to prevent firewall rule injection
	if !isValidIP(ip) {
		return plan.Plan{}, fmt.Errorf("invalid IP address format: %q", ip)
	}

	p := plan.New("security.block", fmt.Sprintf("Block IP %s", ip))
	p.DryRun = dryRun
	p.AddOperation(plan.Operation{ID: "block-ip", Kind: "firewall.rich-rule", Target: ip})

	if dryRun {
		return p, nil
	}

	rule := buildDropRule(ip)
	result, err := m.exec.Run(ctx, system.Command{
		Name: "firewall-cmd",
		Args: []string{"--permanent", "--add-rich-rule=" + rule},
	})
	if err != nil {
		return p, fmt.Errorf("firewall-cmd: %w", err)
	}
	if result.ExitCode != 0 {
		return p, fmt.Errorf("firewall-cmd: exit %d: %s", result.ExitCode, result.Stderr)
	}
	m.exec.Run(ctx, system.Command{Name: "firewall-cmd", Args: []string{"--reload"}})
	return p, nil
}

// UnblockIP removes an IP block.
func (m Manager) UnblockIP(ctx context.Context, ip string) error {
	rule := buildDropRule(ip)
	m.exec.Run(ctx, system.Command{
		Name: "firewall-cmd",
		Args: []string{"--permanent", "--remove-rich-rule=" + rule},
	})
	m.exec.Run(ctx, system.Command{Name: "firewall-cmd", Args: []string{"--reload"}})
	return nil
}

// BlockList returns currently blocked IPs.
func (m Manager) BlockList(ctx context.Context) ([]string, error) {
	result, err := m.exec.Run(ctx, system.Command{
		Name: "firewall-cmd",
		Args: []string{"--list-rich-rules"},
	})
	if err != nil {
		return nil, err
	}
	var blocked []string
	for _, line := range strings.Split(result.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "drop") && strings.Contains(line, "source address") {
			// Extract IP from: rule family='ipv4' source address='1.2.3.4' drop
			parts := strings.Split(line, "'")
			for i, part := range parts {
				if strings.Contains(part, "source address=") && i+1 < len(parts) {
					blocked = append(blocked, parts[i+1])
				}
			}
		}
	}
	return blocked, nil
}

// --- Rate Limiting ---

// RateLimitConfig holds unified rate limiting parameters.
type RateLimitConfig struct {
	MaxRequestsPerSecond int
	BurstSize            int
}

// EnableRateLimit configures per-backend rate limiting.
func (m Manager) EnableRateLimit(ctx context.Context, backend string, cfg RateLimitConfig, dryRun bool) (plan.Plan, error) {
	p := plan.New("security.ratelimit", fmt.Sprintf("Enable rate limiting (%d req/s)", cfg.MaxRequestsPerSecond))
	p.DryRun = dryRun

	if cfg.MaxRequestsPerSecond <= 0 {
		cfg.MaxRequestsPerSecond = 10
	}
	if cfg.BurstSize <= 0 {
		cfg.BurstSize = 50
	}

	switch backend {
	case "apache":
		p.AddOperation(plan.Operation{
			ID: "install-mod-evasive", Kind: "package.install", Target: "mod_evasive",
		})
		p.AddOperation(plan.Operation{
			ID: "write-evasive-config", Kind: "file.write", Target: "/etc/httpd/conf.d/llstack-ratelimit.conf",
		})
	case "ols", "lsws":
		p.AddOperation(plan.Operation{
			ID: "write-ols-ratelimit", Kind: "config.update", Target: "perClientConnLimit",
			Details: map[string]string{
				"dynReqPerSec":    fmt.Sprintf("%d", cfg.MaxRequestsPerSecond),
				"staticReqPerSec": fmt.Sprintf("%d", cfg.MaxRequestsPerSecond),
			},
		})
	}

	if dryRun {
		return p, nil
	}

	switch backend {
	case "apache":
		m.exec.Run(ctx, system.Command{Name: "dnf", Args: []string{"-y", "install", "mod_evasive"}})
		evasiveConfig := fmt.Sprintf(`# Managed by LLStack
<IfModule mod_evasive24.c>
    DOSHashTableSize 3097
    DOSPageCount %d
    DOSSiteCount %d
    DOSPageInterval 1
    DOSSiteInterval 1
    DOSBlockingPeriod 60
</IfModule>
`, cfg.MaxRequestsPerSecond, cfg.BurstSize)
		writeFileWithDir("/etc/httpd/conf.d/llstack-ratelimit.conf", evasiveConfig)
		m.exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"reload", "httpd"}})
	case "ols", "lsws":
		olsConfig := fmt.Sprintf(`# Managed by LLStack rate limiting
perClientConnLimit {
  staticReqPerSec         %d
  dynReqPerSec            %d
  outBandwidth            0
  inBandwidth             0
  softLimit               %d
  hardLimit               %d
  gracePeriod             15
  banPeriod               60
}
`, cfg.MaxRequestsPerSecond, cfg.MaxRequestsPerSecond, cfg.BurstSize, cfg.BurstSize*2)
		writeFileWithDir("/usr/local/lsws/conf/llstack/ratelimit.conf", olsConfig)
		// Note: OLS/LSWS need to include this file or have perClientConnLimit in main config
		// For now we write the config; operator includes it or LLStack configmanager integrates it
		m.exec.Run(ctx, system.Command{Name: "lswsctrl", Args: []string{"restart"}})
	}

	return p, nil
}

// --- Firewall Management ---

// FirewallStatus returns current firewalld status summary.
func (m Manager) FirewallStatus(ctx context.Context) (map[string]string, error) {
	info := map[string]string{}

	result, err := m.exec.Run(ctx, system.Command{Name: "firewall-cmd", Args: []string{"--state"}})
	if err != nil {
		info["state"] = "unavailable"
		return info, nil
	}
	info["state"] = strings.TrimSpace(result.Stdout)

	result, _ = m.exec.Run(ctx, system.Command{Name: "firewall-cmd", Args: []string{"--list-ports"}})
	info["ports"] = strings.TrimSpace(result.Stdout)

	result, _ = m.exec.Run(ctx, system.Command{Name: "firewall-cmd", Args: []string{"--list-services"}})
	info["services"] = strings.TrimSpace(result.Stdout)

	result, _ = m.exec.Run(ctx, system.Command{Name: "firewall-cmd", Args: []string{"--get-active-zones"}})
	info["zones"] = strings.TrimSpace(result.Stdout)

	return info, nil
}

// FirewallOpenPort opens a port.
func (m Manager) FirewallOpenPort(ctx context.Context, port string) error {
	if !strings.Contains(port, "/") {
		port += "/tcp"
	}
	result, err := m.exec.Run(ctx, system.Command{
		Name: "firewall-cmd",
		Args: []string{"--permanent", "--add-port=" + port},
	})
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("firewall-cmd: %s", result.Stderr)
	}
	m.exec.Run(ctx, system.Command{Name: "firewall-cmd", Args: []string{"--reload"}})
	return nil
}

// FirewallClosePort closes a port.
func (m Manager) FirewallClosePort(ctx context.Context, port string) error {
	if !strings.Contains(port, "/") {
		port += "/tcp"
	}
	m.exec.Run(ctx, system.Command{
		Name: "firewall-cmd",
		Args: []string{"--permanent", "--remove-port=" + port},
	})
	m.exec.Run(ctx, system.Command{Name: "firewall-cmd", Args: []string{"--reload"}})
	return nil
}

// --- helpers ---

func buildDropRule(ip string) string {
	family := "ipv4"
	if strings.Contains(ip, ":") {
		family = "ipv6"
	}
	return fmt.Sprintf("rule family='%s' source address='%s' drop", family, ip)
}

func isValidIP(ip string) bool {
	return net.ParseIP(strings.TrimSpace(ip)) != nil
}

func writeFileWithDir(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
