package php

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/core/apply"
	"github.com/web-casa/llstack/internal/core/plan"
	"github.com/web-casa/llstack/internal/logging"
	"github.com/web-casa/llstack/internal/system"
)

// Manager manages PHP runtimes backed by Remi packages.
type Manager struct {
	cfg      config.RuntimeConfig
	logger   logging.Logger
	exec     system.Executor
	applier  apply.FileApplier
	resolver Resolver
}

// NewManager constructs a PHP manager.
func NewManager(cfg config.RuntimeConfig, logger logging.Logger, exec system.Executor) Manager {
	return Manager{
		cfg:      cfg,
		logger:   logger,
		exec:     exec,
		applier:  apply.NewFileApplier(cfg.Paths.BackupsDir),
		resolver: NewResolver(cfg),
	}
}

// Install plans or applies a PHP runtime installation.
func (m Manager) Install(ctx context.Context, opts InstallOptions) (plan.Plan, error) {
	if opts.Profile == "" {
		opts.Profile = ProfileGeneric
	}
	basePackages, err := m.resolver.BasePackages(opts.Version, opts.IncludeFPM, opts.IncludeLSAPI)
	if err != nil {
		return plan.Plan{}, err
	}
	extensions := append([]string{}, m.cfg.PHP.DefaultExtensions...)
	extensions = append(extensions, opts.Extensions...)
	extensions = uniqueSorted(extensions)
	extensionPackages, err := m.resolver.ExtensionPackages(opts.Version, extensions)
	if err != nil {
		return plan.Plan{}, err
	}
	allPackages := uniqueSorted(append(append([]string{}, basePackages...), extensionPackages...))
	profileBody, err := RenderProfile(opts.Profile)
	if err != nil {
		return plan.Plan{}, err
	}
	profilePath := m.resolver.ManagedProfilePath(opts.Version)
	manifestPath := m.runtimeManifestPath(opts.Version)
	elMajor := m.cfg.PHP.ELMajorOverride
	if elMajor == "" {
		elMajor = "9"
	}

	p := plan.New("php.install", "Install PHP runtime "+opts.Version)
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly
	if isEOLVersion(opts.Version) {
		p.Warnings = append(p.Warnings, fmt.Sprintf("PHP %s has reached end-of-life and no longer receives security updates", opts.Version))
	}
	p.AddOperation(plan.Operation{
		ID:     "install-remi-release",
		Kind:   "repo.install",
		Target: m.resolver.RemiReleaseURL(elMajor),
	})
	p.AddOperation(plan.Operation{
		ID:      "install-runtime-packages",
		Kind:    "package.install",
		Target:  strings.Join(allPackages, " "),
		Details: map[string]string{"version": opts.Version},
	})
	p.AddOperation(plan.Operation{
		ID:     "write-profile",
		Kind:   "write_file",
		Target: profilePath,
	})
	p.AddOperation(plan.Operation{
		ID:     "write-runtime-manifest",
		Kind:   "write_file",
		Target: manifestPath,
	})

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	if err := m.runDNF(ctx, append([]string{m.resolver.RemiReleaseURL(elMajor)}, allPackages...)...); err != nil {
		return p, err
	}
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
		return p, err
	}
	if _, err := m.applier.WriteFile(profilePath, []byte(profileBody), 0o644); err != nil {
		return p, err
	}

	manifest := RuntimeManifest{
		Version:     opts.Version,
		Packages:    allPackages,
		Extensions:  extensions,
		Profile:     opts.Profile,
		ProfilePath: profilePath,
		InstalledAt: time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if _, err := m.applier.WriteJSON(manifestPath, manifest, 0o644); err != nil {
		return p, err
	}

	// Ensure Apache can access FPM socket by setting listen.acl_users
	if opts.IncludeFPM {
		versionTag := strings.ReplaceAll(opts.Version, ".", "")
		wwwConf := filepath.Join(m.cfg.PHP.ProfileRoot, "php"+versionTag, "php-fpm.d", "www.conf")
		if raw, err := os.ReadFile(wwwConf); err == nil {
			content := string(raw)
			if strings.Contains(content, "listen.acl_users") && !strings.Contains(content, "apache") {
				content = strings.Replace(content, "listen.acl_users =", "listen.acl_users = apache,", 1)
				os.WriteFile(wwwConf, []byte(content), 0o644)
			}
		}
	}

	m.logger.Info("php runtime installed", "version", opts.Version)
	return p, nil
}

