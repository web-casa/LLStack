package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/core/apply"
	"github.com/web-casa/llstack/internal/core/plan"
	"github.com/web-casa/llstack/internal/logging"
	"github.com/web-casa/llstack/internal/system"
)

// Manager manages cache providers and manifests.
type Manager struct {
	cfg     config.RuntimeConfig
	logger  logging.Logger
	exec    system.Executor
	applier apply.FileApplier
}

// NewManager constructs a cache manager.
func NewManager(cfg config.RuntimeConfig, logger logging.Logger, exec system.Executor) Manager {
	return Manager{
		cfg:     cfg,
		logger:  logger,
		exec:    exec,
		applier: apply.NewFileApplier(cfg.Paths.BackupsDir),
	}
}

// Install plans or applies a cache provider installation.
func (m Manager) Install(ctx context.Context, opts InstallOptions) (plan.Plan, error) {
	spec, err := ResolveProvider(m.cfg, opts.Provider)
	if err != nil {
		return plan.Plan{}, err
	}
	p := plan.New("cache.install", "Install cache provider "+string(spec.Name))
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly
	p.AddOperation(plan.Operation{ID: "install-packages", Kind: "package.install", Target: strings.Join(spec.Packages, " ")})
	p.AddOperation(plan.Operation{ID: "enable-service", Kind: "service.enable", Target: spec.ServiceName})
	p.AddOperation(plan.Operation{ID: "write-provider-manifest", Kind: "write_file", Target: m.manifestPath(spec.Name)})

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}
	if err := m.runDNF(ctx, spec.Packages...); err != nil {
		return p, err
	}
	if err := m.runSystemctl(ctx, "enable", "--now", spec.ServiceName); err != nil {
		return p, err
	}
	now := time.Now().UTC()
	manifest := ProviderManifest{
		Provider:     spec.Name,
		ServiceName:  spec.ServiceName,
		Packages:     append([]string{}, spec.Packages...),
		ConfigPath:   spec.ConfigPath,
		Status:       "installed",
		Capabilities: spec.Capabilities,
		InstalledAt:  now,
		UpdatedAt:    now,
	}
	if _, err := m.applier.WriteJSON(m.manifestPath(spec.Name), manifest, 0o644); err != nil {
		return p, err
	}
	m.logger.Info("cache provider installed", "provider", spec.Name)
	return p, nil
}

// Configure writes a managed cache configuration and restarts the service.
func (m Manager) Configure(ctx context.Context, opts ConfigureOptions) (plan.Plan, error) {
	spec, manifest, err := m.resolveManifest(opts.Provider, opts.DryRun || opts.PlanOnly)
	if err != nil {
		return plan.Plan{}, err
	}
	if opts.Bind == "" {
		opts.Bind = "127.0.0.1"
	}
	if opts.Port == 0 {
		if spec.Name == ProviderRedis || spec.Name == ProviderValkey {
			opts.Port = 6379
		} else {
			opts.Port = 11211
		}
	}
	if opts.MaxMemoryMB == 0 {
		opts.MaxMemoryMB = 256
	}
	body := renderConfig(spec, opts)

	p := plan.New("cache.configure", "Configure cache provider "+string(spec.Name))
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly
	p.AddOperation(plan.Operation{ID: "write-config", Kind: "write_file", Target: spec.ConfigPath})
	p.AddOperation(plan.Operation{ID: "restart-service", Kind: "service.restart", Target: spec.ServiceName})
	p.AddOperation(plan.Operation{ID: "write-provider-manifest", Kind: "write_file", Target: m.manifestPath(spec.Name)})

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}
	if err := os.MkdirAll(filepath.Dir(spec.ConfigPath), 0o755); err != nil {
		return p, err
	}
	if _, err := m.applier.WriteFile(spec.ConfigPath, []byte(body), 0o644); err != nil {
		return p, err
	}
	if err := m.runSystemctl(ctx, "restart", spec.ServiceName); err != nil {
		return p, err
	}
	manifest.Bind = opts.Bind
	manifest.Port = opts.Port
	manifest.MaxMemoryMB = opts.MaxMemoryMB
	manifest.ConfigPath = spec.ConfigPath
	manifest.Status = "configured"
	manifest.UpdatedAt = time.Now().UTC()
	if _, err := m.applier.WriteJSON(m.manifestPath(spec.Name), manifest, 0o644); err != nil {
		return p, err
	}
	return p, nil
}

