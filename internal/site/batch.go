package site

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/core/plan"

	"gopkg.in/yaml.v3"
)

// BatchSiteSpec defines a single site in a batch config file.
type BatchSiteSpec struct {
	Name       string `json:"name" yaml:"name"`
	Backend    string `json:"backend" yaml:"backend"`
	Profile    string `json:"profile" yaml:"profile"`
	PHPVersion string `json:"php_version" yaml:"php_version"`
	Aliases    string `json:"aliases" yaml:"aliases"`
	Upstream   string `json:"upstream" yaml:"upstream"`
}

// BatchConfig is the top-level structure for batch site creation.
type BatchConfig struct {
	Sites []BatchSiteSpec `json:"sites" yaml:"sites"`
}

// LoadBatchConfig reads a YAML or JSON file containing multiple site definitions.
func LoadBatchConfig(path string) (BatchConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return BatchConfig{}, fmt.Errorf("read batch config: %w", err)
	}

	var cfg BatchConfig
	if strings.HasSuffix(path, ".json") {
		err = json.Unmarshal(raw, &cfg)
	} else {
		err = yaml.Unmarshal(raw, &cfg)
	}
	if err != nil {
		return BatchConfig{}, fmt.Errorf("parse batch config: %w", err)
	}
	if len(cfg.Sites) == 0 {
		return BatchConfig{}, fmt.Errorf("batch config contains no sites")
	}
	return cfg, nil
}

// CreateBatch creates multiple sites from a batch config.
func (m Manager) CreateBatch(ctx context.Context, cfg BatchConfig, dryRun bool, skipReload bool) ([]plan.Plan, error) {
	var plans []plan.Plan

	for _, spec := range cfg.Sites {
		if spec.Name == "" {
			return plans, fmt.Errorf("site name is required in batch config")
		}

		backend := spec.Backend
		if backend == "" {
			backend = "apache"
		}
		profile := spec.Profile
		if profile == "" {
			profile = ProfileGeneric
		}

		siteSpec := model.Site{
			Name:    spec.Name,
			Backend: backend,
			Domain: model.DomainBinding{
				ServerName: spec.Name,
			},
		}

		if spec.Aliases != "" {
			for _, alias := range strings.Split(spec.Aliases, ",") {
				alias = strings.TrimSpace(alias)
				if alias != "" {
					siteSpec.Domain.Aliases = append(siteSpec.Domain.Aliases, alias)
				}
			}
		}

		if spec.PHPVersion != "" && profile != ProfileStatic && profile != ProfileReverseProxy {
			siteSpec.PHP.Enabled = true
			siteSpec.PHP.Version = spec.PHPVersion
		}

		if err := ApplyProfile(&siteSpec, profile, spec.Upstream); err != nil {
			return plans, fmt.Errorf("site %s: %w", spec.Name, err)
		}

		p, err := m.Create(ctx, CreateOptions{
			Site:       siteSpec,
			DryRun:     dryRun,
			PlanOnly:   dryRun,
			SkipReload: skipReload || dryRun,
		})
		if err != nil {
			return plans, fmt.Errorf("site %s: %w", spec.Name, err)
		}
		plans = append(plans, p)
	}

	return plans, nil
}
