package php

import (
	"fmt"

	"github.com/web-casa/llstack/internal/core/model"
)

// BindSiteRuntime applies backend-specific PHP runtime defaults to a site.
func BindSiteRuntime(resolver Resolver, site *model.Site) error {
	if site == nil || !site.PHP.Enabled {
		return nil
	}
	if site.PHP.Version == "" {
		return fmt.Errorf("php version is required when php is enabled")
	}
	if err := resolver.ValidateVersion(site.PHP.Version); err != nil {
		return err
	}

	switch site.Backend {
	case "apache":
		site.PHP.Handler = "php-fpm"
		site.PHP.Socket = resolver.FPMSocketPath(site.PHP.Version)
		site.PHP.FPMService = resolver.FPMServiceName(site.PHP.Version)
		site.PHP.Command = ""
	case "ols", "lsws":
		site.PHP.Handler = "lsphp"
		site.PHP.Command = resolver.LSPHPCommand(site.PHP.Version)
		site.PHP.Socket = ""
		site.PHP.FPMService = ""
	default:
		return fmt.Errorf("unsupported backend %q", site.Backend)
	}
	return nil
}
