package tui_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/logging"
	phpruntime "github.com/web-casa/llstack/internal/php"
	"github.com/web-casa/llstack/internal/site"
	"github.com/web-casa/llstack/internal/system"
	"github.com/web-casa/llstack/internal/tui"
)

type fakeExecutor struct{}

func (fakeExecutor) Run(ctx context.Context, cmd system.Command) (system.Result, error) {
	return system.Result{ExitCode: 0}, nil
}

func TestModelNavigationMovesToNextScreen(t *testing.T) {
	model := tui.NewModel("test", config.DefaultRuntimeConfig(), logging.NewDefault(os.Stderr), system.NewLocalExecutor())
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	next := updated.(tui.Model)

	if next.ActiveScreen() != tui.ScreenInstall {
		t.Fatalf("expected active screen %q, got %q", tui.ScreenInstall, next.ActiveScreen())
	}
}

func TestSitesScreenMovesSelection(t *testing.T) {
	cfg := config.DefaultRuntimeConfig()
	root := t.TempDir()
	cfg.Paths.ConfigDir = filepath.Join(root, "etc", "llstack")
	if err := os.MkdirAll(cfg.Paths.ManagedSitesDir(), 0o755); err != nil {
		t.Fatalf("mkdir managed sites: %v", err)
	}
	writeManifest(t, cfg.Paths.ManagedSitesDir(), "a.example.com")
	writeManifest(t, cfg.Paths.ManagedSitesDir(), "b.example.com")

	model := tui.NewModel("test", cfg, logging.NewDefault(os.Stderr), system.NewLocalExecutor())
	for range 2 {
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model = updated.(tui.Model)
	}
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	next := updated.(tui.Model)

	if next.ActiveScreen() != tui.ScreenSites {
		t.Fatalf("expected sites screen, got %q", next.ActiveScreen())
	}
	if next.SelectedSiteIndex() != 1 {
		t.Fatalf("expected selected site index 1, got %d", next.SelectedSiteIndex())
	}
}

func TestInstallScreenShowsPlanPreview(t *testing.T) {
	model := tui.NewModel("test", config.DefaultRuntimeConfig(), logging.NewDefault(os.Stderr), system.NewLocalExecutor())

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(tui.Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model = updated.(tui.Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	model = updated.(tui.Model)

	view := model.View()
	if !strings.Contains(view, "Plan Preview") {
		t.Fatalf("expected install view to contain plan preview, got %s", view)
	}
}

func TestDatabaseScreenShowsPlanPreviewAndAcceptsTextInput(t *testing.T) {
	cfg := config.DefaultRuntimeConfig()
	root := t.TempDir()
	cfg.Paths.ConfigDir = filepath.Join(root, "etc", "llstack")
	cfg.DB.ManagedProvidersDir = filepath.Join(cfg.Paths.ConfigDir, "db", "providers")
	cfg.DB.ManagedConnectionsDir = filepath.Join(cfg.Paths.ConfigDir, "db", "connections")

	model := tui.NewModel("test", cfg, logging.NewDefault(os.Stderr), system.NewLocalExecutor())

	for range 3 {
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model = updated.(tui.Model)
	}
	if model.ActiveScreen() != tui.ScreenDatabase {
		t.Fatalf("expected database screen, got %q", model.ActiveScreen())
	}

	for range 4 {
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		model = updated.(tui.Model)
	}
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	model = updated.(tui.Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("appdb")})
	model = updated.(tui.Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(tui.Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	model = updated.(tui.Model)

	view := model.View()
	if !strings.Contains(view, "Database Setup") {
		t.Fatalf("expected database screen heading, got %s", view)
	}
	if !strings.Contains(view, "Create Database") {
		t.Fatalf("expected database preview to include create database block, got %s", view)
	}
	if !strings.Contains(view, "appdb") {
		t.Fatalf("expected database preview to include typed database name, got %s", view)
	}
}

func TestDatabaseScreenPreviewShowsOperationDetailsButRedactsSQL(t *testing.T) {
	cfg := fullTestConfig(t.TempDir())
	exec := fakeExecutor{}
	tuiModel := tui.NewModel("test", cfg, logging.NewDefault(os.Stderr), exec)

	for range 3 {
		updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyTab})
		tuiModel = updated.(tui.Model)
	}
	if tuiModel.ActiveScreen() != tui.ScreenDatabase {
		t.Fatalf("expected database screen, got %q", tuiModel.ActiveScreen())
	}

	for range 3 {
		updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		tuiModel = updated.(tui.Model)
	}
	updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("supersecret")})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	tuiModel = updated.(tui.Model)

	view := tuiModel.View()
	if !strings.Contains(view, "provider=mariadb") {
		t.Fatalf("expected non-sensitive operation details in database preview, got %s", view)
	}
	if !strings.Contains(view, "sql=[redacted]") {
		t.Fatalf("expected SQL details to be redacted in database preview, got %s", view)
	}
	if strings.Contains(view, "supersecret") {
		t.Fatalf("expected database preview to redact secrets, got %s", view)
	}
}

