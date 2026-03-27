package cli_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/web-casa/llstack/internal/buildinfo"
	"github.com/web-casa/llstack/internal/cli"
	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/logging"
	"github.com/web-casa/llstack/internal/rollback"
	"github.com/web-casa/llstack/internal/site"
	"github.com/web-casa/llstack/internal/system"
)

func TestVersionCommandJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := cli.NewRoot(cli.Dependencies{
		Version: "test-version",
		Build: buildinfo.Info{
			Version:    "test-version",
			Commit:     "abc1234",
			BuildDate:  "2026-03-26T00:00:00Z",
			TargetOS:   "linux",
			TargetArch: "amd64",
			GoVersion:  "go1.24.4",
		},
		Config: config.DefaultRuntimeConfig(),
		Logger: logging.NewDefault(&stderr),
		Exec:   system.NewLocalExecutor(),
		Stdin:  bytes.NewBuffer(nil),
		Stdout: &stdout,
		Stderr: &stderr,
	})

	cmd := root.Command(context.Background())
	cmd.SetArgs([]string{"version", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute version: %v", err)
	}

	output := stdout.String()
	if output == "" {
		t.Fatal("expected output from version command")
	}

	if want := `"version": "test-version"`; !bytes.Contains(stdout.Bytes(), []byte(want)) {
		t.Fatalf("expected output to contain %s, got %s", want, output)
	}
	if want := `"commit": "abc1234"`; !bytes.Contains(stdout.Bytes(), []byte(want)) {
		t.Fatalf("expected output to contain %s, got %s", want, output)
	}
	if want := `"target_os": "linux"`; !bytes.Contains(stdout.Bytes(), []byte(want)) {
		t.Fatalf("expected output to contain %s, got %s", want, output)
	}
}

func TestRootHelpShowsCommandGroupsAndExamples(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := cli.NewRoot(cli.Dependencies{
		Version: "test-version",
		Config:  config.DefaultRuntimeConfig(),
		Logger:  logging.NewDefault(&stderr),
		Exec:    system.NewLocalExecutor(),
		Stdin:   bytes.NewBuffer(nil),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})

	cmd := root.Command(context.Background())
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute root help: %v", err)
	}

	output := stdout.String()
	for _, want := range []string{
		"Getting Started",
		"Site Lifecycle",
		"PHP, Database, and Cache",
		"Diagnostics and Recovery",
		"Interfaces and Metadata",
		"llstack install --config examples/install/basic.yaml --dry-run",
	} {
		if !bytes.Contains(stdout.Bytes(), []byte(want)) {
			t.Fatalf("expected root help to contain %q, got %s", want, output)
		}
	}
}

func TestInstallHelpShowsConfigExamples(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := cli.NewRoot(cli.Dependencies{
		Version: "test-version",
		Config:  config.DefaultRuntimeConfig(),
		Logger:  logging.NewDefault(&stderr),
		Exec:    system.NewLocalExecutor(),
		Stdin:   bytes.NewBuffer(nil),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})

	cmd := root.Command(context.Background())
	cmd.SetArgs([]string{"install", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute install help: %v", err)
	}

	output := stdout.String()
	for _, want := range []string{
		"Build or apply an installation plan for LLStack",
		"--config",
		"examples/install/basic.yaml",
	} {
		if !bytes.Contains(stdout.Bytes(), []byte(want)) {
			t.Fatalf("expected install help to contain %q, got %s", want, output)
		}
	}
}

func TestInstallUnknownFlagShowsHelpHint(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := cli.NewRoot(cli.Dependencies{
		Version: "test-version",
		Config:  config.DefaultRuntimeConfig(),
		Logger:  logging.NewDefault(&stderr),
		Exec:    system.NewLocalExecutor(),
		Stdin:   bytes.NewBuffer(nil),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})

	cmd := root.Command(context.Background())
	cmd.SetArgs([]string{"install", "--unknown-flag"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected install command with unknown flag to fail")
	}
	if want := "run 'llstack install --help' for usage"; !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error to contain %q, got %v", want, err)
	}
}

func TestListCommandsShowEmptyStateHints(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := cli.NewRoot(cli.Dependencies{
		Version: "test-version",
		Config:  testCLIConfig(t.TempDir()),
		Logger:  logging.NewDefault(&stderr),
		Exec:    system.NewLocalExecutor(),
		Stdin:   bytes.NewBuffer(nil),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})

	for _, args := range [][]string{
		{"site:list"},
		{"php:list"},
		{"db:list"},
		{"cache:status"},
	} {
		stdout.Reset()
		cmd := root.Command(context.Background())
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute %v: %v", args, err)
		}
		if !strings.Contains(stdout.String(), "hint:") {
			t.Fatalf("expected empty-state hint in %v output, got %s", args, stdout.String())
		}
	}
}

