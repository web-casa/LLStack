package validate

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/web-casa/llstack/internal/core/model"
)

var hostPattern = regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)

// Site validates a canonical site definition.
func Site(site model.Site) error {
	var problems []string

	if site.Name == "" {
		problems = append(problems, "name is required")
	}
	if site.Backend == "" {
		problems = append(problems, "backend is required")
	}
	if site.Backend != "apache" && site.Backend != "ols" && site.Backend != "lsws" {
		problems = append(problems, "supported backends are apache, ols, and lsws")
	}
	if site.Domain.ServerName == "" {
		problems = append(problems, "domain.server_name is required")
	} else if !hostPattern.MatchString(site.Domain.ServerName) {
		problems = append(problems, "domain.server_name must contain only letters, digits, dots, and hyphens")
	}
	for _, alias := range site.Domain.Aliases {
		if alias != "" && !hostPattern.MatchString(alias) {
			problems = append(problems, fmt.Sprintf("alias %q is invalid", alias))
		}
	}
	if site.DocumentRoot == "" {
		problems = append(problems, "document_root is required")
	} else if !filepath.IsAbs(site.DocumentRoot) {
		problems = append(problems, "document_root must be absolute")
	}
	if len(site.IndexFiles) == 0 {
		problems = append(problems, "index_files must not be empty")
	}
	if site.TLS.Enabled {
		if site.TLS.CertificateFile == "" || site.TLS.CertificateKey == "" {
			problems = append(problems, "tls certificate_file and certificate_key are required when TLS is enabled")
		}
	}
	if site.PHP.Enabled && site.PHP.Handler == "" {
		problems = append(problems, "php.handler is required when PHP is enabled")
	}
	if site.PHP.Enabled && site.PHP.Handler == "php-fpm" && site.PHP.Socket == "" {
		problems = append(problems, "php.socket is required for php-fpm")
	}
	if site.PHP.Enabled && site.PHP.Handler == "lsphp" && site.PHP.Command == "" {
		problems = append(problems, "php.command is required for lsphp")
	}
	for _, rule := range site.RewriteRules {
		if strings.TrimSpace(rule.Pattern) == "" || strings.TrimSpace(rule.Substitution) == "" {
			problems = append(problems, "rewrite rules require pattern and substitution")
		}
	}
	for _, rule := range site.ReverseProxyRules {
		if strings.TrimSpace(rule.PathPrefix) == "" || strings.TrimSpace(rule.Upstream) == "" {
			problems = append(problems, "reverse proxy rules require path_prefix and upstream")
		}
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}

	return nil
}