func TestInstallScreenCanApplyPlan(t *testing.T) {
	cfg := fullTestConfig(t.TempDir())
	exec := fakeExecutor{}
	tuiModel := tui.NewModel("test", cfg, logging.NewDefault(os.Stderr), exec)

	updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyTab})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	tuiModel = updated.(tui.Model)
	if !strings.Contains(tuiModel.View(), "pending: install current plan") {
		t.Fatalf("expected install confirmation prompt, got %s", tuiModel.View())
	}

	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tuiModel = updated.(tui.Model)
	view := tuiModel.View()
	if !strings.Contains(view, "last action: install plan applied") {
		t.Fatalf("expected install feedback, got %s", view)
	}
}

func TestDoctorScreenShowsRepairPreview(t *testing.T) {
	cfg := fullTestConfig(t.TempDir())
	exec := fakeExecutor{}
	if err := os.MkdirAll(cfg.Paths.ConfigDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.Chmod(cfg.Paths.ConfigDir, 0o500); err != nil {
		t.Fatalf("chmod config dir: %v", err)
	}
	defer os.Chmod(cfg.Paths.ConfigDir, 0o755)

	tuiModel := tui.NewModel("test", cfg, logging.NewDefault(os.Stderr), exec)
	for range 7 {
		updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyTab})
		tuiModel = updated.(tui.Model)
	}
	if tuiModel.ActiveScreen() != tui.ScreenDoctor {
		t.Fatalf("expected doctor screen, got %q", tuiModel.ActiveScreen())
	}

	updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	tuiModel = updated.(tui.Model)
	view := tuiModel.View()
	if !strings.Contains(view, "Repair Plan Preview") {
		t.Fatalf("expected doctor repair preview, got %s", view)
	}
	if !strings.Contains(view, "reason=managed directory is missing") {
		t.Fatalf("expected repair operation details in doctor preview, got %s", view)
	}
}

func TestDoctorScreenCanApplyRepair(t *testing.T) {
	cfg := fullTestConfig(t.TempDir())
	exec := fakeExecutor{}
	if err := os.MkdirAll(cfg.Paths.ConfigDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.Chmod(cfg.Paths.ConfigDir, 0o500); err != nil {
		t.Fatalf("chmod config dir: %v", err)
	}

	tuiModel := tui.NewModel("test", cfg, logging.NewDefault(os.Stderr), exec)
	for range 7 {
		updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyTab})
		tuiModel = updated.(tui.Model)
	}
	if tuiModel.ActiveScreen() != tui.ScreenDoctor {
		t.Fatalf("expected doctor screen, got %q", tuiModel.ActiveScreen())
	}

	updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	tuiModel = updated.(tui.Model)
	if !strings.Contains(tuiModel.View(), "pending: apply current repair plan") {
		t.Fatalf("expected doctor repair confirmation prompt, got %s", tuiModel.View())
	}

	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tuiModel = updated.(tui.Model)
	view := tuiModel.View()
	if !strings.Contains(view, "last action: repair plan applied") {
		t.Fatalf("expected doctor repair feedback, got %s", view)
	}
	info, err := os.Stat(cfg.Paths.ConfigDir)
	if err != nil {
		t.Fatalf("stat config dir: %v", err)
	}
	if info.Mode().Perm()&0o700 != 0o700 {
		t.Fatalf("expected repaired mode to include owner rwx, got %#o", info.Mode().Perm())
	}
}

func TestDoctorScreenCanFilterChecks(t *testing.T) {
	cfg := fullTestConfig(t.TempDir())
	exec := fakeExecutor{}
	if err := os.MkdirAll(cfg.Paths.ConfigDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.Chmod(cfg.Paths.ConfigDir, 0o500); err != nil {
		t.Fatalf("chmod config dir: %v", err)
	}

	tuiModel := tui.NewModel("test", cfg, logging.NewDefault(os.Stderr), exec)
	for range 7 {
		updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyTab})
		tuiModel = updated.(tui.Model)
	}
	if tuiModel.ActiveScreen() != tui.ScreenDoctor {
		t.Fatalf("expected doctor screen, got %q", tuiModel.ActiveScreen())
	}

	updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	tuiModel = updated.(tui.Model)
	view := tuiModel.View()
	if !strings.Contains(view, "filter=warn") {
		t.Fatalf("expected warn filter, got %s", view)
	}
	if strings.Contains(view, "db_tls_state") {
		t.Fatalf("expected pass-only checks to be hidden in warn filter, got %s", view)
	}

	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	tuiModel = updated.(tui.Model)
	view = tuiModel.View()
	if !strings.Contains(view, "filter=pass") {
		t.Fatalf("expected pass filter, got %s", view)
	}
	if !strings.Contains(view, "db_tls_state") {
		t.Fatalf("expected db_tls_state check in pass filter, got %s", view)
	}
}