// List returns all managed runtime manifests.
func (m Manager) List() ([]RuntimeManifest, error) {
	dir := m.cfg.PHP.ManagedRuntimesDir
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	out := make([]RuntimeManifest, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var manifest RuntimeManifest
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return nil, err
		}
		out = append(out, manifest)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// ConfigureExtensions installs extra extensions for an existing runtime.
func (m Manager) ConfigureExtensions(ctx context.Context, opts ExtensionsOptions) (plan.Plan, error) {
	manifest, err := m.loadRuntime(opts.Version)
	if err != nil {
		return plan.Plan{}, err
	}
	extensions := uniqueSorted(append(manifest.Extensions, opts.Extensions...))
	packages, err := m.resolver.ExtensionPackages(opts.Version, extensions)
	if err != nil {
		return plan.Plan{}, err
	}

	p := plan.New("php.extensions", "Configure PHP extensions for "+opts.Version)
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly
	p.AddOperation(plan.Operation{
		ID:     "install-extension-packages",
		Kind:   "package.install",
		Target: strings.Join(packages, " "),
	})
	p.AddOperation(plan.Operation{
		ID:     "write-runtime-manifest",
		Kind:   "write_file",
		Target: m.runtimeManifestPath(opts.Version),
	})

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	if err := m.runDNF(ctx, packages...); err != nil {
		return p, err
	}
	manifest.Extensions = extensions
	manifest.Packages = uniqueSorted(append(manifest.Packages, packages...))
	manifest.UpdatedAt = time.Now().UTC()
	if _, err := m.applier.WriteJSON(m.runtimeManifestPath(opts.Version), manifest, 0o644); err != nil {
		return p, err
	}
	return p, nil
}

// ApplyProfile writes a managed php.ini snippet for a runtime.
func (m Manager) ApplyProfile(ctx context.Context, opts ProfileOptions) (plan.Plan, error) {
	_ = ctx
	manifest, err := m.loadRuntime(opts.Version)
	if err != nil {
		return plan.Plan{}, err
	}
	body, err := RenderProfile(opts.Profile)
	if err != nil {
		return plan.Plan{}, err
	}
	path := m.resolver.ManagedProfilePath(opts.Version)

	p := plan.New("php.ini", "Apply PHP profile "+string(opts.Profile)+" to "+opts.Version)
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly
	p.AddOperation(plan.Operation{ID: "write-profile", Kind: "write_file", Target: path})
	p.AddOperation(plan.Operation{ID: "write-runtime-manifest", Kind: "write_file", Target: m.runtimeManifestPath(opts.Version)})

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return p, err
	}
	if _, err := m.applier.WriteFile(path, []byte(body), 0o644); err != nil {
		return p, err
	}
	manifest.Profile = opts.Profile
	manifest.ProfilePath = path
	manifest.UpdatedAt = time.Now().UTC()
	if _, err := m.applier.WriteJSON(m.runtimeManifestPath(opts.Version), manifest, 0o644); err != nil {
		return p, err
	}
	return p, nil
}

func (m Manager) runDNF(ctx context.Context, packages ...string) error {
	if len(packages) == 0 {
		return nil
	}
	args := append([]string{"install", "-y"}, packages...)
	result, err := m.exec.Run(ctx, system.Command{Name: "dnf", Args: args})
	if err != nil {
		return fmt.Errorf("dnf install: %w (%s)", err, result.Stderr)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("dnf exited with %d: %s", result.ExitCode, result.Stderr)
	}
	return nil
}

func (m Manager) runtimeManifestPath(version string) string {
	return filepath.Join(m.cfg.PHP.ManagedRuntimesDir, strings.ReplaceAll(version, ".", "-")+".json")
}

func (m Manager) loadRuntime(version string) (RuntimeManifest, error) {
	raw, err := os.ReadFile(m.runtimeManifestPath(version))
	if err != nil {
		return RuntimeManifest{}, err
	}
	var manifest RuntimeManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return RuntimeManifest{}, err
	}
	return manifest, nil
}

// UninstallOptions controls PHP runtime removal.
type UninstallOptions struct {
	Version  string
	DryRun   bool
	PlanOnly bool
}

