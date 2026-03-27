package cache_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/web-casa/llstack/internal/cache"
	"github.com/web-casa/llstack/internal/cli"
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

func TestCacheLifecycleWritesManifestAndConfig(t *testing.T) {
	cfg := testConfig(t.TempDir())
	exec := &fakeExecutor{}
	manager := cache.NewManager(cfg, logging.NewDefault(io.Discard), exec)

	if _, err := manager.Install(context.Background(), cache.InstallOptions{Provider: cache.ProviderMemcached}); err != nil {
		t.Fatalf("install cache provider: %v", err)
	}
	if _, err := manager.Configure(context.Background(), cache.ConfigureOptions{
		Provider:    cache.ProviderMemcached,
		Bind:        "127.0.0.1",
		MaxMemoryMB: 192,
	}); err != nil {
		t.Fatalf("configure cache provider: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(cfg.Cache.ManagedProvidersDir, "memcached.json"))
	if err != nil {
		t.Fatalf("read cache manifest: %v", err)
	}
	if !bytes.Contains(raw, []byte(`"status": "configured"`)) {
		t.Fatalf("unexpected manifest: %s", string(raw))
	}
	if _, err := os.Stat(cfg.Cache.MemcachedConfigPath); err != nil {
		t.Fatalf("config missing: %v", err)
	}
}

func TestCacheInstallCLIJSON(t *testing.T) {
	cfg := testConfig(t.TempDir())
	exec := &fakeExecutor{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := cli.NewRoot(cli.Dependencies{
		Version: "test-version",
		Config:  cfg,
		Logger:  logging.NewDefault(&stderr),
		Exec:    exec,
		Stdin:   bytes.NewBuffer(nil),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})

	cmd := root.Command(context.Background())
	cmd.SetArgs([]string{"cache:install", "memcached", "--dry-run", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute cache:install: %v", err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"kind": "cache.install"`)) {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func testConfig(root string) config.RuntimeConfig {
	cfg := config.DefaultRuntimeConfig()
	cfg.Paths.ConfigDir = filepath.Join(root, "etc", "llstack")
	cfg.Paths.StateDir = filepath.Join(root, "var", "lib", "llstack", "state")
	cfg.Paths.HistoryDir = filepath.Join(root, "var", "lib", "llstack", "history")
	cfg.Paths.BackupsDir = filepath.Join(root, "var", "lib", "llstack", "backups")
	cfg.Paths.LogDir = filepath.Join(root, "var", "log", "llstack")
	cfg.Cache.ManagedProvidersDir = filepath.Join(root, "etc", "llstack", "cache", "providers")
	cfg.Cache.MemcachedConfigPath = filepath.Join(root, "etc", "sysconfig", "memcached")
	cfg.Cache.RedisConfigPath = filepath.Join(root, "etc", "redis.conf")
	return cfg
}