func TestFormatErrorAddsManagedSiteHint(t *testing.T) {
	formatted := cli.FormatError(errors.New(`managed site "missing.example.com" not found`))
	for _, want := range []string{
		"managed site",
		"llstack site:list",
		"llstack site:create <server-name> --dry-run",
	} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("expected formatted error to contain %q, got %s", want, formatted)
		}
	}
}

func TestSiteCreateDryRunJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := cli.NewRoot(cli.Dependencies{
		Version: "test-version",
		Config:  config.DefaultRuntimeConfig(),
		Logger:  logging.NewDefault(&stderr),
		Exec:    system.NewLocalExecutor(),
		Stdin:   bytes.NewBuffer(nil),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})

	cmd := root.Command(context.Background())
	cmd.SetArgs([]string{"site:create", "example.com", "--non-interactive", "--dry-run", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute site:create: %v", err)
	}
	if want := `"kind": "site.create"`; !bytes.Contains(stdout.Bytes(), []byte(want)) {
		t.Fatalf("expected output to contain %s, got %s", want, stdout.String())
	}
}

func TestSiteCreateOLSDryRunJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := cli.NewRoot(cli.Dependencies{
		Version: "test-version",
		Config:  config.DefaultRuntimeConfig(),
		Logger:  logging.NewDefault(&stderr),
		Exec:    system.NewLocalExecutor(),
		Stdin:   bytes.NewBuffer(nil),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})

	cmd := root.Command(context.Background())
	cmd.SetArgs([]string{"site:create", "ols.example.com", "--backend", "ols", "--non-interactive", "--dry-run", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute ols site:create: %v", err)
	}
	if want := `.ols.json`; !bytes.Contains(stdout.Bytes(), []byte(want)) {
		t.Fatalf("expected output to contain %s, got %s", want, stdout.String())
	}
}

func TestSiteCreateLSWSDryRunJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := cli.NewRoot(cli.Dependencies{
		Version: "test-version",
		Config:  config.DefaultRuntimeConfig(),
		Logger:  logging.NewDefault(&stderr),
		Exec:    system.NewLocalExecutor(),
		Stdin:   bytes.NewBuffer(nil),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})

	cmd := root.Command(context.Background())
	cmd.SetArgs([]string{"site:create", "lsws.example.com", "--backend", "lsws", "--non-interactive", "--dry-run", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute lsws site:create: %v", err)
	}
	if want := `.lsws.json`; !bytes.Contains(stdout.Bytes(), []byte(want)) {
		t.Fatalf("expected output to contain %s, got %s", want, stdout.String())
	}
}

func TestInstallDryRunJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := cli.NewRoot(cli.Dependencies{
		Version: "test-version",
		Config:  config.DefaultRuntimeConfig(),
		Logger:  logging.NewDefault(&stderr),
		Exec:    system.NewLocalExecutor(),
		Stdin:   bytes.NewBuffer(nil),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})

	cmd := root.Command(context.Background())
	cmd.SetArgs([]string{"install", "--backend", "apache", "--php_version", "8.3", "--db", "mariadb", "--site", "example.com", "--dry-run", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute install: %v", err)
	}
	if want := `"kind": "install"`; !bytes.Contains(stdout.Bytes(), []byte(want)) {
		t.Fatalf("expected output to contain %s, got %s", want, stdout.String())
	}
}

func TestInstallDryRunJSONFromConfigAndFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	rootDir := t.TempDir()
	configPath := filepath.Join(rootDir, "install.yaml")
	if err := os.WriteFile(configPath, []byte(`
backend: ols
php:
  primary_version: "8.4"
database:
  provider: postgresql
  tls: required
first_site:
  domain: config.example.com
  profile: static
execution:
  dry_run: true
`), 0o644); err != nil {
		t.Fatalf("write install config: %v", err)
	}

	root := cli.NewRoot(cli.Dependencies{
		Version: "test-version",
		Config:  config.DefaultRuntimeConfig(),
		Logger:  logging.NewDefault(&stderr),
		Exec:    system.NewLocalExecutor(),
		Stdin:   bytes.NewBuffer(nil),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})

	cmd := root.Command(context.Background())
	cmd.SetArgs([]string{"install", "--config", configPath, "--backend", "apache", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute install with config: %v", err)
	}
	output := stdout.String()
	if !bytes.Contains(stdout.Bytes(), []byte(`"target": "apache"`)) {
		t.Fatalf("expected CLI backend override in output, got %s", output)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"provider": "postgresql"`)) {
		t.Fatalf("expected config-driven db install in output, got %s", output)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`config.example.com`)) {
		t.Fatalf("expected config-driven site in output, got %s", output)
	}
}

func TestSiteUpdateDryRunJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := cli.NewRoot(cli.Dependencies{
		Version: "test-version",
		Config:  config.DefaultRuntimeConfig(),
		Logger:  logging.NewDefault(&stderr),
		Exec:    system.NewLocalExecutor(),
		Stdin:   bytes.NewBuffer(nil),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})

	cmd := root.Command(context.Background())
	cmd.SetArgs([]string{"site:update", "missing.example.com", "--docroot", "/data/www/missing.example.com", "--alias", "www.missing.example.com", "--dry-run", "--json"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected site:update against missing site to fail")
	}
}

func TestSiteDiffCommandJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := cli.NewRoot(cli.Dependencies{
		Version: "test-version",
		Config:  config.DefaultRuntimeConfig(),
		Logger:  logging.NewDefault(&stderr),
		Exec:    system.NewLocalExecutor(),
		Stdin:   bytes.NewBuffer(nil),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})

	cmd := root.Command(context.Background())
	cmd.SetArgs([]string{"site:diff", "missing.example.com", "--json"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected diff command against missing site to fail")
	}
}

func TestRepairDryRunJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := cli.NewRoot(cli.Dependencies{
		Version: "test-version",
		Config:  config.DefaultRuntimeConfig(),
		Logger:  logging.NewDefault(&stderr),
		Exec:    system.NewLocalExecutor(),
		Stdin:   bytes.NewBuffer(nil),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})

	cmd := root.Command(context.Background())
	cmd.SetArgs([]string{"repair", "--dry-run", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute repair: %v", err)
	}
	if want := `"kind": "repair"`; !bytes.Contains(stdout.Bytes(), []byte(want)) {
		t.Fatalf("expected output to contain %s, got %s", want, stdout.String())
	}
}

func TestRepairDryRunTextShowsWarningsAndOperationDetails(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cfg := config.DefaultRuntimeConfig()
	rootDir := t.TempDir()
	cfg.Paths.ConfigDir = filepath.Join(rootDir, "etc", "llstack")
	cfg.Paths.StateDir = filepath.Join(rootDir, "var", "lib", "llstack", "state")
	cfg.Paths.HistoryDir = filepath.Join(rootDir, "var", "lib", "llstack", "history")
	cfg.Paths.BackupsDir = filepath.Join(rootDir, "var", "lib", "llstack", "backups")
	cfg.Paths.LogDir = filepath.Join(rootDir, "var", "log", "llstack")
	cfg.Paths.SitesRootDir = filepath.Join(rootDir, "data", "www")
	cfg.Apache.ManagedVhostsDir = filepath.Join(rootDir, "etc", "httpd", "conf.d", "llstack", "sites")
	cfg.OLS.ManagedVhostsRoot = filepath.Join(rootDir, "usr", "local", "lsws", "conf", "vhosts")
	cfg.OLS.ManagedListenersDir = filepath.Join(rootDir, "usr", "local", "lsws", "conf", "llstack", "listeners")
	cfg.LSWS.ManagedIncludesDir = filepath.Join(rootDir, "usr", "local", "lsws", "conf", "llstack", "includes")
	cfg.PHP.ManagedRuntimesDir = filepath.Join(rootDir, "etc", "llstack", "php", "runtimes")
	cfg.PHP.ManagedProfilesDir = filepath.Join(rootDir, "etc", "llstack", "php", "profiles")
	cfg.DB.ManagedProvidersDir = filepath.Join(rootDir, "etc", "llstack", "db", "providers")
	cfg.DB.ManagedConnectionsDir = filepath.Join(rootDir, "etc", "llstack", "db", "connections")
	cfg.DB.CertificatesDir = filepath.Join(rootDir, "etc", "llstack", "db", "certs")
	cfg.Cache.ManagedProvidersDir = filepath.Join(rootDir, "etc", "llstack", "cache", "providers")
	if err := os.MkdirAll(cfg.Paths.ConfigDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.Chmod(cfg.Paths.ConfigDir, 0o500); err != nil {
		t.Fatalf("chmod config dir: %v", err)
	}

	root := cli.NewRoot(cli.Dependencies{
		Version: "test-version",
		Config:  cfg,
		Logger:  logging.NewDefault(&stderr),
		Exec:    system.NewLocalExecutor(),
		Stdin:   bytes.NewBuffer(nil),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})

	cmd := root.Command(context.Background())
	cmd.SetArgs([]string{"repair", "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute repair text: %v", err)
	}
	if want := "warning: repair will address managed_directories:"; !bytes.Contains(stdout.Bytes(), []byte(want)) {
		t.Fatalf("expected warning in repair text, got %s", stdout.String())
	}
	if want := "reason=managed path is missing owner access bits"; !bytes.Contains(stdout.Bytes(), []byte(want)) {
		t.Fatalf("expected operation details in repair text, got %s", stdout.String())
	}
}

func TestDoctorBundleJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cfg := config.DefaultRuntimeConfig()
	rootDir := t.TempDir()
	cfg.Paths.ConfigDir = filepath.Join(rootDir, "etc", "llstack")
	cfg.Paths.StateDir = filepath.Join(rootDir, "var", "lib", "llstack", "state")
	cfg.Paths.HistoryDir = filepath.Join(rootDir, "var", "lib", "llstack", "history")
	cfg.Paths.BackupsDir = filepath.Join(rootDir, "var", "lib", "llstack", "backups")
	cfg.Paths.LogDir = filepath.Join(rootDir, "var", "log", "llstack")
	cfg.Paths.SitesRootDir = filepath.Join(rootDir, "data", "www")
	cfg.Apache.ManagedVhostsDir = filepath.Join(rootDir, "etc", "httpd", "conf.d", "llstack", "sites")
	cfg.OLS.ManagedVhostsRoot = filepath.Join(rootDir, "usr", "local", "lsws", "conf", "vhosts")
	cfg.OLS.ManagedListenersDir = filepath.Join(rootDir, "usr", "local", "lsws", "conf", "llstack", "listeners")
	cfg.LSWS.ManagedIncludesDir = filepath.Join(rootDir, "usr", "local", "lsws", "conf", "llstack", "includes")
	cfg.PHP.ManagedRuntimesDir = filepath.Join(rootDir, "etc", "llstack", "php", "runtimes")
	cfg.PHP.ManagedProfilesDir = filepath.Join(rootDir, "etc", "llstack", "php", "profiles")
	cfg.DB.ManagedProvidersDir = filepath.Join(rootDir, "etc", "llstack", "db", "providers")
	cfg.DB.ManagedConnectionsDir = filepath.Join(rootDir, "etc", "llstack", "db", "connections")
	cfg.DB.CertificatesDir = filepath.Join(rootDir, "etc", "llstack", "db", "certs")
	cfg.Cache.ManagedProvidersDir = filepath.Join(rootDir, "etc", "llstack", "cache", "providers")

	root := cli.NewRoot(cli.Dependencies{
		Version: "test-version",
		Config:  cfg,
		Logger:  logging.NewDefault(&stderr),
		Exec:    system.NewLocalExecutor(),
		Stdin:   bytes.NewBuffer(nil),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})

	bundlePath := filepath.Join(rootDir, "bundle.tar.gz")
	cmd := root.Command(context.Background())
	cmd.SetArgs([]string{"doctor", "--bundle", "--bundle-path", bundlePath, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute doctor bundle: %v", err)
	}
	if want := `"path": "`; !bytes.Contains(stdout.Bytes(), []byte(want)) {
		t.Fatalf("expected output to contain %s, got %s", want, stdout.String())
	}
	if _, err := os.Stat(bundlePath); err != nil {
		t.Fatalf("expected bundle file to be created: %v", err)
	}
}

func TestRollbackListJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cfg := testCLIConfig(t.TempDir())
	manager := site.NewManager(cfg, logging.NewDefault(&stderr), system.NewLocalExecutor())
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "history-list.example.com",
			Domain: model.DomainBinding{
				ServerName: "history-list.example.com",
			},
		},
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	root := cli.NewRoot(cli.Dependencies{
		Version: "test-version",
		Config:  cfg,
		Logger:  logging.NewDefault(&stderr),
		Exec:    system.NewLocalExecutor(),
		Stdin:   bytes.NewBuffer(nil),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})

	cmd := root.Command(context.Background())
	cmd.SetArgs([]string{"rollback:list", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute rollback:list: %v", err)
	}
	if want := `"id": "site-create-history-list.example.com"`; !bytes.Contains(stdout.Bytes(), []byte(want)) {
		t.Fatalf("expected rollback:list output to contain %s, got %s", want, stdout.String())
	}
}

func TestRollbackShowJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cfg := testCLIConfig(t.TempDir())
	manager := site.NewManager(cfg, logging.NewDefault(&stderr), system.NewLocalExecutor())
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "history-show.example.com",
			Domain: model.DomainBinding{
				ServerName: "history-show.example.com",
			},
		},
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}
	entry, err := rollback.LoadLatestPending(cfg.Paths.HistoryDir)
	if err != nil {
		t.Fatalf("load latest pending rollback: %v", err)
	}

	root := cli.NewRoot(cli.Dependencies{
		Version: "test-version",
		Config:  cfg,
		Logger:  logging.NewDefault(&stderr),
		Exec:    system.NewLocalExecutor(),
		Stdin:   bytes.NewBuffer(nil),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})

	cmd := root.Command(context.Background())
	cmd.SetArgs([]string{"rollback:show", entry.ID, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute rollback:show: %v", err)
	}
	if want := `"resource": "history-show.example.com"`; !bytes.Contains(stdout.Bytes(), []byte(want)) {
		t.Fatalf("expected rollback:show output to contain %s, got %s", want, stdout.String())
	}
	if want := `"rolled_back": false`; !bytes.Contains(stdout.Bytes(), []byte(want)) {
		t.Fatalf("expected rollback:show output to contain %s, got %s", want, stdout.String())
	}
}