// Status returns all managed cache provider manifests.
func (m Manager) Status() ([]ProviderManifest, error) {
	dir := m.cfg.Cache.ManagedProvidersDir
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]ProviderManifest, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var manifest ProviderManifest
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return nil, err
		}
		out = append(out, manifest)
	}
	slices.SortFunc(out, func(a, b ProviderManifest) int {
		return strings.Compare(string(a.Provider), string(b.Provider))
	})
	return out, nil
}

func (m Manager) resolveManifest(provider ProviderName, allowMissing bool) (ProviderSpec, ProviderManifest, error) {
	if provider == "" {
		manifests, err := m.Status()
		if err != nil {
			return ProviderSpec{}, ProviderManifest{}, err
		}
		if len(manifests) != 1 {
			return ProviderSpec{}, ProviderManifest{}, fmt.Errorf("provider is required when multiple or zero cache providers are managed")
		}
		provider = manifests[0].Provider
	}
	spec, err := ResolveProvider(m.cfg, provider)
	if err != nil {
		return ProviderSpec{}, ProviderManifest{}, err
	}
	raw, err := os.ReadFile(m.manifestPath(provider))
	if err != nil {
		if allowMissing && os.IsNotExist(err) {
			return spec, ProviderManifest{
				Provider:     spec.Name,
				ServiceName:  spec.ServiceName,
				ConfigPath:   spec.ConfigPath,
				Status:       "planned",
				Capabilities: spec.Capabilities,
			}, nil
		}
		return ProviderSpec{}, ProviderManifest{}, err
	}
	var manifest ProviderManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return ProviderSpec{}, ProviderManifest{}, err
	}
	return spec, manifest, nil
}

func (m Manager) manifestPath(provider ProviderName) string {
	return filepath.Join(m.cfg.Cache.ManagedProvidersDir, string(provider)+".json")
}

func (m Manager) runDNF(ctx context.Context, packages ...string) error {
	result, err := m.exec.Run(ctx, system.Command{Name: "dnf", Args: append([]string{"install", "-y"}, packages...)})
	if err != nil {
		return fmt.Errorf("dnf install: %w (%s)", err, result.Stderr)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("dnf exited with %d: %s", result.ExitCode, result.Stderr)
	}
	return nil
}

func (m Manager) runSystemctl(ctx context.Context, args ...string) error {
	result, err := m.exec.Run(ctx, system.Command{Name: "systemctl", Args: args})
	if err != nil {
		return fmt.Errorf("systemctl %s: %w (%s)", strings.Join(args, " "), err, result.Stderr)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("systemctl %s exited with %d: %s", strings.Join(args, " "), result.ExitCode, result.Stderr)
	}
	return nil
}

func renderConfig(spec ProviderSpec, opts ConfigureOptions) string {
	switch spec.Name {
	case ProviderMemcached:
		return fmt.Sprintf("PORT=\"%d\"\nUSER=\"memcached\"\nMAXCONN=\"1024\"\nCACHESIZE=\"%d\"\nOPTIONS=\"-l %s\"\n", opts.Port, opts.MaxMemoryMB, opts.Bind)
	case ProviderRedis:
		return fmt.Sprintf("bind %s\nport %d\nmaxmemory %dmb\nmaxmemory-policy allkeys-lru\n", opts.Bind, opts.Port, opts.MaxMemoryMB)
	case ProviderValkey:
		return fmt.Sprintf("bind %s\nport %d\nmaxmemory %dmb\nmaxmemory-policy allkeys-lru\n", opts.Bind, opts.Port, opts.MaxMemoryMB)
	default:
		return ""
	}
}