func TestDoctorScreenShowsRepairCoverageGroups(t *testing.T) {
	cfg := fullTestConfig(t.TempDir())
	exec := fakeExecutor{}
	if err := os.MkdirAll(cfg.DB.ManagedProvidersDir, 0o755); err != nil {
		t.Fatalf("mkdir db providers: %v", err)
	}
	dbManifest := `{"provider":"mariadb","service_name":"mariadb","status":"initialized","admin_connection":{"host":"127.0.0.1","port":3306}}`
	if err := os.WriteFile(filepath.Join(cfg.DB.ManagedProvidersDir, "mariadb.json"), []byte(dbManifest), 0o644); err != nil {
		t.Fatalf("write db manifest: %v", err)
	}

	tuiModel := tui.NewModel("test", cfg, logging.NewDefault(os.Stderr), exec)
	for range 7 {
		updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyTab})
		tuiModel = updated.(tui.Model)
	}
	view := tuiModel.View()
	if !strings.Contains(view, "repair_coverage=auto:") {
		t.Fatalf("expected repair coverage summary, got %s", view)
	}
	if !strings.Contains(view, "auto_repair:") {
		t.Fatalf("expected auto repair section, got %s", view)
	}
	if !strings.Contains(view, "manual_only:") {
		t.Fatalf("expected manual-only section, got %s", view)
	}
	if !strings.Contains(view, "managed_directories") {
		t.Fatalf("expected repairable check group to mention managed_directories, got %s", view)
	}
	if !strings.Contains(view, "managed_db_live_probe") {
		t.Fatalf("expected manual-only group to mention managed_db_live_probe, got %s", view)
	}
}

func TestHistoryScreenShowsRollbackPreview(t *testing.T) {
	cfg := fullTestConfig(t.TempDir())
	exec := fakeExecutor{}
	manager := site.NewManager(cfg, logging.NewDefault(os.Stderr), exec)
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "history-preview.example.com",
			Domain: model.DomainBinding{
				ServerName: "history-preview.example.com",
			},
			PHP: model.PHPRuntimeBinding{
				Enabled: true,
			},
		},
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	tuiModel := tui.NewModel("test", cfg, logging.NewDefault(os.Stderr), exec)
	for range 8 {
		updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyTab})
		tuiModel = updated.(tui.Model)
	}
	if tuiModel.ActiveScreen() != tui.ScreenHistory {
		t.Fatalf("expected history screen, got %q", tuiModel.ActiveScreen())
	}

	updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	tuiModel = updated.(tui.Model)
	view := tuiModel.View()
	if !strings.Contains(view, "Rollback Plan Preview") {
		t.Fatalf("expected rollback preview, got %s", view)
	}
	if !strings.Contains(view, "Rollback last operation site.create on history-preview.example.com") {
		t.Fatalf("expected rollback summary in history preview, got %s", view)
	}
	if !strings.Contains(view, "rollback.file_created") {
		t.Fatalf("expected rollback operations in history preview, got %s", view)
	}
}

func TestHistoryScreenCanApplyRollback(t *testing.T) {
	cfg := fullTestConfig(t.TempDir())
	exec := fakeExecutor{}
	manager := site.NewManager(cfg, logging.NewDefault(os.Stderr), exec)
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "history-apply.example.com",
			Domain: model.DomainBinding{
				ServerName: "history-apply.example.com",
			},
			PHP: model.PHPRuntimeBinding{
				Enabled: true,
			},
		},
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	tuiModel := tui.NewModel("test", cfg, logging.NewDefault(os.Stderr), exec)
	for range 8 {
		updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyTab})
		tuiModel = updated.(tui.Model)
	}
	if tuiModel.ActiveScreen() != tui.ScreenHistory {
		t.Fatalf("expected history screen, got %q", tuiModel.ActiveScreen())
	}

	updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	tuiModel = updated.(tui.Model)
	if !strings.Contains(tuiModel.View(), "pending: apply latest rollback plan") {
		t.Fatalf("expected rollback confirmation prompt, got %s", tuiModel.View())
	}

	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tuiModel = updated.(tui.Model)
	view := tuiModel.View()
	if !strings.Contains(view, "last action: rollback applied") {
		t.Fatalf("expected rollback feedback, got %s", view)
	}
	if !strings.Contains(view, "rolled_back=true") {
		t.Fatalf("expected history entry to be marked rolled back, got %s", view)
	}
	if _, err := manager.Show("history-apply.example.com"); err == nil {
		t.Fatalf("expected site manifest to be removed after rollback")
	}
}

