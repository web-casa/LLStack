package lsws

import (
	"fmt"
	"strings"

	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/core/render"
)

// Renderer renders Apache-style LSWS configs with LiteSpeed extensions.
type Renderer struct {
	cfg     config.RuntimeConfig
	license LicenseInfo
}

// NewRenderer constructs a LSWS renderer.
func NewRenderer(cfg config.RuntimeConfig, license LicenseInfo) Renderer {
	return Renderer{cfg: cfg, license: license}
}

// RenderSite renders a site into LSWS-managed assets.
func (r Renderer) RenderSite(site model.Site, opts render.SiteRenderOptions) (render.SiteRenderResult, error) {
	parity := buildParityReport(site, r.license)
	capabilities := buildCapabilities(r.license)

	assets := []render.Asset{
		{
			Path:    opts.VHostPath,
			Content: []byte(renderLSWSConfig(site, r.cfg.LSWS.DefaultPHPBinary, capabilities, opts.SystemUser) + "\n"),
			Mode:    0o644,
		},
	}
	if opts.ParityReportPath != "" {
		raw, err := parity.JSON()
		if err != nil {
			return render.SiteRenderResult{}, err
		}
		assets = append(assets, render.Asset{
			Path:    opts.ParityReportPath,
			Content: append(raw, '\n'),
			Mode:    0o644,
		})
	}

	warnings := []string{fmt.Sprintf("lsws license mode: %s", r.license.Mode)}
	for _, note := range capabilities.Notes {
		warnings = append(warnings, note)
	}

	return render.SiteRenderResult{
		Assets:       assets,
		Warnings:     warnings,
		ParityReport: &parity,
		Capabilities: &capabilities,
	}, nil
}

func renderLSWSConfig(site model.Site, phpBinary string, capabilities model.BackendCapabilities, systemUser string) string {
	lines := []string{
		"# Managed by LLStack for LiteSpeed Enterprise",
		"<VirtualHost *:80>",
		fmt.Sprintf("    ServerName %s", site.Domain.ServerName),
	}
	for _, alias := range site.Domain.Aliases {
		lines = append(lines, fmt.Sprintf("    ServerAlias %s", alias))
	}
	lines = append(lines,
		fmt.Sprintf("    DocumentRoot %s", site.DocumentRoot),
		fmt.Sprintf("    DirectoryIndex %s", strings.Join(site.IndexFiles, " ")),
		fmt.Sprintf("    ErrorLog %s", site.Logs.ErrorLog),
		fmt.Sprintf("    CustomLog %s combined", site.Logs.AccessLog),
	)
	if site.TLS.Enabled {
		lines = append(lines,
			"    RewriteEngine On",
			fmt.Sprintf("    RewriteRule ^ https://%s%%{REQUEST_URI} [R=301,L]", site.Domain.ServerName),
		)
	}
	lines = append(lines,
		fmt.Sprintf("    <Directory %s>", site.DocumentRoot),
		"        AllowOverride All",
		"        Require all granted",
		"    </Directory>",
	)
	if site.PHP.Enabled {
		phpLines := []string{
			"    <IfModule LiteSpeed>",
			"    LSPHP_ProcessGroup on",
			"    LSPHP_Workers 10",
			fmt.Sprintf("    LSPHP_Command %s", lsphpCommand(site, phpBinary)),
		}
		if systemUser != "" {
			phpLines = append(phpLines,
				fmt.Sprintf("    php_admin_value open_basedir %s:/tmp:/usr/share/php", site.DocumentRoot),
			)
		}
		phpLines = append(phpLines, "    </IfModule>")
		phpLines = append(phpLines,
			"    <FilesMatch \\.php$>",
			fmt.Sprintf("        SetHandler \"proxy:unix:/tmp/lshttpd/%s.sock|fcgi://localhost\"", site.Name),
			"    </FilesMatch>",
		)
		lines = append(lines, phpLines...)
	}
	for _, rule := range site.RewriteRules {
		flags := ""
		if len(rule.Flags) > 0 {
			flags = " [" + strings.Join(rule.Flags, ",") + "]"
		}
		lines = append(lines, fmt.Sprintf("    RewriteRule %s %s%s", rule.Pattern, rule.Substitution, flags))
	}
	for _, rule := range site.ReverseProxyRules {
		lines = append(lines,
			fmt.Sprintf("    ProxyPass %s %s", rule.PathPrefix, rule.Upstream),
			fmt.Sprintf("    ProxyPassReverse %s %s", rule.PathPrefix, rule.Upstream),
		)
	}
	if site.LSWS != nil {
		for _, directive := range site.LSWS.CustomDirectives {
			lines = append(lines, "    "+directive)
		}
	}
	if capabilities.Flags["quic"] {
		lines = append(lines, "    # feature flag: quic available")
	}
	lines = append(lines, "</VirtualHost>")

	if site.TLS.Enabled {
		lines = append(lines,
			"",
			"<VirtualHost *:443>",
			fmt.Sprintf("    ServerName %s", site.Domain.ServerName),
			fmt.Sprintf("    DocumentRoot %s", site.DocumentRoot),
			fmt.Sprintf("    SSLCertificateFile %s", site.TLS.CertificateFile),
			fmt.Sprintf("    SSLCertificateKeyFile %s", site.TLS.CertificateKey),
			"</VirtualHost>",
		)
	}

	return strings.Join(lines, "\n")
}

