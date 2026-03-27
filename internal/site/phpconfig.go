package site

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/web-casa/llstack/internal/core/plan"
	"github.com/web-casa/llstack/internal/system"
)

// PHPConfigOverride represents a per-site PHP configuration parameter.
type PHPConfigOverride struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// PHPConfigOptions controls per-site PHP configuration.
type PHPConfigOptions struct {
	Name     string              // site name
	Overrides []PHPConfigOverride // parameters to set
	Reset    bool                // remove all overrides
	Show     bool                // show current config
	DryRun   bool
	PlanOnly bool
}

// Allowed per-site PHP override parameters.
var allowedPHPOverrides = map[string]bool{
	"memory_limit":              true,
	"upload_max_filesize":       true,
	"post_max_size":             true,
	"max_execution_time":        true,
	"max_input_time":            true,
	"max_input_vars":            true,
	"display_errors":            true,
	"error_reporting":           true,
	"session.save_path":         true,
	"opcache.memory_consumption": true,
}

// UpdatePHPConfig applies per-site PHP configuration overrides.
func (m Manager) UpdatePHPConfig(ctx context.Context, opts PHPConfigOptions) (plan.Plan, error) {
	manifest, err := m.loadManifest(opts.Name)
	if err != nil {
		return plan.Plan{}, fmt.Errorf("site %q not found", opts.Name)
	}

	if !manifest.Site.PHP.Enabled {
		return plan.Plan{}, fmt.Errorf("PHP is not enabled for site %q", opts.Name)
	}

	p := plan.New("site.php-config", fmt.Sprintf("Update PHP config for %s", opts.Name))
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly

	// Validate overrides
	for _, ov := range opts.Overrides {
		if !allowedPHPOverrides[ov.Key] {
			return plan.Plan{}, fmt.Errorf("PHP parameter %q is not in the allowed override list", ov.Key)
		}
	}

	configPath, content := m.buildPHPConfigForSite(opts)

	if opts.Reset {
		p.AddOperation(plan.Operation{
			ID:     "reset-php-config-" + opts.Name,
			Kind:   "file.delete",
			Target: configPath,
		})
	} else {
		p.AddOperation(plan.Operation{
			ID:     "write-php-config-" + opts.Name,
			Kind:   "file.write",
			Target: configPath,
			Details: map[string]string{
				"backend":    manifest.Site.Backend,
				"parameters": fmt.Sprintf("%d", len(opts.Overrides)),
			},
		})
	}

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	if opts.Reset {
		os.Remove(configPath)
	} else {
		if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
			return p, err
		}
		if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
			return p, err
		}
	}

	// Reload PHP service
	if manifest.Site.PHP.FPMService != "" {
		m.exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"reload", manifest.Site.PHP.FPMService}})
	}

	return p, nil
}

// ShowPHPConfig returns the current per-site PHP config content.
func (m Manager) ShowPHPConfig(name string) (string, error) {
	configPath, _ := m.resolvePHPConfigPath(name)
	raw, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "; No per-site PHP overrides configured\n", nil
		}
		return "", err
	}
	return string(raw), nil
}

func (m Manager) resolvePHPConfigPath(name string) (string, string) {
	opts := PHPConfigOptions{Name: name}
	return m.buildPHPConfigForSite(opts)
}

func (m Manager) buildPHPConfigForSite(opts PHPConfigOptions) (string, string) {
	manifest, _ := m.loadManifest(opts.Name)
	site := manifest.Site
	backend := site.Backend

	var configPath string
	var lines []string

	switch backend {
	case "apache":
		// Per-site FPM pool override: write to php-fpm.d/llstack-site-override.conf
		versionTag := strings.ReplaceAll(site.PHP.Version, ".", "")
		configPath = filepath.Join(m.cfg.PHP.ProfileRoot, "php"+versionTag, "php-fpm.d", "llstack-"+opts.Name+"-override.conf")
		lines = append(lines, fmt.Sprintf("; Per-site PHP overrides for %s (managed by LLStack)", opts.Name))
		lines = append(lines, fmt.Sprintf("[%s]", opts.Name))
		for _, ov := range opts.Overrides {
			lines = append(lines, fmt.Sprintf("php_admin_value[%s] = %s", ov.Key, ov.Value))
		}

	case "ols":
		// OLS: write .user.ini in site docroot
		configPath = filepath.Join(site.DocumentRoot, ".user.ini")
		lines = append(lines, "; Per-site PHP overrides (managed by LLStack)")
		for _, ov := range opts.Overrides {
			lines = append(lines, fmt.Sprintf("%s = %s", ov.Key, ov.Value))
		}

	case "lsws":
		// LSWS: write .user.ini in site docroot (LSWS supports both .htaccess and .user.ini)
		configPath = filepath.Join(site.DocumentRoot, ".user.ini")
		lines = append(lines, "; Per-site PHP overrides (managed by LLStack)")
		for _, ov := range opts.Overrides {
			lines = append(lines, fmt.Sprintf("%s = %s", ov.Key, ov.Value))
		}

	default:
		configPath = filepath.Join(site.DocumentRoot, ".user.ini")
		lines = append(lines, "; Per-site PHP overrides (managed by LLStack)")
		for _, ov := range opts.Overrides {
			lines = append(lines, fmt.Sprintf("%s = %s", ov.Key, ov.Value))
		}
	}

	return configPath, strings.Join(lines, "\n") + "\n"
}