func TestHistoryScreenCanFilterEntries(t *testing.T) {
	cfg := fullTestConfig(t.TempDir())
	exec := fakeExecutor{}
	manager := site.NewManager(cfg, logging.NewDefault(os.Stderr), exec)
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "history-filter-a.example.com",
			Domain: model.DomainBinding{
				ServerName: "history-filter-a.example.com",
			},
		},
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create site a: %v", err)
	}
	if _, err := manager.RollbackLast(context.Background(), site.RollbackOptions{SkipReload: true}); err != nil {
		t.Fatalf("rollback site a: %v", err)
	}
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "history-filter-b.example.com",
			Domain: model.DomainBinding{
				ServerName: "history-filter-b.example.com",
			},
		},
		SkipReload: true,
	}); err != nil {
		t.Fatalf("create site b: %v", err)
	}

	tuiModel := tui.NewModel("test", cfg, logging.NewDefault(os.Stderr), exec)
	for range 8 {
		updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyTab})
		tuiModel = updated.(tui.Model)
	}

	updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	tuiModel = updated.(tui.Model)
	view := tuiModel.View()
	if !strings.Contains(view, "filter=pending") || strings.Contains(view, "history-filter-a.example.com  rolled-back") {
		t.Fatalf("expected pending-only filter, got %s", view)
	}

	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	tuiModel = updated.(tui.Model)
	view = tuiModel.View()
	if !strings.Contains(view, "filter=rolled-back") || !strings.Contains(view, "history-filter-a.example.com") {
		t.Fatalf("expected rolled-back filter view, got %s", view)
	}
}

func TestSitesScreenCanConfirmStopAction(t *testing.T) {
	cfg := config.DefaultRuntimeConfig()
	root := t.TempDir()
	cfg.Paths.ConfigDir = filepath.Join(root, "etc", "llstack")
	cfg.Paths.StateDir = filepath.Join(root, "var", "lib", "llstack", "state")
	cfg.Paths.HistoryDir = filepath.Join(root, "var", "lib", "llstack", "history")
	cfg.Paths.BackupsDir = filepath.Join(root, "var", "lib", "llstack", "backups")
	cfg.Paths.LogDir = filepath.Join(root, "var", "log", "llstack")
	cfg.Paths.SitesRootDir = filepath.Join(root, "data", "www")
	cfg.Apache.ManagedVhostsDir = filepath.Join(root, "etc", "httpd", "conf.d", "llstack", "sites")

	exec := fakeExecutor{}
	manager := site.NewManager(cfg, logging.NewDefault(os.Stderr), exec)
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "action.example.com",
			Domain: model.DomainBinding{
				ServerName: "action.example.com",
			},
			PHP: model.PHPRuntimeBinding{
				Enabled: true,
			},
		},
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	tuiModel := tui.NewModel("test", cfg, logging.NewDefault(os.Stderr), exec)
	for range 2 {
		updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyTab})
		tuiModel = updated.(tui.Model)
	}
	if tuiModel.ActiveScreen() != tui.ScreenSites {
		t.Fatalf("expected sites screen, got %q", tuiModel.ActiveScreen())
	}

	updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	tuiModel = updated.(tui.Model)
	if !strings.Contains(tuiModel.View(), "pending: stop action.example.com") {
		t.Fatalf("expected stop confirmation prompt, got %s", tuiModel.View())
	}

	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tuiModel = updated.(tui.Model)
	view := tuiModel.View()
	if !strings.Contains(view, "state=disabled") {
		t.Fatalf("expected stopped site to render disabled state, got %s", view)
	}
	if !strings.Contains(view, "last action: stop action.example.com completed") {
		t.Fatalf("expected stop feedback, got %s", view)
	}
}

func TestSitesScreenCanConfirmRestartAction(t *testing.T) {
	cfg := fullTestConfig(t.TempDir())
	exec := fakeExecutor{}
	manager := site.NewManager(cfg, logging.NewDefault(os.Stderr), exec)
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "restart-ui.example.com",
			Domain: model.DomainBinding{
				ServerName: "restart-ui.example.com",
			},
			PHP: model.PHPRuntimeBinding{
				Enabled: true,
			},
		},
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	tuiModel := tui.NewModel("test", cfg, logging.NewDefault(os.Stderr), exec)
	for range 2 {
		updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyTab})
		tuiModel = updated.(tui.Model)
	}

	updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	tuiModel = updated.(tui.Model)
	if !strings.Contains(tuiModel.View(), "pending: restart restart-ui.example.com") {
		t.Fatalf("expected restart confirmation prompt, got %s", tuiModel.View())
	}

	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tuiModel = updated.(tui.Model)
	view := tuiModel.View()
	if !strings.Contains(view, "last action: restart restart-ui.example.com completed") {
		t.Fatalf("expected restart feedback, got %s", view)
	}
}

