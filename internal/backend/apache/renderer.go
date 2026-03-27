package apache

import (
	"fmt"
	"strings"

	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/core/render"
)

// Renderer renders canonical sites into Apache VirtualHost files.
type Renderer struct{}

// NewRenderer creates an Apache renderer.
func NewRenderer() Renderer {
	return Renderer{}
}

// RenderSite renders the site into an Apache vhost file.
func (r Renderer) RenderSite(site model.Site, opts render.SiteRenderOptions) (render.SiteRenderResult, error) {
	content := renderHTTPBlock(site)
	if site.TLS.Enabled {
		content += "\n\n" + renderHTTPSBlock(site)
	}

	assets := []render.Asset{
		{
			Path:    opts.VHostPath,
			Content: []byte(content + "\n"),
			Mode:    0o644,
		},
	}

	// Generate per-site FPM pool config when user isolation is active
	if opts.FPMPoolConfigPath != "" && opts.SystemUser != "" && site.PHP.Enabled && site.PHP.Handler == "php-fpm" {
		poolContent := renderFPMPoolConfig(site, opts.SystemUser)
		assets = append(assets, render.Asset{
			Path:    opts.FPMPoolConfigPath,
			Content: []byte(poolContent),
			Mode:    0o644,
		})
	}

	return render.SiteRenderResult{Assets: assets}, nil
}

func renderFPMPoolConfig(site model.Site, siteUser string) string {
	socketPath := site.PHP.Socket
	lines := []string{
		fmt.Sprintf("[%s]", site.Name),
		fmt.Sprintf("user = %s", siteUser),
		fmt.Sprintf("group = %s", siteUser),
		fmt.Sprintf("listen = %s", socketPath),
		fmt.Sprintf("listen.owner = %s", siteUser),
		fmt.Sprintf("listen.group = %s", siteUser),
		"listen.mode = 0660",
		"",
		"pm = dynamic",
		"pm.max_children = 5",
		"pm.start_servers = 2",
		"pm.min_spare_servers = 1",
		"pm.max_spare_servers = 3",
		"",
		fmt.Sprintf("php_admin_value[open_basedir] = %s:/tmp:/usr/share/php", site.DocumentRoot),
		"php_admin_value[session.save_path] = /tmp",
	}
	return strings.Join(lines, "\n") + "\n"
}

func renderHTTPBlock(site model.Site) string {
	lines := []string{
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
	lines = append(lines, renderDirectoryBlock(site)...)
	lines = append(lines, renderPHPBlock(site)...)
	lines = append(lines, renderRewriteRules(site)...)
	lines = append(lines, renderProxyRules(site)...)
	lines = append(lines, "</VirtualHost>")
	return strings.Join(filterEmpty(lines), "\n")
}

func renderHTTPSBlock(site model.Site) string {
	lines := []string{
		"<VirtualHost *:443>",
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
		"    SSLEngine on",
		fmt.Sprintf("    SSLCertificateFile %s", site.TLS.CertificateFile),
		fmt.Sprintf("    SSLCertificateKeyFile %s", site.TLS.CertificateKey),
	)
	lines = append(lines, renderDirectoryBlock(site)...)
	lines = append(lines, renderPHPBlock(site)...)
	lines = append(lines, renderRewriteRules(site)...)
	lines = append(lines, renderProxyRules(site)...)
	lines = append(lines, "</VirtualHost>")
	return strings.Join(filterEmpty(lines), "\n")
}

func renderDirectoryBlock(site model.Site) []string {
	lines := []string{
		fmt.Sprintf("    <Directory %s>", site.DocumentRoot),
		"        AllowOverride All",
		"        Require all granted",
		"    </Directory>",
	}
	for _, rule := range site.HeaderRules {
		lines = append(lines, fmt.Sprintf("    Header %s %s \"%s\"", rule.Action, rule.Name, rule.Value))
	}
	for _, rule := range site.AccessRules {
		lines = append(lines, fmt.Sprintf("    # access rule: %s %s %s", rule.Path, rule.Action, rule.Source))
	}
	return lines
}

func renderPHPBlock(site model.Site) []string {
	if !site.PHP.Enabled {
		return nil
	}
	if site.PHP.Handler != "php-fpm" {
		return []string{fmt.Sprintf("    # unsupported php handler in phase 2: %s", site.PHP.Handler)}
	}

	return []string{
		"    <FilesMatch \\.php$>",
		fmt.Sprintf("        SetHandler \"proxy:unix:%s|fcgi://localhost\"", site.PHP.Socket),
		"    </FilesMatch>",
	}
}

func renderRewriteRules(site model.Site) []string {
	if len(site.RewriteRules) == 0 {
		return nil
	}

	lines := []string{"    RewriteEngine On"}
	for _, rule := range site.RewriteRules {
		flags := ""
		if len(rule.Flags) > 0 {
			flags = " [" + strings.Join(rule.Flags, ",") + "]"
		}
		lines = append(lines, fmt.Sprintf("    RewriteRule %s %s%s", rule.Pattern, rule.Substitution, flags))
	}
	return lines
}

func renderProxyRules(site model.Site) []string {
	if len(site.ReverseProxyRules) == 0 {
		return nil
	}

	lines := make([]string, 0, len(site.ReverseProxyRules)*2)
	for _, rule := range site.ReverseProxyRules {
		lines = append(lines,
			fmt.Sprintf("    ProxyPass %s %s", rule.PathPrefix, rule.Upstream),
			fmt.Sprintf("    ProxyPassReverse %s %s", rule.PathPrefix, rule.Upstream),
		)
	}
	return lines
}

func filterEmpty(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}