// Uninstall plans or applies a PHP runtime removal.
func (m Manager) Uninstall(ctx context.Context, opts UninstallOptions) (plan.Plan, error) {
	p := plan.New("php.uninstall", fmt.Sprintf("Remove PHP %s runtime", opts.Version))
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly

	manifestPath := filepath.Join(m.cfg.PHP.ManagedRuntimesDir, strings.ReplaceAll(opts.Version, ".", "-")+".json")
	manifest, err := m.loadRuntime(opts.Version)
	if err != nil {
		return plan.Plan{}, fmt.Errorf("PHP %s runtime manifest not found", opts.Version)
	}

	serviceName := m.resolver.FPMServiceName(opts.Version)
	p.AddOperation(plan.Operation{ID: "stop-fpm-" + opts.Version, Kind: "service.stop", Target: serviceName})
	p.AddOperation(plan.Operation{ID: "disable-fpm-" + opts.Version, Kind: "service.disable", Target: serviceName})

	for _, pkg := range manifest.Packages {
		p.AddOperation(plan.Operation{ID: "remove-pkg-" + pkg, Kind: "package.remove", Target: pkg})
	}

	p.AddOperation(plan.Operation{ID: "remove-manifest-" + opts.Version, Kind: "file.delete", Target: manifestPath})
	if manifest.ProfilePath != "" {
		p.AddOperation(plan.Operation{ID: "remove-profile-" + opts.Version, Kind: "file.delete", Target: manifest.ProfilePath})
	}

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	if _, err := m.exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"stop", serviceName}}); err != nil {
		m.logger.Info("systemctl stop skipped", "service", serviceName, "error", err)
	}
	if _, err := m.exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"disable", serviceName}}); err != nil {
		m.logger.Info("systemctl disable skipped", "service", serviceName, "error", err)
	}

	if len(manifest.Packages) > 0 {
		args := append([]string{"-y", "remove"}, manifest.Packages...)
		if _, err := m.exec.Run(ctx, system.Command{Name: "dnf", Args: args}); err != nil {
			return p, fmt.Errorf("dnf remove failed: %w", err)
		}
	}

	os.Remove(manifestPath)
	if manifest.ProfilePath != "" {
		os.Remove(manifest.ProfilePath)
	}

	m.logger.Info("php runtime uninstalled", "version", opts.Version)
	return p, nil
}

// PoolTuneOptions controls PHP-FPM pool tuning.
type PoolTuneOptions struct {
	Version      string
	MaxChildren  int
	StartServers int
	MinSpare     int
	MaxSpare     int
	DryRun       bool
	PlanOnly     bool
}

// TunePool writes a managed FPM pool config snippet.
func (m Manager) TunePool(ctx context.Context, opts PoolTuneOptions) (plan.Plan, error) {
	p := plan.New("php.pool.tune", fmt.Sprintf("Tune PHP %s FPM pool", opts.Version))
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly

	if opts.MaxChildren <= 0 {
		opts.MaxChildren = 50
	}
	if opts.StartServers <= 0 {
		opts.StartServers = 5
	}
	if opts.MinSpare <= 0 {
		opts.MinSpare = 5
	}
	if opts.MaxSpare <= 0 {
		opts.MaxSpare = 35
	}

	versionTag := strings.ReplaceAll(opts.Version, ".", "")
	poolConfigPath := filepath.Join(m.cfg.PHP.ProfileRoot, "php"+versionTag, "php-fpm.d", "90-llstack-pool.conf")
	content := fmt.Sprintf("; Managed by LLStack\npm.max_children = %d\npm.start_servers = %d\npm.min_spare_servers = %d\npm.max_spare_servers = %d\n",
		opts.MaxChildren, opts.StartServers, opts.MinSpare, opts.MaxSpare)

	p.AddOperation(plan.Operation{
		ID:     "write-pool-config-" + opts.Version,
		Kind:   "file.write",
		Target: poolConfigPath,
		Details: map[string]string{
			"pm.max_children":    fmt.Sprintf("%d", opts.MaxChildren),
			"pm.start_servers":   fmt.Sprintf("%d", opts.StartServers),
			"pm.min_spare_servers": fmt.Sprintf("%d", opts.MinSpare),
			"pm.max_spare_servers": fmt.Sprintf("%d", opts.MaxSpare),
		},
	})

	serviceName := m.resolver.FPMServiceName(opts.Version)
	p.AddOperation(plan.Operation{ID: "restart-fpm-" + opts.Version, Kind: "service.restart", Target: serviceName})

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	if err := os.MkdirAll(filepath.Dir(poolConfigPath), 0o755); err != nil {
		return p, err
	}
	if _, err := m.applier.WriteFile(poolConfigPath, []byte(content), 0o644); err != nil {
		return p, err
	}
	if _, err := m.exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"restart", serviceName}}); err != nil {
		return p, fmt.Errorf("fpm restart failed: %w", err)
	}

	m.logger.Info("php fpm pool tuned", "version", opts.Version, "max_children", opts.MaxChildren)
	return p, nil
}

func isEOLVersion(version string) bool {
	switch version {
	case "7.4", "8.0", "8.1":
		return true
	default:
		return false
	}
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