func TestSitesScreenCanShowLogsAndTLSPreview(t *testing.T) {
	cfg := config.DefaultRuntimeConfig()
	root := t.TempDir()
	cfg.Paths.ConfigDir = filepath.Join(root, "etc", "llstack")
	cfg.Paths.StateDir = filepath.Join(root, "var", "lib", "llstack", "state")
	cfg.Paths.HistoryDir = filepath.Join(root, "var", "lib", "llstack", "history")
	cfg.Paths.BackupsDir = filepath.Join(root, "var", "lib", "llstack", "backups")
	cfg.Paths.LogDir = filepath.Join(root, "var", "log", "llstack")
	cfg.Paths.SitesRootDir = filepath.Join(root, "data", "www")
	cfg.Apache.ManagedVhostsDir = filepath.Join(root, "etc", "httpd", "conf.d", "llstack", "sites")

	exec := fakeExecutor{}
	manager := site.NewManager(cfg, logging.NewDefault(os.Stderr), exec)
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "inspect.example.com",
			Domain: model.DomainBinding{
				ServerName: "inspect.example.com",
			},
			PHP: model.PHPRuntimeBinding{
				Enabled: true,
			},
		},
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}
	manifest, err := manager.Show("inspect.example.com")
	if err != nil {
		t.Fatalf("show site: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(manifest.Site.Logs.AccessLog), 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	if err := os.WriteFile(manifest.Site.Logs.AccessLog, []byte("line-1\nline-2\n"), 0o644); err != nil {
		t.Fatalf("write access log: %v", err)
	}

	tuiModel := tui.NewModel("test", cfg, logging.NewDefault(os.Stderr), exec)
	for range 2 {
		updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyTab})
		tuiModel = updated.(tui.Model)
	}

	updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	tuiModel = updated.(tui.Model)

	view := tuiModel.View()
	if !strings.Contains(view, "Logs") || !strings.Contains(view, "line-1") {
		t.Fatalf("expected logs panel in sites view, got %s", view)
	}
	if !strings.Contains(view, "access log  lines=10") {
		t.Fatalf("expected default log line count in sites view, got %s", view)
	}
	if !strings.Contains(view, "TLS Plan Preview") || !strings.Contains(view, "Configure TLS for inspect.example.com") {
		t.Fatalf("expected TLS plan preview in sites view, got %s", view)
	}
}

func TestSitesScreenCanAdjustAndRefreshLogsPanel(t *testing.T) {
	cfg := fullTestConfig(t.TempDir())
	exec := fakeExecutor{}
	manager := site.NewManager(cfg, logging.NewDefault(os.Stderr), exec)
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "logpanel.example.com",
			Domain: model.DomainBinding{
				ServerName: "logpanel.example.com",
			},
			PHP: model.PHPRuntimeBinding{
				Enabled: true,
			},
		},
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}
	manifest, err := manager.Show("logpanel.example.com")
	if err != nil {
		t.Fatalf("show site: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(manifest.Site.Logs.AccessLog), 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	if err := os.WriteFile(manifest.Site.Logs.AccessLog, []byte(strings.Join([]string{
		"line-01", "line-02", "line-03", "line-04", "line-05", "line-06",
		"line-07", "line-08", "line-09", "line-10", "line-11", "line-12",
	}, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write access log: %v", err)
	}

	tuiModel := tui.NewModel("test", cfg, logging.NewDefault(os.Stderr), exec)
	for range 2 {
		updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyTab})
		tuiModel = updated.(tui.Model)
	}

	updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	tuiModel = updated.(tui.Model)
	view := tuiModel.View()
	if !strings.Contains(view, "access log  lines=10") || !strings.Contains(view, "line-03") {
		t.Fatalf("expected default access log tail, got %s", view)
	}

	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	tuiModel = updated.(tui.Model)
	view = tuiModel.View()
	if !strings.Contains(view, "access log  lines=15") {
		t.Fatalf("expected increased log line count, got %s", view)
	}

	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	tuiModel = updated.(tui.Model)
	view = tuiModel.View()
	if !strings.Contains(view, "last action: refreshed access logs (15 lines)") {
		t.Fatalf("expected refresh feedback, got %s", view)
	}
}

func TestSitesScreenCanApplyTLSFromPreview(t *testing.T) {
	cfg := fullTestConfig(t.TempDir())
	mockCertbot := filepath.Join(t.TempDir(), "certbot")
	if err := os.WriteFile(mockCertbot, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write mock certbot: %v", err)
	}
	cfg.SSL.CertbotCandidates = []string{mockCertbot}

	exec := fakeExecutor{}
	manager := site.NewManager(cfg, logging.NewDefault(os.Stderr), exec)
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "tlsapply.example.com",
			Domain: model.DomainBinding{
				ServerName: "tlsapply.example.com",
			},
			PHP: model.PHPRuntimeBinding{
				Enabled: true,
			},
		},
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	tuiModel := tui.NewModel("test", cfg, logging.NewDefault(os.Stderr), exec)
	for range 2 {
		updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyTab})
		tuiModel = updated.(tui.Model)
	}

	updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	tuiModel = updated.(tui.Model)
	if !strings.Contains(tuiModel.View(), "pending: tls tlsapply.example.com") {
		t.Fatalf("expected TLS confirmation prompt, got %s", tuiModel.View())
	}

	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tuiModel = updated.(tui.Model)
	view := tuiModel.View()
	if !strings.Contains(view, "last action: tls tlsapply.example.com completed") {
		t.Fatalf("expected TLS apply feedback, got %s", view)
	}
	if !strings.Contains(view, "tls=true") {
		t.Fatalf("expected TLS enabled state in sites view, got %s", view)
	}

	manifest, err := manager.Show("tlsapply.example.com")
	if err != nil {
		t.Fatalf("show site after tls apply: %v", err)
	}
	if !manifest.Site.TLS.Enabled || manifest.Site.TLS.Mode != "letsencrypt" {
		t.Fatalf("expected letsencrypt tls state, got %#v", manifest.Site.TLS)
	}
}

