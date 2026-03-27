package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Profile captures file-driven install input.
type Profile struct {
	Backend         string            `json:"backend" yaml:"backend"`
	PHPVersion      string            `json:"php_version" yaml:"php_version"`
	PHPVersions     []string          `json:"php_versions" yaml:"php_versions"`
	DB              string            `json:"db" yaml:"db"`
	DBTLS           string            `json:"db_tls" yaml:"db_tls"`
	WithMemcached   bool              `json:"with_memcached" yaml:"with_memcached"`
	WithRedis       bool              `json:"with_redis" yaml:"with_redis"`
	Site            string            `json:"site" yaml:"site"`
	Email           string            `json:"email" yaml:"email"`
	NonInteractive  bool              `json:"non_interactive" yaml:"non_interactive"`
	DryRun          bool              `json:"dry_run" yaml:"dry_run"`
	PlanOnly        bool              `json:"plan_only" yaml:"plan_only"`
	SiteProfile     string            `json:"site_profile" yaml:"site_profile"`
	Scenario        string            `json:"scenario,omitempty" yaml:"scenario,omitempty"`
	PHP             *PHPProfile       `json:"php,omitempty" yaml:"php,omitempty"`
	Database        *DatabaseProfile  `json:"database,omitempty" yaml:"database,omitempty"`
	Cache           *CacheProfile     `json:"cache,omitempty" yaml:"cache,omitempty"`
	FirstSite       *FirstSiteProfile `json:"first_site,omitempty" yaml:"first_site,omitempty"`
	Operator        *OperatorProfile  `json:"operator,omitempty" yaml:"operator,omitempty"`
	Execution       *ExecutionProfile `json:"execution,omitempty" yaml:"execution,omitempty"`
	ScenarioProfile *ScenarioProfile  `json:"scenario_profile,omitempty" yaml:"scenario_profile,omitempty"`
}

type ScenarioProfile struct {
	Name         string `json:"name,omitempty" yaml:"name,omitempty"`
	Upstream     string `json:"upstream,omitempty" yaml:"upstream,omitempty"`
	InstallRedis *bool  `json:"install_redis,omitempty" yaml:"install_redis,omitempty"`
}

type PHPProfile struct {
	PrimaryVersion string   `json:"primary_version,omitempty" yaml:"primary_version,omitempty"`
	Versions       []string `json:"versions,omitempty" yaml:"versions,omitempty"`
}

type DatabaseProfile struct {
	Provider string `json:"provider,omitempty" yaml:"provider,omitempty"`
	TLS      string `json:"tls,omitempty" yaml:"tls,omitempty"`
}

type CacheProfile struct {
	Memcached *bool `json:"memcached,omitempty" yaml:"memcached,omitempty"`
	Redis     *bool `json:"redis,omitempty" yaml:"redis,omitempty"`
}

type FirstSiteProfile struct {
	Domain  string `json:"domain,omitempty" yaml:"domain,omitempty"`
	Profile string `json:"profile,omitempty" yaml:"profile,omitempty"`
}

type OperatorProfile struct {
	Email string `json:"email,omitempty" yaml:"email,omitempty"`
}

type ExecutionProfile struct {
	NonInteractive *bool `json:"non_interactive,omitempty" yaml:"non_interactive,omitempty"`
	DryRun         *bool `json:"dry_run,omitempty" yaml:"dry_run,omitempty"`
	PlanOnly       *bool `json:"plan_only,omitempty" yaml:"plan_only,omitempty"`
}

// LoadProfile loads an install profile from YAML or JSON.
func LoadProfile(path string) (Profile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, err
	}

	var profile Profile
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		if err := json.Unmarshal(raw, &profile); err != nil {
			return Profile{}, err
		}
	default:
		if err := yaml.Unmarshal(raw, &profile); err != nil {
			return Profile{}, err
		}
	}
	profile.PHPVersions = uniqueStrings(profile.PHPVersions)
	return profile, nil
}

