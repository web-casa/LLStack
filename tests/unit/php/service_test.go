package php_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/logging"
	phpruntime "github.com/web-casa/llstack/internal/php"
	"github.com/web-casa/llstack/internal/system"
)

type fakeExecutor struct {
	commands []system.Command
}

func (f *fakeExecutor) Run(ctx context.Context, cmd system.Command) (system.Result, error) {
	f.commands = append(f.commands, cmd)
	return system.Result{ExitCode: 0}, nil
}

func TestResolverBuildsRemiPaths(t *testing.T) {
	cfg := testPHPConfig(t.TempDir())
	resolver := phpruntime.NewResolver(cfg)

	if got := resolver.CollectionPrefix("8.4"); got != "php84-php" {
		t.Fatalf("unexpected collection prefix: %s", got)
	}
	if got := resolver.FPMSocketPath("8.3"); !strings.Contains(got, "/php83/run/php-fpm/www.sock") {
		t.Fatalf("unexpected fpm socket path: %s", got)
	}
	if got := resolver.LSPHPCommand("8.4"); !strings.Contains(got, "/php84/root/usr/bin/lsphp") {
		t.Fatalf("unexpected lsphp command path: %s", got)
	}
}

func TestBindSiteRuntimeForApacheAndOLS(t *testing.T) {
	cfg := testPHPConfig(t.TempDir())
	resolver := phpruntime.NewResolver(cfg)

	apacheSite := model.Site{
		Backend: "apache",
		PHP: model.PHPRuntimeBinding{
			Enabled: true,
			Version: "8.3",
		},
	}
	if err := phpruntime.BindSiteRuntime(resolver, &apacheSite); err != nil {
		t.Fatalf("bind apache runtime: %v", err)
	}
	if apacheSite.PHP.Handler != "php-fpm" || apacheSite.PHP.Socket == "" {
		t.Fatalf("unexpected apache binding: %#v", apacheSite.PHP)
	}

	olsSite := model.Site{
		Backend: "ols",
		PHP: model.PHPRuntimeBinding{
			Enabled: true,
			Version: "8.4",
		},
	}
	if err := phpruntime.BindSiteRuntime(resolver, &olsSite); err != nil {
		t.Fatalf("bind ols runtime: %v", err)
	}
	if olsSite.PHP.Handler != "lsphp" || olsSite.PHP.Command == "" {
		t.Fatalf("unexpected ols binding: %#v", olsSite.PHP)
	}
}

func TestInstallRuntimeWritesManifestAndProfile(t *testing.T) {
	cfg := testPHPConfig(t.TempDir())
	exec := &fakeExecutor{}
	manager := phpruntime.NewManager(cfg, logging.NewDefault(io.Discard), exec)

	if _, err := manager.Install(context.Background(), phpruntime.InstallOptions{
		Version:      "8.4",
		Profile:      phpruntime.ProfileWP,
		Extensions:   []string{"gd", "intl"},
		IncludeFPM:   true,
		IncludeLSAPI: true,
	}); err != nil {
		t.Fatalf("install runtime: %v", err)
	}

	if len(exec.commands) != 1 {
		t.Fatalf("expected one dnf command, got %d", len(exec.commands))
	}
	manifestPath := filepath.Join(cfg.PHP.ManagedRuntimesDir, "8-4.json")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
	profilePath := filepath.Join(cfg.PHP.ProfileRoot, "php84", "php.d", "90-llstack-profile.ini")
	if _, err := os.Stat(profilePath); err != nil {
		t.Fatalf("profile snippet missing: %v", err)
	}
}

func testPHPConfig(root string) config.RuntimeConfig {
	cfg := config.DefaultRuntimeConfig()
	cfg.Paths.BackupsDir = filepath.Join(root, "var", "lib", "llstack", "backups")
	cfg.PHP.ManagedRuntimesDir = filepath.Join(root, "etc", "llstack", "php", "runtimes")
	cfg.PHP.ManagedProfilesDir = filepath.Join(root, "etc", "llstack", "php", "profiles")
	cfg.PHP.ProfileRoot = filepath.Join(root, "etc", "opt", "remi")
	cfg.PHP.RuntimeRoot = filepath.Join(root, "opt", "remi")
	cfg.PHP.StateRoot = filepath.Join(root, "var", "opt", "remi")
	cfg.PHP.ELMajorOverride = "9"
	return cfg
}
