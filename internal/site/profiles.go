package site

import (
	"fmt"

	"github.com/web-casa/llstack/internal/core/model"
)

const (
	ProfileGeneric      = "generic"
	ProfileWordPress    = "wordpress"
	ProfileLaravel      = "laravel"
	ProfileStatic       = "static"
	ProfileReverseProxy = "reverse-proxy"
)

// AvailableProfiles returns the built-in deploy profiles.
func AvailableProfiles() []model.DeployProfile {
	return []model.DeployProfile{
		{
			Name:              ProfileGeneric,
			Description:       "Generic PHP site with index.php first",
			EnablePHP:         true,
			DefaultIndexFiles: []string{"index.php", "index.html", "index.htm"},
		},
		{
			Name:              ProfileWordPress,
			Description:       "WordPress-friendly rewrite and PHP defaults",
			EnablePHP:         true,
			DefaultIndexFiles: []string{"index.php", "index.html"},
			RewriteRules: []model.RewriteRule{
				{Pattern: "^index\\.php$", Substitution: "-", Flags: []string{"L"}},
				{Pattern: ".", Substitution: "/index.php", Flags: []string{"L"}},
			},
			HeaderRules: []model.HeaderRule{
				{Name: "X-Frame-Options", Value: "SAMEORIGIN", Action: "set"},
				{Name: "X-Content-Type-Options", Value: "nosniff", Action: "set"},
			},
		},
		{
			Name:              ProfileLaravel,
			Description:       "Laravel public/ root and front-controller rewrite",
			EnablePHP:         true,
			DefaultIndexFiles: []string{"index.php", "index.html"},
			RewriteRules: []model.RewriteRule{
				{Pattern: "^", Substitution: "public/", Flags: []string{"L"}},
			},
		},
		{
			Name:              ProfileStatic,
			Description:       "Static site without PHP runtime",
			EnablePHP:         false,
			DefaultIndexFiles: []string{"index.html", "index.htm"},
		},
		{
			Name:              ProfileReverseProxy,
			Description:       "Reverse proxy site with backend upstream",
			EnablePHP:         false,
			DefaultIndexFiles: []string{"index.html"},
		},
	}
}

// ApplyProfile mutates a site with the selected deploy profile defaults.
func ApplyProfile(site *model.Site, profileName string, upstream string) error {
	if profileName == "" {
		profileName = ProfileGeneric
	}
	for _, profile := range AvailableProfiles() {
		if profile.Name != profileName {
			continue
		}
		site.Profile = profile.Name
		if len(site.IndexFiles) == 0 {
			site.IndexFiles = append([]string{}, profile.DefaultIndexFiles...)
		}
		if !site.PHP.Enabled && profile.EnablePHP {
			site.PHP.Enabled = true
		}
		if !profile.EnablePHP {
			site.PHP = model.PHPRuntimeBinding{}
		}
		if len(site.RewriteRules) == 0 && len(profile.RewriteRules) > 0 {
			site.RewriteRules = append([]model.RewriteRule{}, profile.RewriteRules...)
		}
		if len(site.HeaderRules) == 0 && len(profile.HeaderRules) > 0 {
			site.HeaderRules = append([]model.HeaderRule{}, profile.HeaderRules...)
		}
		if profile.Name == ProfileLaravel && site.DocumentRoot != "" {
			site.DocumentRoot = site.DocumentRoot + "/public"
		}
		if profile.Name == ProfileReverseProxy {
			if upstream == "" {
				return fmt.Errorf("reverse-proxy profile requires upstream")
			}
			site.ReverseProxyRules = []model.ReverseProxyRule{
				{
					PathPrefix:   "/",
					Upstream:     upstream,
					PreserveHost: true,
				},
			}
		}
		return nil
	}
	return fmt.Errorf("unknown site profile %q", profileName)
}
