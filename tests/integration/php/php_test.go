package php_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/web-casa/llstack/internal/cli"
	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/logging"
	phpruntime "github.com/web-casa/llstack/internal/php"
	"github.com/web-casa/llstack/internal/site"
	"github.com/web-casa/llstack/internal/system"
)

type fakeExecutor struct {
	commands []system.Command
}

func (f *fakeExecutor) Run(ctx context.Context, cmd system.Command) (system.Result, error) {
	f.commands = append(f.commands, cmd)
	return system.Result{ExitCode: 0}, nil
}

func TestSitePHPVersionSwitchUpdatesManifest(t *testing.T) {
	cfg := testConfig(t.TempDir())
	exec := &fakeExecutor{}
	phpManager := phpruntime.NewManager(cfg, logging.NewDefault(io.Discard), exec)
	if _, err := phpManager.Install(context.Background(), phpruntime.InstallOptions{
		Version:      "8.4",
		Profile:      phpruntime.ProfileGeneric,
		IncludeFPM:   true,
		IncludeLSAPI: true,
	}); err != nil {
		t.Fatalf("install php runtime: %v", err)
	}

	siteManager := site.NewManager(cfg, logging.NewDefault(io.Discard), exec)
	if _, err := siteManager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "example.com",
			Domain: model.DomainBinding{
				ServerName: "example.com",
			},
			PHP: model.PHPRuntimeBinding{
				Enabled: true,
				Version: "8.3",
			},
		},
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	if _, err := siteManager.UpdatePHPVersion(context.Background(), site.UpdatePHPOptions{
		Name:       "example.com",
		Version:    "8.4",
		SkipReload: true,
	}); err != nil {
		t.Fatalf("update site php: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(cfg.Paths.ManagedSitesDir(), "example.com.json"))
	if err != nil {
		t.Fatalf("read site manifest: %v", err)
	}
	if !bytes.Contains(raw, []byte(`"version": "8.4"`)) {
		t.Fatalf("expected site manifest to contain php version 8.4, got %s", string(raw))
	}
}

func TestPHPInstallCLIJSON(t *testing.T) {
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
	cmd.SetArgs([]string{"php:install", "8.4", "--profile", "wp", "--extensions", "gd,intl", "--dry-run", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute php:install: %v", err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"kind": "php.install"`)) {
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
	cfg.Paths.SitesRootDir = filepath.Join(root, "data", "www")
	cfg.Apache.ManagedVhostsDir = filepath.Join(root, "etc", "httpd", "conf.d", "llstack", "sites")
	cfg.Apache.ConfigTestCmd = []string{"apachectl", "configtest"}
	cfg.Apache.ReloadCmd = []string{"systemctl", "reload", "httpd"}
	cfg.OLS.ManagedVhostsRoot = filepath.Join(root, "usr", "local", "lsws", "conf", "vhosts")
	cfg.OLS.ManagedListenersDir = filepath.Join(root, "usr", "local", "lsws", "conf", "llstack", "listeners")
	cfg.OLS.DefaultPHPBinary = "/usr/bin/lsphp"
	cfg.OLS.ConfigTestCmd = []string{"lswsctrl", "configtest"}
	cfg.OLS.ReloadCmd = []string{"lswsctrl", "reload"}
	cfg.LSWS.ManagedIncludesDir = filepath.Join(root, "usr", "local", "lsws", "conf", "llstack", "includes")
	cfg.LSWS.DefaultPHPBinary = "/usr/bin/lsphp"
	cfg.LSWS.LicenseSerialFile = filepath.Join(root, "usr", "local", "lsws", "conf", "serial.no")
	cfg.LSWS.DetectCmd = []string{"lshttpd", "-v"}
	cfg.LSWS.ConfigTestCmd = []string{"lswsctrl", "configtest"}
	cfg.LSWS.ReloadCmd = []string{"lswsctrl", "reload"}
	cfg.PHP.ManagedRuntimesDir = filepath.Join(root, "etc", "llstack", "php", "runtimes")
	cfg.PHP.ManagedProfilesDir = filepath.Join(root, "etc", "llstack", "php", "profiles")
	cfg.PHP.ProfileRoot = filepath.Join(root, "etc", "opt", "remi")
	cfg.PHP.RuntimeRoot = filepath.Join(root, "opt", "remi")
	cfg.PHP.StateRoot = filepath.Join(root, "var", "opt", "remi")
	cfg.PHP.ELMajorOverride = "9"
	return cfg
}