func TestSitesScreenCanPreviewAndApplyPHPVersion(t *testing.T) {
	cfg := config.DefaultRuntimeConfig()
	root := t.TempDir()
	cfg.Paths.ConfigDir = filepath.Join(root, "etc", "llstack")
	cfg.Paths.StateDir = filepath.Join(root, "var", "lib", "llstack", "state")
	cfg.Paths.HistoryDir = filepath.Join(root, "var", "lib", "llstack", "history")
	cfg.Paths.BackupsDir = filepath.Join(root, "var", "lib", "llstack", "backups")
	cfg.Paths.LogDir = filepath.Join(root, "var", "log", "llstack")
	cfg.Paths.SitesRootDir = filepath.Join(root, "data", "www")
	cfg.Apache.ManagedVhostsDir = filepath.Join(root, "etc", "httpd", "conf.d", "llstack", "sites")
	cfg.PHP.ManagedRuntimesDir = filepath.Join(root, "etc", "llstack", "php", "runtimes")
	cfg.PHP.ManagedProfilesDir = filepath.Join(root, "etc", "llstack", "php", "profiles")
	cfg.PHP.ProfileRoot = filepath.Join(root, "etc", "opt", "remi")
	cfg.PHP.RuntimeRoot = filepath.Join(root, "opt", "remi")
	cfg.PHP.StateRoot = filepath.Join(root, "var", "opt", "remi")
	cfg.PHP.ELMajorOverride = "9"

	exec := fakeExecutor{}
	phpManager := phpruntime.NewManager(cfg, logging.NewDefault(os.Stderr), exec)
	if _, err := phpManager.Install(context.Background(), phpruntime.InstallOptions{
		Version:      "8.4",
		Profile:      phpruntime.ProfileGeneric,
		IncludeFPM:   true,
		IncludeLSAPI: true,
	}); err != nil {
		t.Fatalf("install php 8.4: %v", err)
	}

	manager := site.NewManager(cfg, logging.NewDefault(os.Stderr), exec)
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: model.Site{
			Name: "phpswitch.example.com",
			Domain: model.DomainBinding{
				ServerName: "phpswitch.example.com",
			},
			PHP: model.PHPRuntimeBinding{
				Enabled: true,
				Version: "8.3",
			},
		},
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	tuiModel := tui.NewModel("test", cfg, logging.NewDefault(os.Stderr), exec)
	for range 2 {
		updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyTab})
		tuiModel = updated.(tui.Model)
	}

	updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	tuiModel = updated.(tui.Model)
	view := tuiModel.View()
	if !strings.Contains(view, "target=8.4") || !strings.Contains(view, "PHP Switch Preview") {
		t.Fatalf("expected php preview after cycling target, got %s", view)
	}

	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	tuiModel = updated.(tui.Model)
	if !strings.Contains(tuiModel.View(), "pending: php phpswitch.example.com") {
		t.Fatalf("expected php confirmation prompt, got %s", tuiModel.View())
	}

	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tuiModel = updated.(tui.Model)
	view = tuiModel.View()
	if !strings.Contains(view, "last action: php phpswitch.example.com completed") {
		t.Fatalf("expected php apply feedback, got %s", view)
	}
	if !strings.Contains(view, "php: enabled=true version=8.4") {
		t.Fatalf("expected updated php version in detail view, got %s", view)
	}

	manifest, err := manager.Show("phpswitch.example.com")
	if err != nil {
		t.Fatalf("show site after php switch: %v", err)
	}
	if manifest.Site.PHP.Version != "8.4" {
		t.Fatalf("expected php version 8.4, got %s", manifest.Site.PHP.Version)
	}
}

