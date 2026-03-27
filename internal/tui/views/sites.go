package views

import (
	"fmt"
	"strings"

	"github.com/web-casa/llstack/internal/config"
)

// RenderSites renders the sites page.
func RenderSites(cfg config.RuntimeConfig) string {
	lines := []string{"Sites", ""}
	sites, err := readSiteManifests(cfg)
	if err != nil {
		return strings.Join([]string{"Sites", "", "error: " + err.Error()}, "\n")
	}
	if len(sites) == 0 {
		return strings.Join([]string{
			"Sites",
			"",
			"No sites configured yet.",
			"Use `llstack site:create` or the install flow to bootstrap the first site.",
		}, "\n")
	}
	for _, item := range sites {
		lines = append(lines, fmt.Sprintf("%s  backend=%s  state=%s  profile=%s  tls=%t  missing_assets=%d", item.Site.Name, item.Site.Backend, item.Site.State, item.Site.Profile, item.Site.TLS.Enabled, siteDriftCount(item)))
	}
	return strings.Join(lines, "\n")
}
