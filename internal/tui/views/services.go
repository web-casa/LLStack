package views

import (
	"fmt"
	"strings"

	"github.com/web-casa/llstack/internal/config"
)

// RenderServices renders the services page.
func RenderServices(cfg config.RuntimeConfig) string {
	phpRuntimes, _ := readPHPRuntimes(cfg)
	dbProviders, _ := readDBProviders(cfg)
	cacheProviders, _ := readCacheProviders(cfg)
	sites, _ := readSiteManifests(cfg)

	return strings.Join([]string{
		"Services",
		"",
		fmt.Sprintf("web       managed_sites=%d", len(sites)),
		fmt.Sprintf("php       runtimes=%d", len(phpRuntimes)),
		fmt.Sprintf("database  providers=%d", len(dbProviders)),
		fmt.Sprintf("cache     providers=%d", len(cacheProviders)),
	}, "\n")
}