func TestSitesScreenCanPreviewAndCreateSite(t *testing.T) {
	cfg := config.DefaultRuntimeConfig()
	root := t.TempDir()
	cfg.Paths.ConfigDir = filepath.Join(root, "etc", "llstack")
	cfg.Paths.StateDir = filepath.Join(root, "var", "lib", "llstack", "state")
	cfg.Paths.HistoryDir = filepath.Join(root, "var", "lib", "llstack", "history")
	cfg.Paths.BackupsDir = filepath.Join(root, "var", "lib", "llstack", "backups")
	cfg.Paths.LogDir = filepath.Join(root, "var", "log", "llstack")
	cfg.Paths.SitesRootDir = filepath.Join(root, "data", "www")
	cfg.Apache.ManagedVhostsDir = filepath.Join(root, "etc", "httpd", "conf.d", "llstack", "sites")

	exec := fakeExecutor{}
	tuiModel := tui.NewModel("test", cfg, logging.NewDefault(os.Stderr), exec)
	for range 2 {
		updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyTab})
		tuiModel = updated.(tui.Model)
	}

	updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	tuiModel = updated.(tui.Model)
	view := tuiModel.View()
	if !strings.Contains(view, "Create Site Wizard") {
		t.Fatalf("expected create site wizard, got %s", view)
	}

	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("newsite.example.com")})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	tuiModel = updated.(tui.Model)

	view = tuiModel.View()
	if !strings.Contains(view, "Plan Preview") || !strings.Contains(view, "Create APACHE-managed site newsite.example.com") {
		t.Fatalf("expected site create preview, got %s", view)
	}

	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	tuiModel = updated.(tui.Model)
	if !strings.Contains(tuiModel.View(), "pending: create newsite.example.com") {
		t.Fatalf("expected create confirmation prompt, got %s", tuiModel.View())
	}

	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tuiModel = updated.(tui.Model)
	view = tuiModel.View()
	if !strings.Contains(view, "last action: create newsite.example.com completed") {
		t.Fatalf("expected create feedback, got %s", view)
	}
	if strings.Contains(view, "Create Site Wizard") {
		t.Fatalf("expected wizard to close after create, got %s", view)
	}

	manager := site.NewManager(cfg, logging.NewDefault(os.Stderr), exec)
	manifest, err := manager.Show("newsite.example.com")
	if err != nil {
		t.Fatalf("show created site: %v", err)
	}
	if manifest.Site.Name != "newsite.example.com" {
		t.Fatalf("unexpected site manifest: %#v", manifest.Site)
	}
}

func TestSitesScreenCanPreviewAndApplyEditSite(t *testing.T) {
	cfg := fullTestConfig(t.TempDir())
	exec := fakeExecutor{}
	manager := site.NewManager(cfg, logging.NewDefault(os.Stderr), exec)
	siteSpec := model.Site{
		Name: "edit-ui.example.com",
		Domain: model.DomainBinding{
			ServerName: "edit-ui.example.com",
		},
		PHP: model.PHPRuntimeBinding{
			Enabled: true,
		},
	}
	if err := site.ApplyProfile(&siteSpec, site.ProfileReverseProxy, "http://127.0.0.1:8080"); err != nil {
		t.Fatalf("apply reverse proxy profile: %v", err)
	}
	if _, err := manager.Create(context.Background(), site.CreateOptions{
		Site: siteSpec,
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	tuiModel := tui.NewModel("test", cfg, logging.NewDefault(os.Stderr), exec)
	for range 2 {
		updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyTab})
		tuiModel = updated.(tui.Model)
	}

	updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	tuiModel = updated.(tui.Model)
	view := tuiModel.View()
	if !strings.Contains(view, "Edit Site Wizard") {
		t.Fatalf("expected edit site wizard, got %s", view)
	}

	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(filepath.Join(cfg.Paths.SitesRootDir, "custom", "edit-ui.example.com"))})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("www.edit-ui.example.com")})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("index.html,index.php")})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("http://127.0.0.1:9000")})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	tuiModel = updated.(tui.Model)
	view = tuiModel.View()
	if !strings.Contains(view, "Plan Preview") || !strings.Contains(view, "Update managed site edit-ui.example.com") {
		t.Fatalf("expected site update preview, got %s", view)
	}
	if !strings.Contains(view, "index.html") || !strings.Contains(view, "127.0.0.1:9000") {
		t.Fatalf("expected site update preview to include index/upstream, got %s", view)
	}

	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	tuiModel = updated.(tui.Model)
	if !strings.Contains(tuiModel.View(), "pending: update edit-ui.example.com") {
		t.Fatalf("expected update confirmation prompt, got %s", tuiModel.View())
	}

	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tuiModel = updated.(tui.Model)
	view = tuiModel.View()
	if !strings.Contains(view, "last action: update edit-ui.example.com completed") {
		t.Fatalf("expected update feedback, got %s", view)
	}

	manifest, err := manager.Show("edit-ui.example.com")
	if err != nil {
		t.Fatalf("show updated site: %v", err)
	}
	if manifest.Site.DocumentRoot != filepath.Join(cfg.Paths.SitesRootDir, "custom", "edit-ui.example.com") {
		t.Fatalf("expected updated docroot, got %s", manifest.Site.DocumentRoot)
	}
	if len(manifest.Site.Domain.Aliases) != 1 || manifest.Site.Domain.Aliases[0] != "www.edit-ui.example.com" {
		t.Fatalf("expected updated alias, got %#v", manifest.Site.Domain.Aliases)
	}
	if len(manifest.Site.IndexFiles) != 2 || manifest.Site.IndexFiles[0] != "index.html" || manifest.Site.IndexFiles[1] != "index.php" {
		t.Fatalf("expected updated index files, got %#v", manifest.Site.IndexFiles)
	}
	if len(manifest.Site.ReverseProxyRules) != 1 || manifest.Site.ReverseProxyRules[0].Upstream != "http://127.0.0.1:9000" {
		t.Fatalf("expected updated upstream, got %#v", manifest.Site.ReverseProxyRules)
	}
}

