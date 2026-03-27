package db_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/web-casa/llstack/internal/cli"
	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/db"
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

func TestDatabaseLifecycleUpdatesManifest(t *testing.T) {
	cfg := testConfig(t.TempDir())
	exec := &fakeExecutor{}
	manager := db.NewManager(cfg, logging.NewDefault(io.Discard), exec)

	if _, err := manager.Install(context.Background(), db.InstallOptions{
		Provider: db.ProviderMariaDB,
		TLSMode:  db.TLSEnabled,
	}); err != nil {
		t.Fatalf("install provider: %v", err)
	}
	if _, err := manager.Init(context.Background(), db.InitOptions{
		Provider:      db.ProviderMariaDB,
		AdminUser:     "llstack_admin",
		AdminPassword: "secret",
		TLSMode:       db.TLSEnabled,
	}); err != nil {
		t.Fatalf("init provider: %v", err)
	}
	if _, err := manager.CreateDatabase(context.Background(), db.CreateDatabaseOptions{
		Provider: db.ProviderMariaDB,
		Name:     "appdb",
	}); err != nil {
		t.Fatalf("create database: %v", err)
	}
	if _, err := manager.CreateUser(context.Background(), db.CreateUserOptions{
		Provider: db.ProviderMariaDB,
		Name:     "appuser",
		Password: "secret",
		Database: "appdb",
		TLSMode:  db.TLSRequired,
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(cfg.DB.ManagedProvidersDir, "mariadb.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if !bytes.Contains(raw, []byte(`"name": "appdb"`)) || !bytes.Contains(raw, []byte(`"name": "appuser"`)) {
		t.Fatalf("manifest missing database or user records: %s", string(raw))
	}
	if _, err := os.Stat(filepath.Join(cfg.DB.ManagedConnectionsDir, "mariadb-appuser_appdb.json")); err != nil {
		t.Fatalf("connection info missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.Paths.ConfigDir, "db", "credentials", "mariadb-mariadb-admin.secret")); err != nil {
		t.Fatalf("admin credential missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.Paths.ConfigDir, "db", "credentials", "mariadb-appuser_appdb.secret")); err != nil {
		t.Fatalf("user credential missing: %v", err)
	}
}

func TestDBInstallCLIJSON(t *testing.T) {
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
	cmd.SetArgs([]string{"db:install", "mariadb", "--tls", "enabled", "--dry-run", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute db:install: %v", err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"kind": "db.install"`)) {
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
	cfg.DB.ManagedProvidersDir = filepath.Join(root, "etc", "llstack", "db", "providers")
	cfg.DB.ManagedConnectionsDir = filepath.Join(root, "etc", "llstack", "db", "connections")
	cfg.DB.CertificatesDir = filepath.Join(root, "etc", "llstack", "db", "certs")
	cfg.DB.MariaDBTLSConfigPath = filepath.Join(root, "etc", "my.cnf.d", "llstack-mariadb-tls.cnf")
	cfg.DB.MySQLTLSConfigPath = filepath.Join(root, "etc", "my.cnf.d", "llstack-mysql-tls.cnf")
	cfg.DB.PerconaTLSConfigPath = filepath.Join(root, "etc", "my.cnf.d", "llstack-percona-tls.cnf")
	cfg.DB.PostgreSQLTLSConfigPath = filepath.Join(root, "var", "lib", "pgsql", "16", "data", "conf.d", "llstack-tls.conf")
	return cfg
}
