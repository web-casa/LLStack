package db_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestResolveProviderAndTLSProfile(t *testing.T) {
	cfg := testConfig(t.TempDir())
	spec, err := db.ResolveProvider(cfg, db.ProviderPostgres, "")
	if err != nil {
		t.Fatalf("resolve provider: %v", err)
	}
	if spec.ServiceName != "postgresql-16" {
		t.Fatalf("unexpected postgres service name: %s", spec.ServiceName)
	}

	profile, warnings := db.BuildTLSProfile(cfg, spec, db.TLSRequired)
	if !profile.Enabled || profile.ServerConfigPath == "" {
		t.Fatalf("unexpected tls profile: %#v", profile)
	}
	if len(warnings) == 0 {
		t.Fatal("expected postgres TLS warning for required mode")
	}
}

func TestInstallWritesManifestAndTLSConfig(t *testing.T) {
	cfg := testConfig(t.TempDir())
	exec := &fakeExecutor{}
	manager := db.NewManager(cfg, logging.NewDefault(io.Discard), exec)

	if _, err := manager.Install(context.Background(), db.InstallOptions{
		Provider: db.ProviderMariaDB,
		TLSMode:  db.TLSEnabled,
	}); err != nil {
		t.Fatalf("install mariadb: %v", err)
	}

	if len(exec.commands) != 2 {
		t.Fatalf("expected dnf and systemctl commands, got %d", len(exec.commands))
	}
	manifestPath := filepath.Join(cfg.DB.ManagedProvidersDir, "mariadb.json")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
	if _, err := os.Stat(cfg.DB.MariaDBTLSConfigPath); err != nil {
		t.Fatalf("tls config missing: %v", err)
	}
}

func TestCreateUserRequiresPassword(t *testing.T) {
	cfg := testConfig(t.TempDir())
	exec := &fakeExecutor{}
	manager := db.NewManager(cfg, logging.NewDefault(io.Discard), exec)
	if _, err := manager.Install(context.Background(), db.InstallOptions{
		Provider: db.ProviderMariaDB,
	}); err != nil {
		t.Fatalf("install mariadb: %v", err)
	}

	_, err := manager.CreateUser(context.Background(), db.CreateUserOptions{
		Provider: db.ProviderMariaDB,
		Name:     "appuser",
	})
	if err == nil || !strings.Contains(err.Error(), "password is required") {
		t.Fatalf("expected password error, got %v", err)
	}
}

func testConfig(root string) config.RuntimeConfig {
	cfg := config.DefaultRuntimeConfig()
	cfg.Paths.BackupsDir = filepath.Join(root, "var", "lib", "llstack", "backups")
	cfg.DB.ManagedProvidersDir = filepath.Join(root, "etc", "llstack", "db", "providers")
	cfg.DB.ManagedConnectionsDir = filepath.Join(root, "etc", "llstack", "db", "connections")
	cfg.DB.CertificatesDir = filepath.Join(root, "etc", "llstack", "db", "certs")
	cfg.DB.MariaDBTLSConfigPath = filepath.Join(root, "etc", "my.cnf.d", "llstack-mariadb-tls.cnf")
	cfg.DB.MySQLTLSConfigPath = filepath.Join(root, "etc", "my.cnf.d", "llstack-mysql-tls.cnf")
	cfg.DB.PerconaTLSConfigPath = filepath.Join(root, "etc", "my.cnf.d", "llstack-percona-tls.cnf")
	cfg.DB.PostgreSQLTLSConfigPath = filepath.Join(root, "var", "lib", "pgsql", "16", "data", "conf.d", "llstack-tls.conf")
	return cfg
}
