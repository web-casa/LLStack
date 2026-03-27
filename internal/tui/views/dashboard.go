package views

import (
	"fmt"
	"strings"

	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/system"
	"github.com/web-casa/llstack/internal/tuning"
)

// RenderDashboard renders the Phase 1 dashboard placeholder.
func RenderDashboard(cfg config.RuntimeConfig) string {
	sites, _ := readSiteManifests(cfg)
	phpRuntimes, _ := readPHPRuntimes(cfg)
	dbProviders, _ := readDBProviders(cfg)
	cacheProviders, _ := readCacheProviders(cfg)
	disabledSites := 0
	driftedSites := 0
	for _, site := range sites {
		if site.Site.State == "disabled" {
			disabledSites++
		}
		if siteDriftCount(site) > 0 {
			driftedSites++
		}
	}

	webStatus := "not_configured"
	if len(sites) > 0 {
		webStatus = fmt.Sprintf("%d sites managed", len(sites))
	}
	phpStatus := "not_configured"
	if len(phpRuntimes) > 0 {
		phpStatus = fmt.Sprintf("%d runtimes", len(phpRuntimes))
	}
	dbStatus := "not_configured"
	if len(dbProviders) > 0 {
		dbStatus = fmt.Sprintf("%d providers", len(dbProviders))
	}
	cacheStatus := "not_configured"
	if len(cacheProviders) > 0 {
		cacheStatus = fmt.Sprintf("%d providers", len(cacheProviders))
	}

	lines := []string{
		"Dashboard",
		"",
		"Current backend: unconfigured",
		"Web service: " + webStatus,
		fmt.Sprintf("Disabled sites: %d", disabledSites),
		fmt.Sprintf("Sites with missing managed assets: %d", driftedSites),
		"PHP service: " + phpStatus,
		"Database service: " + dbStatus,
		"Cache service: " + cacheStatus,
		fmt.Sprintf("Default site root: %s", cfg.Paths.SitesRootDir),
		"Recent operations: use History/rollback records from CLI today",
	}
	if len(sites) == 0 {
		lines = append(lines, "Quick start: create the first site from Sites with `c`, or run `llstack site:create <server-name> --dry-run`.")
	}
	if len(phpRuntimes) == 0 {
		lines = append(lines, "PHP hint: run `llstack php:install 8.3 --dry-run` before binding PHP sites.")
	}
	if len(dbProviders) == 0 {
		lines = append(lines, "DB hint: run `llstack db:install mariadb --dry-run` to preview provider bootstrap.")
	}

	// Hardware & tuning summary
	hw := system.DetectHardware()
	siteCount := len(sites)
	if siteCount < 1 {
		siteCount = 1
	}
	profile := tuning.Calculate(hw, siteCount)
	lines = append(lines, "",
		fmt.Sprintf("Hardware: %d CPU / %.1f GB RAM", hw.CPUCores, hw.MemoryGB),
		fmt.Sprintf("Tuning:  PHP max_children=%d/site  Apache workers=%d  DB buffer=%dMB  Cache=%dMB",
			profile.PHPMaxChildrenSite, profile.ApacheMaxRequestWorkers, profile.DBBufferPoolMB, profile.RedisMaxMemoryMB),
		"Run `llstack tune --json` for full recommendations.",
	)

	return strings.Join(lines, "\n")
}
