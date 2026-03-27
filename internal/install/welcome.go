package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/web-casa/llstack/internal/system"
)

const defaultWelcomeSiteName = "default"

// WelcomePageConfig controls welcome page generation.
type WelcomePageConfig struct {
	SitesRoot  string // /data/www
	Backend    string
	PHPVersion string
	ServerIP   string
}

// SetupWelcomePage creates the default welcome site with PHP probe and Adminer.
func SetupWelcomePage(ctx context.Context, exec system.Executor, cfg WelcomePageConfig) error {
	docroot := filepath.Join(cfg.SitesRoot, defaultWelcomeSiteName)
	if err := os.MkdirAll(docroot, 0o755); err != nil {
		return fmt.Errorf("create welcome docroot: %w", err)
	}

	// Generate index.php
	indexContent := generateWelcomeIndex(cfg)
	if err := os.WriteFile(filepath.Join(docroot, "index.php"), []byte(indexContent), 0o644); err != nil {
		return err
	}

	// Download x-prober
	proberURL := "https://github.com/kmvan/x-prober/releases/latest/download/dist.php"
	proberPath := filepath.Join(docroot, "prober.php")
	downloadFile(ctx, exec, proberURL, proberPath)

	// Download Adminer
	adminerURL := "https://github.com/vrana/adminer/releases/latest/download/adminer-4.8.4.php"
	adminerPath := filepath.Join(docroot, "adminer.php")
	downloadFile(ctx, exec, adminerURL, adminerPath)

	// Set permissions
	exec.Run(ctx, system.Command{Name: "chown", Args: []string{"-R", "apache:apache", docroot}})

	// Configure as default vhost (listen on IP directly)
	if cfg.Backend == "apache" {
		vhostContent := generateDefaultApacheVhost(docroot, cfg.PHPVersion)
		vhostPath := "/etc/httpd/conf.d/00-llstack-welcome.conf"
		os.WriteFile(vhostPath, []byte(vhostContent), 0o644)
		exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"reload", "httpd"}})
	}

	return nil
}

// RemoveWelcomePage removes the default welcome site.
func RemoveWelcomePage(ctx context.Context, exec system.Executor, sitesRoot string, backend string) error {
	docroot := filepath.Join(sitesRoot, defaultWelcomeSiteName)
	os.RemoveAll(docroot)
	if backend == "apache" {
		os.Remove("/etc/httpd/conf.d/00-llstack-welcome.conf")
		exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"reload", "httpd"}})
	}
	return nil
}