// ToOptions normalizes a file profile into install service options.
func (p Profile) ToOptions() Options {
	phpVersion := strings.TrimSpace(p.PHPVersion)
	phpVersions := uniqueStrings(p.PHPVersions)
	dbProvider := strings.TrimSpace(p.DB)
	dbTLS := strings.TrimSpace(p.DBTLS)
	withMemcached := p.WithMemcached
	withRedis := p.WithRedis
	siteName := strings.TrimSpace(p.Site)
	siteProfile := strings.TrimSpace(p.SiteProfile)
	email := strings.TrimSpace(p.Email)
	nonInteractive := p.NonInteractive
	dryRun := p.DryRun
	planOnly := p.PlanOnly

	if p.PHP != nil {
		if value := strings.TrimSpace(p.PHP.PrimaryVersion); value != "" {
			phpVersion = value
		}
		if len(p.PHP.Versions) > 0 {
			phpVersions = uniqueStrings(p.PHP.Versions)
		}
	}
	if p.Database != nil {
		if value := strings.TrimSpace(p.Database.Provider); value != "" {
			dbProvider = value
		}
		if value := strings.TrimSpace(p.Database.TLS); value != "" {
			dbTLS = value
		}
	}
	if p.Cache != nil {
		if p.Cache.Memcached != nil {
			withMemcached = *p.Cache.Memcached
		}
		if p.Cache.Redis != nil {
			withRedis = *p.Cache.Redis
		}
	}
	if p.FirstSite != nil {
		if value := strings.TrimSpace(p.FirstSite.Domain); value != "" {
			siteName = value
		}
		if value := strings.TrimSpace(p.FirstSite.Profile); value != "" {
			siteProfile = value
		}
	}
	if p.Operator != nil {
		if value := strings.TrimSpace(p.Operator.Email); value != "" {
			email = value
		}
	}
	if p.Execution != nil {
		if p.Execution.NonInteractive != nil {
			nonInteractive = *p.Execution.NonInteractive
		}
		if p.Execution.DryRun != nil {
			dryRun = *p.Execution.DryRun
		}
		if p.Execution.PlanOnly != nil {
			planOnly = *p.Execution.PlanOnly
		}
	}

	scenario := strings.TrimSpace(p.Scenario)
	siteUpstream := ""
	if p.ScenarioProfile != nil {
		if value := strings.TrimSpace(p.ScenarioProfile.Name); value != "" {
			scenario = value
		}
		if value := strings.TrimSpace(p.ScenarioProfile.Upstream); value != "" {
			siteUpstream = value
		}
		if p.ScenarioProfile.InstallRedis != nil {
			withRedis = *p.ScenarioProfile.InstallRedis
		}
	}

	switch normalizeScenario(scenario) {
	case "wordpress":
		if siteProfile == "" {
			siteProfile = "wordpress"
		}
		if dbProvider == "" {
			dbProvider = "mariadb"
		}
		if dbTLS == "" {
			dbTLS = "enabled"
		}
		if phpVersion == "" {
			phpVersion = "8.3"
		}
		withMemcached = true
	case "laravel":
		if siteProfile == "" {
			siteProfile = "laravel"
		}
		if dbProvider == "" {
			dbProvider = "postgresql"
		}
		if dbTLS == "" {
			dbTLS = "enabled"
		}
		if phpVersion == "" {
			phpVersion = "8.3"
		}
		withRedis = true
	case "api":
		if siteProfile == "" {
			siteProfile = "generic"
		}
		if dbProvider == "" {
			dbProvider = "postgresql"
		}
		if phpVersion == "" {
			phpVersion = "8.3"
		}
		withRedis = true
	case "static":
		if siteProfile == "" {
			siteProfile = "static"
		}
	case "reverse-proxy":
		if siteProfile == "" {
			siteProfile = "reverse-proxy"
		}
		if p.ScenarioProfile != nil && strings.TrimSpace(p.ScenarioProfile.Upstream) != "" && siteName == "" {
			siteName = "proxy.local"
		}
	}

	return Options{
		Backend:        strings.TrimSpace(p.Backend),
		PHPVersion:     phpVersion,
		PHPVersions:    phpVersions,
		DBProvider:     dbProvider,
		DBTLS:          dbTLS,
		WithMemcached:  withMemcached,
		WithRedis:      withRedis,
		Site:           siteName,
		Email:          email,
		SiteProfile:    siteProfile,
		SiteUpstream:   siteUpstream,
		NonInteractive: nonInteractive,
		DryRun:         dryRun,
		PlanOnly:       planOnly,
	}
}

func normalizeScenario(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "_", "-")
	return value
}
