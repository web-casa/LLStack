package ssl

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/core/plan"
	"github.com/web-casa/llstack/internal/system"
)

// CertStatus captures the state of a site's TLS certificate.
type CertStatus struct {
	Site       string    `json:"site"`
	CertFile   string    `json:"cert_file,omitempty"`
	Issuer     string    `json:"issuer,omitempty"`
	NotAfter   time.Time `json:"not_after,omitempty"`
	DaysLeft   int       `json:"days_left"`
	Status     string    `json:"status"` // ok, expiring, expired, missing, error
}

// LifecycleManager manages SSL certificate lifecycle.
type LifecycleManager struct {
	cfg  config.RuntimeConfig
	exec system.Executor
}

// NewLifecycleManager creates an SSL lifecycle manager.
func NewLifecycleManager(cfg config.RuntimeConfig, exec system.Executor) LifecycleManager {
	return LifecycleManager{cfg: cfg, exec: exec}
}

// Status returns certificate status for all managed sites with TLS enabled.
func (m LifecycleManager) Status(sites []SiteInfo) []CertStatus {
	var results []CertStatus
	for _, site := range sites {
		results = append(results, m.checkCert(site))
	}
	return results
}

// SiteInfo is a minimal site reference for SSL operations.
type SiteInfo struct {
	Name     string
	CertFile string
	Domain   string
	Docroot  string
}

func (m LifecycleManager) checkCert(site SiteInfo) CertStatus {
	if strings.TrimSpace(site.CertFile) == "" {
		return CertStatus{Site: site.Name, Status: "missing"}
	}
	raw, err := os.ReadFile(site.CertFile)
	if err != nil {
		return CertStatus{Site: site.Name, CertFile: site.CertFile, Status: "missing"}
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return CertStatus{Site: site.Name, CertFile: site.CertFile, Status: "error"}
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return CertStatus{Site: site.Name, CertFile: site.CertFile, Status: "error"}
	}

	daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)
	status := "ok"
	if daysLeft <= 0 {
		status = "expired"
	} else if daysLeft <= 14 {
		status = "expiring"
	}

	issuer := cert.Issuer.CommonName
	if issuer == "" && len(cert.Issuer.Organization) > 0 {
		issuer = cert.Issuer.Organization[0]
	}

	return CertStatus{
		Site:     site.Name,
		CertFile: site.CertFile,
		Issuer:   issuer,
		NotAfter: cert.NotAfter,
		DaysLeft: daysLeft,
		Status:   status,
	}
}

// Renew attempts to renew a certificate using certbot.
func (m LifecycleManager) Renew(ctx context.Context, site SiteInfo, email string, dryRun bool) (plan.Plan, error) {
	p := plan.New("ssl.renew", fmt.Sprintf("Renew TLS certificate for %s", site.Name))
	p.DryRun = dryRun

	info := DetectCertbot(m.cfg)
	if !info.Found {
		return plan.Plan{}, fmt.Errorf("certbot binary not detected")
	}

	cmd, err := BuildCertbotCommand(info, CertbotRequest{
		Domain:  site.Domain,
		Docroot: site.Docroot,
		Email:   email,
	})
	if err != nil {
		return plan.Plan{}, err
	}

	p.AddOperation(plan.Operation{
		ID:     "renew-cert-" + site.Name,
		Kind:   "ssl.certbot",
		Target: site.Domain,
		Details: map[string]string{
			"certbot": info.Binary,
			"email":   email,
		},
	})

	if dryRun {
		return p, nil
	}

	result, err := m.exec.Run(ctx, cmd)
	if err != nil {
		return p, fmt.Errorf("certbot failed: %w", err)
	}
	if result.ExitCode != 0 {
		return p, fmt.Errorf("certbot exited %d: %s", result.ExitCode, result.Stderr)
	}

	return p, nil
}

// AutoRenewTimerUnit returns the systemd timer unit content for auto-renewal.
func AutoRenewTimerUnit() string {
	return `[Unit]
Description=LLStack SSL Auto-Renewal Timer

[Timer]
OnCalendar=daily
Persistent=true
RandomizedDelaySec=3600

[Install]
WantedBy=timers.target
`
}

// AutoRenewServiceUnit returns the systemd service unit content.
func AutoRenewServiceUnit(llstackBin string) string {
	return fmt.Sprintf(`[Unit]
Description=LLStack SSL Auto-Renewal

[Service]
Type=oneshot
ExecStart=%s ssl:renew --all --non-interactive
`, llstackBin)
}

// EnableAutoRenew installs and enables the systemd timer.
func (m LifecycleManager) EnableAutoRenew(ctx context.Context) error {
	llstackBin, err := exec.LookPath("llstack")
	if err != nil {
		llstackBin = "/usr/local/bin/llstack"
	}

	timerPath := "/etc/systemd/system/llstack-ssl-renew.timer"
	servicePath := "/etc/systemd/system/llstack-ssl-renew.service"

	if err := os.WriteFile(timerPath, []byte(AutoRenewTimerUnit()), 0o644); err != nil {
		return fmt.Errorf("write timer unit: %w", err)
	}
	if err := os.WriteFile(servicePath, []byte(AutoRenewServiceUnit(llstackBin)), 0o644); err != nil {
		return fmt.Errorf("write service unit: %w", err)
	}

	m.exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"daemon-reload"}})
	m.exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"enable", "--now", "llstack-ssl-renew.timer"}})

	return nil
}

// DisableAutoRenew stops and removes the systemd timer.
func (m LifecycleManager) DisableAutoRenew(ctx context.Context) error {
	m.exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"disable", "--now", "llstack-ssl-renew.timer"}})
	os.Remove("/etc/systemd/system/llstack-ssl-renew.timer")
	os.Remove("/etc/systemd/system/llstack-ssl-renew.service")
	m.exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"daemon-reload"}})
	return nil
}

// ExpiryThresholdDays is the default days-before-expiry to trigger warning/renewal.
var ExpiryThresholdDays = 14

// CertsExpiringSoon returns sites with certificates expiring within threshold days.
func CertsExpiringSoon(statuses []CertStatus, thresholdDays int) []CertStatus {
	var expiring []CertStatus
	for _, s := range statuses {
		if s.Status == "expiring" || s.Status == "expired" || (s.DaysLeft > 0 && s.DaysLeft <= thresholdDays) {
			expiring = append(expiring, s)
		}
	}
	return expiring
}

// RenewAllOptions controls batch renewal.
type RenewAllOptions struct {
	Email        string
	ThresholdDays int
	DryRun       bool
}

// RenewExpiring renews all certificates expiring within threshold.
func (m LifecycleManager) RenewExpiring(ctx context.Context, sites []SiteInfo, opts RenewAllOptions) ([]plan.Plan, error) {
	threshold := opts.ThresholdDays
	if threshold <= 0 {
		threshold = ExpiryThresholdDays
	}

	statuses := m.Status(sites)
	expiring := CertsExpiringSoon(statuses, threshold)

	var plans []plan.Plan
	for _, cert := range expiring {
		// Find site info
		for _, site := range sites {
			if site.Name == cert.Site {
				p, err := m.Renew(ctx, site, opts.Email, opts.DryRun)
				if err != nil {
					return plans, fmt.Errorf("renew %s: %w", site.Name, err)
				}
				plans = append(plans, p)
				break
			}
		}
	}
	return plans, nil
}

// CertSummaryDir returns the default path for certificate status cache.
func CertSummaryDir(cfg config.RuntimeConfig) string {
	return filepath.Join(cfg.Paths.StateDir, "ssl")
}
