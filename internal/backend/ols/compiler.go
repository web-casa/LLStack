package ols

import (
	"fmt"
	"strings"

	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/core/render"
)

// Compiler compiles canonical site definitions into OLS-native config trees.
type Compiler struct {
	cfg config.RuntimeConfig
}

// NewCompiler constructs an OLS compiler.
func NewCompiler(cfg config.RuntimeConfig) Compiler {
	return Compiler{cfg: cfg}
}

// RenderSite compiles a site into OLS-native config assets and a parity report.
func (c Compiler) RenderSite(site model.Site, opts render.SiteRenderOptions) (render.SiteRenderResult, error) {
	parity := buildParityReport(site)

	assets := []render.Asset{
		{
			Path:    opts.VHostPath,
			Content: []byte(renderVHostConfig(site, c.cfg.OLS.DefaultPHPBinary, opts.SystemUser) + "\n"),
			Mode:    0o644,
		},
		{
			Path:    opts.ListenerMapPath,
			Content: []byte(renderListenerMap(site) + "\n"),
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

	return render.SiteRenderResult{
		Assets:       assets,
		Warnings:     collectWarnings(parity),
		ParityReport: &parity,
	}, nil
}

func renderVHostConfig(site model.Site, phpBinary string, systemUser string) string {
	var lines []string

	lines = append(lines,
		fmt.Sprintf("vhRoot                   %s", site.DocumentRoot),
		fmt.Sprintf("docRoot                  %s", site.DocumentRoot),
		fmt.Sprintf("index  {\n  useServer               0\n  indexFiles              %s\n}", strings.Join(site.IndexFiles, ", ")),
		fmt.Sprintf("errorlog %s {\n  useServer               0\n  logLevel                WARN\n}\naccesslog %s {\n  useServer               0\n  logFormat               combined\n}", site.Logs.ErrorLog, site.Logs.AccessLog),
	)

	if site.PHP.Enabled {
		lines = append(lines, renderExtProcessor(site, phpBinary))
		lines = append(lines, renderScriptHandler(site))
	}
	if len(site.RewriteRules) > 0 || site.TLS.Enabled {
		lines = append(lines, renderRewriteBlock(site))
	}
	if len(site.ReverseProxyRules) > 0 {
		lines = append(lines, renderContextBlocks(site)...)
	}
	if site.TLS.Enabled {
		lines = append(lines, renderTLSBlock(site))
	}

	// Per-site user isolation: phpIniOverride with open_basedir
	if systemUser != "" && site.PHP.Enabled {
		lines = append(lines, fmt.Sprintf("phpIniOverride {\n  php_admin_value open_basedir %s:/tmp:/usr/share/php\n}", site.DocumentRoot))
	}

	return strings.Join(lines, "\n\n")
}

func renderListenerMap(site model.Site) string {
	aliases := append([]string{site.Domain.ServerName}, site.Domain.Aliases...)
	return fmt.Sprintf("map %s %s", site.Name, strings.Join(aliases, ", "))
}

func renderExtProcessor(site model.Site, phpBinary string) string {
	name := extProcessorName(site)
	command := phpBinary
	if site.PHP.Command != "" {
		command = site.PHP.Command
	} else if site.PHP.Version != "" {
		command = fmt.Sprintf("%s-%s", phpBinary, strings.ReplaceAll(site.PHP.Version, ".", ""))
	}
	return fmt.Sprintf("extprocessor %s {\n  type                    lsapi\n  address                 uds://tmp/lshttpd/%s.sock\n  maxConns                10\n  env                     PHP_LSAPI_CHILDREN=10\n  initTimeout             60\n  retryTimeout            0\n  persistConn             1\n  pcKeepAliveTimeout      60\n  respBuffer              0\n  autoStart               1\n  path                    %s\n  backlog                 100\n  instances               1\n  priority                0\n  memSoftLimit            2047M\n  memHardLimit            2047M\n  procSoftLimit           400\n  procHardLimit           500\n}", name, site.Name, command)
}

func renderScriptHandler(site model.Site) string {
	return fmt.Sprintf("scripthandler  {\n  add                     lsapi:%s php\n}", extProcessorName(site))
}

func renderRewriteBlock(site model.Site) string {
	lines := []string{"rewrite  {\n  enable                  1\n  autoLoadHtaccess        1"}
	if site.TLS.Enabled {
		lines = append(lines, fmt.Sprintf("  rules                   RewriteRule ^ https://%s%%{REQUEST_URI} [R=301,L]", site.Domain.ServerName))
	}
	for _, rule := range site.RewriteRules {
		flags := ""
		if len(rule.Flags) > 0 {
			flags = " [" + strings.Join(rule.Flags, ",") + "]"
		}
		lines = append(lines, fmt.Sprintf("  rules                   RewriteRule %s %s%s", rule.Pattern, rule.Substitution, flags))
	}
	lines = append(lines, "}")
	return strings.Join(lines, "\n")
}

func renderContextBlocks(site model.Site) []string {
	out := make([]string, 0, len(site.ReverseProxyRules))
	for _, rule := range site.ReverseProxyRules {
		out = append(out, fmt.Sprintf("context %s {\n  type                    proxy\n  location                %s\n  handler                 %s\n  addDefaultCharset       off\n}", sanitizeContext(rule.PathPrefix), rule.PathPrefix, rule.Upstream))
	}
	return out
}

func renderTLSBlock(site model.Site) string {
	return fmt.Sprintf("vhssl  {\n  keyFile                 %s\n  certFile                %s\n}", site.TLS.CertificateKey, site.TLS.CertificateFile)
}

func buildParityReport(site model.Site) render.ParityReport {
	items := []render.ParityItem{
		{Feature: "server_name", Status: render.ParityMapped, Target: "vhost"},
		{Feature: "server_alias", Status: render.ParityMapped, Target: "listener map"},
		{Feature: "docroot", Status: render.ParityMapped, Target: "vhRoot/docRoot"},
		{Feature: "index", Status: render.ParityMapped, Target: "index.indexFiles"},
		{Feature: "logs", Status: render.ParityMapped, Target: "errorlog/accesslog"},
	}
	if site.TLS.Enabled {
		items = append(items, render.ParityItem{Feature: "tls_basic", Status: render.ParityMapped, Target: "vhssl"})
	}
	if site.PHP.Enabled {
		items = append(items, render.ParityItem{Feature: "php_handler", Status: render.ParityMapped, Target: "extprocessor + scripthandler"})
	}
	if len(site.RewriteRules) > 0 {
		items = append(items, render.ParityItem{Feature: "rewrite_basic", Status: render.ParityMapped, Target: "rewrite.rules"})
	}
	if len(site.ReverseProxyRules) > 0 {
		items = append(items, render.ParityItem{Feature: "reverse_proxy_basic", Status: render.ParityMapped, Target: "context proxy"})
	}
	if len(site.HeaderRules) > 0 {
		items = append(items, render.ParityItem{
			Feature: "header_rules",
			Status:  render.ParityDegraded,
			Target:  "not_emitted",
			Note:    "Phase 3 records header rules in parity output but does not compile them into OLS config yet",
		})
	}
	if len(site.AccessRules) > 0 {
		items = append(items, render.ParityItem{
			Feature: "access_control",
			Status:  render.ParityDegraded,
			Target:  "not_emitted",
			Note:    "Phase 3 keeps access control rules in canonical model but does not fully map them to OLS contexts",
		})
	}
	return render.ParityReport{
		Backend: "ols",
		Site:    site.Name,
		Items:   items,
	}
}

func collectWarnings(report render.ParityReport) []string {
	var warnings []string
	for _, item := range report.Items {
		if item.Status == render.ParityMapped {
			continue
		}
		warnings = append(warnings, fmt.Sprintf("%s: %s", item.Feature, item.Note))
	}
	return warnings
}

func extProcessorName(site model.Site) string {
	return "lsphp-" + strings.ReplaceAll(site.Name, ".", "-")
}

func sanitizeContext(path string) string {
	s := strings.Trim(path, "/")
	if s == "" {
		return "root"
	}
	return strings.ReplaceAll(s, "/", "_")
}