func buildCapabilities(info LicenseInfo) model.BackendCapabilities {
	flags := map[string]bool{
		"directive_injection": true,
		"quic":                info.Mode == "licensed" || info.Mode == "trial",
		"cache":               info.Mode == "licensed" || info.Mode == "trial",
		"esi":                 info.Mode == "licensed" || info.Mode == "trial",
	}
	notes := []string{}
	switch info.Mode {
	case "trial":
		notes = append(notes, "trial mode detected; enterprise features are enabled but time-limited")
	case "licensed":
		notes = append(notes, "licensed mode detected; enterprise feature flags enabled")
	default:
		notes = append(notes, "license mode unknown; enterprise feature flags treated conservatively")
	}
	return model.BackendCapabilities{
		Backend:     "lsws",
		LicenseMode: info.Mode,
		Flags:       flags,
		Notes:       notes,
	}
}

func buildParityReport(site model.Site, license LicenseInfo) render.ParityReport {
	items := []render.ParityItem{
		{Feature: "server_name", Status: render.ParityMapped, Target: "apache-style vhost"},
		{Feature: "server_alias", Status: render.ParityMapped, Target: "apache-style vhost"},
		{Feature: "docroot", Status: render.ParityMapped, Target: "apache-style vhost"},
		{Feature: "index", Status: render.ParityMapped, Target: "apache-style vhost"},
		{Feature: "logs", Status: render.ParityMapped, Target: "apache-style vhost"},
	}
	if site.TLS.Enabled {
		items = append(items, render.ParityItem{Feature: "tls_basic", Status: render.ParityMapped, Target: "apache-style vhost"})
	}
	if site.PHP.Enabled {
		items = append(items, render.ParityItem{Feature: "php_handler", Status: render.ParityMapped, Target: "lsphp command + handler"})
	}
	if len(site.RewriteRules) > 0 {
		items = append(items, render.ParityItem{Feature: "rewrite_basic", Status: render.ParityMapped, Target: "apache-style rewrite"})
	}
	if len(site.ReverseProxyRules) > 0 {
		items = append(items, render.ParityItem{Feature: "reverse_proxy_basic", Status: render.ParityMapped, Target: "apache-style proxy"})
	}
	if site.LSWS != nil && len(site.LSWS.CustomDirectives) > 0 {
		items = append(items, render.ParityItem{Feature: "lsws_directive_injection", Status: render.ParityMapped, Target: "custom directives"})
	}
	items = append(items, render.ParityItem{
		Feature: "license_mode",
		Status:  render.ParityMapped,
		Target:  license.Mode,
		Note:    "trial/licensed detection is evaluated at render time",
	})
	return render.ParityReport{
		Backend: "lsws",
		Site:    site.Name,
		Items:   items,
	}
}

func lsphpCommand(site model.Site, defaultBinary string) string {
	if site.PHP.Command != "" {
		return site.PHP.Command
	}
	if site.PHP.Version == "" {
		return defaultBinary
	}
	return fmt.Sprintf("%s-%s", defaultBinary, strings.ReplaceAll(site.PHP.Version, ".", ""))
}