func TestDatabaseScreenCanApplyPlan(t *testing.T) {
	cfg := fullTestConfig(t.TempDir())
	exec := fakeExecutor{}
	tuiModel := tui.NewModel("test", cfg, logging.NewDefault(os.Stderr), exec)
	for range 3 {
		updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyTab})
		tuiModel = updated.(tui.Model)
	}
	if tuiModel.ActiveScreen() != tui.ScreenDatabase {
		t.Fatalf("expected database screen, got %q", tuiModel.ActiveScreen())
	}

	for range 3 {
		updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		tuiModel = updated.(tui.Model)
	}
	updated, _ := tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("secret")})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tuiModel = updated.(tui.Model)
	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	tuiModel = updated.(tui.Model)
	if !strings.Contains(tuiModel.View(), "pending: apply current database setup") {
		t.Fatalf("expected database confirmation prompt, got %s", tuiModel.View())
	}

	updated, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tuiModel = updated.(tui.Model)
	view := tuiModel.View()
	if !strings.Contains(view, "last action: database setup applied") {
		t.Fatalf("expected database feedback, got %s", view)
	}
}

func writeManifest(t *testing.T, dir string, name string) {
	t.Helper()
	manifest := model.SiteManifest{
		Site: model.Site{
			Name:    name,
			Backend: "apache",
			State:   "enabled",
			Profile: "generic",
			Domain: model.DomainBinding{
				ServerName: name,
			},
			DocumentRoot: filepath.Join("/data/www", name),
		},
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), raw, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func fullTestConfig(root string) config.RuntimeConfig {
	cfg := config.DefaultRuntimeConfig()
	cfg.Paths.ConfigDir = filepath.Join(root, "etc", "llstack")
	cfg.Paths.StateDir = filepath.Join(root, "var", "lib", "llstack", "state")
	cfg.Paths.HistoryDir = filepath.Join(root, "var", "lib", "llstack", "history")
	cfg.Paths.BackupsDir = filepath.Join(root, "var", "lib", "llstack", "backups")
	cfg.Paths.LogDir = filepath.Join(root, "var", "log", "llstack")
	cfg.Paths.SitesRootDir = filepath.Join(root, "data", "www")
	cfg.Apache.ManagedVhostsDir = filepath.Join(root, "etc", "httpd", "conf.d", "llstack", "sites")
	cfg.OLS.ManagedVhostsRoot = filepath.Join(root, "usr", "local", "lsws", "conf", "vhosts")
	cfg.OLS.ManagedListenersDir = filepath.Join(root, "usr", "local", "lsws", "conf", "llstack", "listeners")
	cfg.LSWS.ManagedIncludesDir = filepath.Join(root, "usr", "local", "lsws", "conf", "llstack", "includes")
	cfg.PHP.ManagedRuntimesDir = filepath.Join(root, "etc", "llstack", "php", "runtimes")
	cfg.PHP.ManagedProfilesDir = filepath.Join(root, "etc", "llstack", "php", "profiles")
	cfg.PHP.ProfileRoot = filepath.Join(root, "etc", "opt", "remi")
	cfg.PHP.RuntimeRoot = filepath.Join(root, "opt", "remi")
	cfg.PHP.StateRoot = filepath.Join(root, "var", "opt", "remi")
	cfg.PHP.ELMajorOverride = "9"
	cfg.DB.ManagedProvidersDir = filepath.Join(root, "etc", "llstack", "db", "providers")
	cfg.DB.ManagedConnectionsDir = filepath.Join(root, "etc", "llstack", "db", "connections")
	cfg.DB.CertificatesDir = filepath.Join(root, "etc", "llstack", "db", "certs")
	cfg.DB.MariaDBTLSConfigPath = filepath.Join(root, "etc", "my.cnf.d", "llstack-mariadb-tls.cnf")
	cfg.DB.MySQLTLSConfigPath = filepath.Join(root, "etc", "my.cnf.d", "llstack-mysql-tls.cnf")
	cfg.DB.PerconaTLSConfigPath = filepath.Join(root, "etc", "my.cnf.d", "llstack-percona-tls.cnf")
	cfg.DB.PostgreSQLTLSConfigPath = filepath.Join(root, "var", "lib", "pgsql", "16", "data", "conf.d", "llstack-tls.conf")
	cfg.Cache.ManagedProvidersDir = filepath.Join(root, "etc", "llstack", "cache", "providers")
	cfg.Cache.MemcachedConfigPath = filepath.Join(root, "etc", "sysconfig", "memcached")
	cfg.Cache.RedisConfigPath = filepath.Join(root, "etc", "redis.conf")
	return cfg
}