func generateWelcomeIndex(cfg WelcomePageConfig) string {
	ip := cfg.ServerIP
	if ip == "" {
		ip = "YOUR_SERVER_IP"
	}

	// Sanitize IP to prevent PHP injection
	safeIP := strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || r == '.' || r == ':' {
			return r
		}
		return -1
	}, ip)

	return fmt.Sprintf(`<?php
$ip = '%s';
if (empty($ip) || $ip === 'YOUR_SERVER_IP') {
    $ip = $_SERVER['SERVER_ADDR'] ?? gethostbyname(gethostname());
}
?>
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>LLStack - Installation Successful</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #f5f7fa; color: #333; min-height: 100vh; display: flex; flex-direction: column; align-items: center; padding: 40px 20px; }
.container { max-width: 720px; width: 100%%; }
.header { text-align: center; margin-bottom: 40px; }
.header h1 { font-size: 2.5em; color: #2d3748; margin-bottom: 8px; }
.header p { font-size: 1.1em; color: #718096; }
.card { background: #fff; border-radius: 12px; padding: 24px; margin-bottom: 20px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
.card h2 { font-size: 1.2em; color: #2d3748; margin-bottom: 16px; border-bottom: 2px solid #e2e8f0; padding-bottom: 8px; }
.info-grid { display: grid; grid-template-columns: 140px 1fr; gap: 8px; }
.info-label { font-weight: 600; color: #4a5568; }
.info-value { color: #2d3748; }
.tools { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.tool-link { display: block; padding: 16px; background: #edf2f7; border-radius: 8px; text-decoration: none; color: #2d3748; text-align: center; transition: background 0.2s; }
.tool-link:hover { background: #e2e8f0; }
.tool-link .icon { font-size: 1.5em; margin-bottom: 4px; }
.tool-link .label { font-weight: 600; }
.tool-link .desc { font-size: 0.85em; color: #718096; }
.cli { background: #1a202c; color: #e2e8f0; border-radius: 8px; padding: 16px; font-family: 'SF Mono', Monaco, monospace; font-size: 0.9em; overflow-x: auto; }
.cli code { display: block; margin: 4px 0; }
.cli .prompt { color: #68d391; }
.cli .comment { color: #718096; }
.warning { background: #fffbeb; border: 1px solid #f6e05e; border-radius: 8px; padding: 16px; margin-top: 20px; }
.warning strong { color: #d69e2e; }
</style>
</head>
<body>
<div class="container">
    <div class="header">
        <h1>LLStack</h1>
        <p>Web Stack Installation Successful</p>
    </div>

    <div class="card">
        <h2>Server Information</h2>
        <div class="info-grid">
            <span class="info-label">Server IP:</span>
            <span class="info-value"><?= htmlspecialchars($ip) ?></span>
            <span class="info-label">OS:</span>
            <span class="info-value"><?= php_uname('s') . ' ' . php_uname('r') ?></span>
            <span class="info-label">PHP Version:</span>
            <span class="info-value"><?= PHP_VERSION ?></span>
            <span class="info-label">Web Server:</span>
            <span class="info-value"><?= $_SERVER['SERVER_SOFTWARE'] ?? 'Unknown' ?></span>
            <span class="info-label">Hostname:</span>
            <span class="info-value"><?= gethostname() ?></span>
        </div>
    </div>

    <div class="card">
        <h2>Tools</h2>
        <div class="tools">
            <a href="/prober.php" class="tool-link">
                <div class="icon">🔍</div>
                <div class="label">PHP X-Prober</div>
                <div class="desc">Server status &amp; PHP info</div>
            </a>
            <a href="/adminer.php" class="tool-link">
                <div class="icon">🗄️</div>
                <div class="label">Adminer</div>
                <div class="desc">Database management</div>
            </a>
        </div>
    </div>

    <div class="card">
        <h2>Quick Start</h2>
        <div class="cli">
            <code><span class="comment"># Add a website</span></code>
            <code><span class="prompt">$</span> llstack site:create example.com --backend apache --profile wordpress</code>
            <code></code>
            <code><span class="comment"># Open TUI interface</span></code>
            <code><span class="prompt">$</span> llstack tui</code>
            <code></code>
            <code><span class="comment"># Run diagnostics</span></code>
            <code><span class="prompt">$</span> llstack doctor</code>
            <code></code>
            <code><span class="comment"># Show all commands</span></code>
            <code><span class="prompt">$</span> llstack --help</code>
            <code></code>
            <code><span class="comment"># Show tuning recommendations</span></code>
            <code><span class="prompt">$</span> llstack tune</code>
            <code></code>
            <code><span class="comment"># Remove this welcome page</span></code>
            <code><span class="prompt">$</span> llstack welcome:remove</code>
        </div>
    </div>

    <div class="warning">
        <strong>⚠ Security Notice:</strong>
        This welcome page, PHP probe, and Adminer are for initial verification only.
        Please remove them after confirming your installation works:
        <br><br>
        <code>llstack welcome:remove</code>
    </div>
</div>
</body>
</html>
`, safeIP)
}

func generateDefaultApacheVhost(docroot string, phpVersion string) string {
	fpmSocket := ""
	if phpVersion != "" {
		vTag := strings.ReplaceAll(phpVersion, ".", "")
		fpmSocket = fmt.Sprintf("/var/opt/remi/php%s/run/php-fpm/www.sock", vTag)
	}

	lines := []string{
		"# LLStack welcome page - remove with: llstack welcome:remove",
		"<VirtualHost *:80>",
		fmt.Sprintf("    DocumentRoot %s", docroot),
		"    DirectoryIndex index.php index.html",
		fmt.Sprintf("    <Directory %s>", docroot),
		"        AllowOverride All",
		"        Require all granted",
		"    </Directory>",
	}
	if fpmSocket != "" {
		lines = append(lines,
			"    <FilesMatch \\.php$>",
			fmt.Sprintf("        SetHandler \"proxy:unix:%s|fcgi://localhost\"", fpmSocket),
			"    </FilesMatch>",
		)
	}
	lines = append(lines, "</VirtualHost>")
	return strings.Join(lines, "\n") + "\n"
}

func downloadFile(ctx context.Context, exec system.Executor, url, dest string) {
	// Try curl first, then wget
	exec.Run(ctx, system.Command{
		Name: "curl",
		Args: []string{"-fsSL", "-o", dest, url},
	})
	if _, err := os.Stat(dest); err != nil {
		exec.Run(ctx, system.Command{
			Name: "wget",
			Args: []string{"-qO", dest, url},
		})
	}
}

// DetectServerIP returns the primary IP address.
func DetectServerIP(ctx context.Context, exec system.Executor) string {
	result, err := exec.Run(ctx, system.Command{
		Name: "hostname",
		Args: []string{"-I"},
	})
	if err == nil && result.ExitCode == 0 {
		fields := strings.Fields(result.Stdout)
		if len(fields) > 0 {
			return fields[0]
		}
	}
	return ""
}
