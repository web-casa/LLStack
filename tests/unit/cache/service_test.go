package cache_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/web-casa/llstack/internal/cache"
	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/logging"
	"github.com/web-casa/llstack/internal/system"
)

type fakeExecutor struct {
	commands []system.Command
}

func (f *fakeExecutor) Run(ctx context.Context, cmd system.Command) (system.Result, error) {
	f.commands = append(f.commands, cmd)
	return system.Result{ExitCode: 0}, nil
}

func TestInstallAndConfigureMemcached(t *testing.T) {
	cfg := testConfig(t.TempDir())
	exec := &fakeExecutor{}
	manager := cache.NewManager(cfg, logging.NewDefault(io.Discard), exec)

	if _, err := manager.Install(context.Background(), cache.InstallOptions{Provider: cache.ProviderMemcached}); err != nil {
		t.Fatalf("install memcached: %v", err)
	}
	if _, err := manager.Configure(context.Background(), cache.ConfigureOptions{
		Provider:    cache.ProviderMemcached,
		Bind:        "127.0.0.1",
		Port:        11211,
		MaxMemoryMB: 128,
	}); err != nil {
		t.Fatalf("configure memcached: %v", err)
	}

	if len(exec.commands) != 3 {
		t.Fatalf("expected dnf, systemctl enable and systemctl restart, got %d", len(exec.commands))
	}
	if _, err := os.Stat(cfg.Cache.ManagedProvidersDir + "/memcached.json"); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
	if _, err := os.Stat(cfg.Cache.MemcachedConfigPath); err != nil {
		t.Fatalf("config missing: %v", err)
	}
}

func testConfig(root string) config.RuntimeConfig {
	cfg := config.DefaultRuntimeConfig()
	cfg.Paths.BackupsDir = filepath.Join(root, "var", "lib", "llstack", "backups")
	cfg.Cache.ManagedProvidersDir = filepath.Join(root, "etc", "llstack", "cache", "providers")
	cfg.Cache.MemcachedConfigPath = filepath.Join(root, "etc", "sysconfig", "memcached")
	cfg.Cache.RedisConfigPath = filepath.Join(root, "etc", "redis.conf")
	return cfg
}