func testCLIConfig(rootDir string) config.RuntimeConfig {
	cfg := config.DefaultRuntimeConfig()
	cfg.Paths.ConfigDir = filepath.Join(rootDir, "etc", "llstack")
	cfg.Paths.StateDir = filepath.Join(rootDir, "var", "lib", "llstack", "state")
	cfg.Paths.HistoryDir = filepath.Join(rootDir, "var", "lib", "llstack", "history")
	cfg.Paths.BackupsDir = filepath.Join(rootDir, "var", "lib", "llstack", "backups")
	cfg.Paths.LogDir = filepath.Join(rootDir, "var", "log", "llstack")
	cfg.Paths.SitesRootDir = filepath.Join(rootDir, "data", "www")
	cfg.Apache.ManagedVhostsDir = filepath.Join(rootDir, "etc", "httpd", "conf.d", "llstack", "sites")
	cfg.OLS.ManagedVhostsRoot = filepath.Join(rootDir, "usr", "local", "lsws", "conf", "vhosts")
	cfg.OLS.ManagedListenersDir = filepath.Join(rootDir, "usr", "local", "lsws", "conf", "llstack", "listeners")
	cfg.LSWS.ManagedIncludesDir = filepath.Join(rootDir, "usr", "local", "lsws", "conf", "llstack", "includes")
	cfg.PHP.ManagedRuntimesDir = filepath.Join(rootDir, "etc", "llstack", "php", "runtimes")
	cfg.PHP.ManagedProfilesDir = filepath.Join(rootDir, "etc", "llstack", "php", "profiles")
	cfg.DB.ManagedProvidersDir = filepath.Join(rootDir, "etc", "llstack", "db", "providers")
	cfg.DB.ManagedConnectionsDir = filepath.Join(rootDir, "etc", "llstack", "db", "connections")
	cfg.DB.CertificatesDir = filepath.Join(rootDir, "etc", "llstack", "db", "certs")
	cfg.Cache.ManagedProvidersDir = filepath.Join(rootDir, "etc", "llstack", "cache", "providers")
	return cfg
}
