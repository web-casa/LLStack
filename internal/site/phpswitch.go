package site

import (
	"context"
	"fmt"
	"strings"

	"github.com/web-casa/llstack/internal/core/plan"
)

// PHPSwitchOptions controls PHP version switching for a site.
type PHPSwitchOptions struct {
	Name       string
	NewVersion string
	DryRun     bool
	SkipReload bool
}

// SwitchPHP changes the PHP version for a site, updating vhost config and FPM socket reference.
func (m Manager) SwitchPHP(ctx context.Context, opts PHPSwitchOptions) (plan.Plan, error) {
	p := plan.New("site.php-switch", fmt.Sprintf("Switch PHP version for %s to %s", opts.Name, opts.NewVersion))
	p.DryRun = opts.DryRun

	manifest, err := m.loadManifest(opts.Name)
	if err != nil {
		return p, fmt.Errorf("site %q not found", opts.Name)
	}

	if !manifest.Site.PHP.Enabled {
		return p, fmt.Errorf("PHP is not enabled for site %q", opts.Name)
	}

	oldVersion := manifest.Site.PHP.Version
	if oldVersion == opts.NewVersion {
		p.Summary = fmt.Sprintf("site %s already uses PHP %s", opts.Name, opts.NewVersion)
		return p, nil
	}

	// Validate new version exists in supported list
	validVersions := []string{"7.4", "8.0", "8.1", "8.2", "8.3", "8.4", "8.5"}
	valid := false
	for _, v := range validVersions {
		if v == opts.NewVersion {
			valid = true
			break
		}
	}
	if !valid {
		return p, fmt.Errorf("unsupported PHP version %q", opts.NewVersion)
	}

	p.AddOperation(plan.Operation{
		ID: "switch-php-version", Kind: "site.php.switch",
		Target: opts.Name,
		Details: map[string]string{
			"old_version": oldVersion,
			"new_version": opts.NewVersion,
		},
	})
	p.AddOperation(plan.Operation{
		ID: "rerender-vhost", Kind: "site.rerender",
		Target: manifest.VHostPath,
	})

	if opts.DryRun {
		return p, nil
	}

	// Update site model
	updated := manifest.Site
	updated.PHP.Version = opts.NewVersion

	// Update FPM socket path
	oldTag := strings.ReplaceAll(oldVersion, ".", "")
	newTag := strings.ReplaceAll(opts.NewVersion, ".", "")
	if updated.PHP.Socket != "" {
		updated.PHP.Socket = strings.ReplaceAll(updated.PHP.Socket, "php"+oldTag, "php"+newTag)
	}
	if updated.PHP.FPMService != "" {
		updated.PHP.FPMService = strings.ReplaceAll(updated.PHP.FPMService, "php"+oldTag, "php"+newTag)
	}
	if updated.PHP.Command != "" {
		updated.PHP.Command = strings.ReplaceAll(updated.PHP.Command, oldTag, newTag)
	}

	// Re-render and apply
	renderer, verifier, renderOpts, err := m.backendComponents(ctx, updated)
	if err != nil {
		return p, err
	}
	renderOpts.SystemUser = manifest.SystemUser

	rendered, err := renderer.RenderSite(updated, renderOpts)
	if err != nil {
		return p, err
	}

	for _, asset := range rendered.Assets {
		if _, err := m.applier.WriteFile(asset.Path, asset.Content, asset.Mode); err != nil {
			return p, err
		}
	}

	// Update manifest
	manifest.Site = updated
	if _, err := m.applier.WriteJSON(m.manifestPath(opts.Name), manifest, 0o644); err != nil {
		return p, err
	}

	if !opts.SkipReload {
		if err := verifier.ConfigTest(ctx); err != nil {
			return p, err
		}
		if err := verifier.Reload(ctx); err != nil {
			return p, err
		}
	}

	return p, nil
}
