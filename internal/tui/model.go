package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/web-casa/llstack/internal/config"
	coremodel "github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/core/plan"
	"github.com/web-casa/llstack/internal/db"
	"github.com/web-casa/llstack/internal/doctor"
	installsvc "github.com/web-casa/llstack/internal/install"
	"github.com/web-casa/llstack/internal/logging"
	phpruntime "github.com/web-casa/llstack/internal/php"
	cronpkg "github.com/web-casa/llstack/internal/cron"
	"github.com/web-casa/llstack/internal/rollback"
	securitypkg "github.com/web-casa/llstack/internal/security"
	"github.com/web-casa/llstack/internal/site"
	sslprovider "github.com/web-casa/llstack/internal/ssl"
	"github.com/web-casa/llstack/internal/system"
	"github.com/web-casa/llstack/internal/tui/views"
)

// Screen identifies a top-level TUI page.
type Screen string

const (
	ScreenDashboard Screen = "Dashboard"
	ScreenInstall   Screen = "Install"
	ScreenDatabase  Screen = "Database"
	ScreenServices  Screen = "Services"
	ScreenSites     Screen = "Sites"
	ScreenPHP       Screen = "PHP"
	ScreenLogs      Screen = "Logs"
	ScreenDoctor    Screen = "Doctor"
	ScreenHistory   Screen = "History"
	ScreenSSL       Screen = "SSL"
	ScreenCron      Screen = "Cron"
	ScreenSecurity  Screen = "Security"
)

// Model is the root Bubble Tea model for the Phase 1 shell.
type Model struct {
	version       string
	config        config.RuntimeConfig
	exec          system.Executor
	installSvc    installsvc.Service
	dbManager     db.Manager
	doctorSvc     doctor.Service
	phpManager    phpruntime.Manager
	siteManager   site.Manager
	screens       []Screen
	active        int
	installState  installWizardState
	databaseState databaseWizardState
	doctorState   doctorScreenState
	historyState  historyScreenState
	createState   createSiteState
	editState     editSiteState
	siteAction    siteActionState
	siteSelection    int
	sslFeedback      string
	cronFeedback     string
	securityFeedback string
	width            int
	height           int
	quitting      bool
}

type installWizardState struct {
	selected      int
	showPreview   bool
	pendingApply  bool
	feedback      string
	scenario      int
	backend       int
	phpVersion    int
	dbProvider    int
	dbTLS         int
	siteName      int
	siteProfile   int
	withMemcached bool
	withRedis     bool
}

type databaseWizardState struct {
	selected      int
	showPreview   bool
	pendingApply  bool
	feedback      string
	editing       bool
	provider      int
	tlsMode       int
	adminUser     string
	adminPassword string
	databaseName  string
	appUser       string
	appPassword   string
}

type doctorScreenState struct {
	selected       int
	showRepairPlan bool
	pendingApply   bool
	feedback       string
	filterMode     int
}

type historyScreenState struct {
	selected         int
	showRollbackPlan bool
	pendingApply     bool
	feedback         string
	filterMode       int
}

type createSiteState struct {
	showPreview bool
	showWizard  bool
	editing     bool
	selected    int
	serverName  string
	aliases     string
	docroot     string
	backend     int
	profile     int
	phpVersion  int
	upstream    string
}

type editSiteState struct {
	showEditor  bool
	showPreview bool
	editing     bool
	selected    int
	docroot     string
	aliases     string
	indexFiles  string
	upstream    string
	phpVersion  int
	tlsEnabled  bool
}

type siteActionState struct {
	pendingAction string
	pendingSite   string
	feedback      string
	logKind       string
	logLines      int
	logFollow     bool
	logGrep       string
	logGrepEdit   bool
	showTLSPlan   bool
	tlsEmail      string
	phpTarget     string
	showPHPPlan   bool
}

var (
	installScenarioOptions  = []string{"(custom)", "wordpress", "laravel", "api", "static", "reverse-proxy"}
	installBackendOptions   = []string{"apache", "ols", "lsws"}
	installPHPOptions       = []string{"", "7.4", "8.0", "8.1", "8.2", "8.3", "8.4", "8.5"}
	installDBOptions        = []string{"", "mariadb", "mysql", "postgresql", "percona"}
	installDBTLSOptions     = []string{"disabled", "enabled", "required"}
	installSiteOptions      = []string{"", "example.com", "app.example.com"}
	installProfileOptions   = []string{site.ProfileGeneric, site.ProfileWordPress, site.ProfileLaravel, site.ProfileStatic}
	databaseProviderOptions = []string{"mariadb", "mysql", "postgresql", "percona"}
	databaseTLSOptions      = []string{"disabled", "enabled", "required"}
	createBackendOptions    = []string{"apache", "ols", "lsws"}
	createProfileOptions    = []string{site.ProfileGeneric, site.ProfileWordPress, site.ProfileLaravel, site.ProfileStatic, site.ProfileReverseProxy}
	createPHPOptions        = []string{"", "7.4", "8.0", "8.1", "8.2", "8.3", "8.4", "8.5"}
)

// NewModel creates the TUI shell model.
func NewModel(version string, cfg config.RuntimeConfig, logger logging.Logger, exec system.Executor) Model {
	return Model{
		version:     version,
		config:      cfg,
		exec:        exec,
		installSvc:  installsvc.NewService(cfg, logger, exec),
		dbManager:   db.NewManager(cfg, logger, exec),
		doctorSvc:   doctor.NewService(cfg, logger, exec),
		phpManager:  phpruntime.NewManager(cfg, logger, exec),
		siteManager: site.NewManager(cfg, logger, exec),
		databaseState: databaseWizardState{
			adminUser: "llstack_admin",
		},
		createState: createSiteState{
			phpVersion: 3,
		},
		siteAction: siteActionState{
			tlsEmail: "admin@example.com",
			logLines: 10,
		},
		screens: []Screen{
			ScreenDashboard,
			ScreenInstall,
			ScreenSites,
			ScreenDatabase,
			ScreenServices,
			ScreenPHP,
			ScreenLogs,
			ScreenDoctor,
			ScreenHistory,
			ScreenSSL,
			ScreenCron,
			ScreenSecurity,
		},
	}
}

// ActiveScreen returns the currently selected screen.
func (m Model) ActiveScreen() Screen {
	return m.screens[m.active]
}

// SelectedSiteIndex returns the currently selected site row in the Sites screen.
func (m Model) SelectedSiteIndex() int {
	return m.siteSelection
}

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles global navigation and shell-level keys.
type logFollowTickMsg struct{}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case logFollowTickMsg:
		if m.siteAction.logFollow && m.siteAction.logKind != "" && m.ActiveScreen() == ScreenSites {
			m.refreshSiteLogs()
			return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return logFollowTickMsg{} })
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.ActiveScreen() == ScreenInstall && m.installState.pendingApply {
			switch msg.String() {
			case "esc":
				m.installState.pendingApply = false
				m.installState.feedback = "install action canceled"
				return m, nil
			case "enter":
				return m.executeInstallAction()
			}
		}
		if m.ActiveScreen() == ScreenDatabase && m.databaseState.pendingApply {
			switch msg.String() {
			case "esc":
				m.databaseState.pendingApply = false
				m.databaseState.feedback = "database action canceled"
				return m, nil
			case "enter":
				return m.executeDatabaseAction()
			}
		}
		if m.ActiveScreen() == ScreenDoctor && m.doctorState.pendingApply {
			switch msg.String() {
			case "esc":
				m.doctorState.pendingApply = false
				m.doctorState.feedback = "repair action canceled"
				return m, nil
			case "enter":
				return m.executeDoctorRepairAction()
			}
		}
		if m.ActiveScreen() == ScreenHistory && m.historyState.pendingApply {
			switch msg.String() {
			case "esc":
				m.historyState.pendingApply = false
				m.historyState.feedback = "rollback action canceled"
				return m, nil
			case "enter":
				return m.executeHistoryRollbackAction()
			}
		}
		if m.ActiveScreen() == ScreenDatabase && m.databaseState.editing {
			switch msg.String() {
			case "esc":
				m.databaseState.editing = false
				return m, nil
			case "enter":
				m.databaseState.editing = false
				return m, nil
			case "ctrl+u":
				m.updateDatabaseTextField("")
				return m, nil
			case "backspace", "ctrl+h":
				m.updateDatabaseTextField(removeLastRune(m.activeDatabaseText()))
				return m, nil
			default:
				if msg.Type == tea.KeyRunes {
					m.updateDatabaseTextField(m.activeDatabaseText() + string(msg.Runes))
					return m, nil
				}
			}
		}
		if m.ActiveScreen() == ScreenSites && m.createState.showWizard && m.createState.editing {
			switch msg.String() {
			case "esc":
				m.createState.editing = false
				return m, nil
			case "enter":
				m.createState.editing = false
				return m, nil
			case "ctrl+u":
				m.updateCreateSiteTextField("")
				return m, nil
			case "backspace", "ctrl+h":
				m.updateCreateSiteTextField(removeLastRune(m.activeCreateSiteText()))
				return m, nil
			default:
				if msg.Type == tea.KeyRunes {
					m.updateCreateSiteTextField(m.activeCreateSiteText() + string(msg.Runes))
					return m, nil
				}
			}
		}
		if m.ActiveScreen() == ScreenSites && m.editState.showEditor && m.editState.editing {
			switch msg.String() {
			case "esc":
				m.editState.editing = false
				return m, nil
			case "enter":
				m.editState.editing = false
				return m, nil
			case "ctrl+u":
				m.updateEditSiteTextField("")
				return m, nil
			case "backspace", "ctrl+h":
				m.updateEditSiteTextField(removeLastRune(m.activeEditSiteText()))
				return m, nil
			default:
				if msg.Type == tea.KeyRunes {
					m.updateEditSiteTextField(m.activeEditSiteText() + string(msg.Runes))
					return m, nil
				}
			}
		}
		if m.ActiveScreen() == ScreenSites && m.siteAction.logGrepEdit {
			switch msg.String() {
			case "esc", "enter":
				m.siteAction.logGrepEdit = false
				return m, nil
			case "ctrl+u":
				m.siteAction.logGrep = ""
				return m, nil
			case "backspace", "ctrl+h":
				if len(m.siteAction.logGrep) > 0 {
					runes := []rune(m.siteAction.logGrep)
					m.siteAction.logGrep = string(runes[:len(runes)-1])
				}
				return m, nil
			default:
				if msg.Type == tea.KeyRunes {
					m.siteAction.logGrep += string(msg.Runes)
					return m, nil
				}
			}
		}
		if m.ActiveScreen() == ScreenSites && m.siteAction.pendingAction != "" {
			switch msg.String() {
			case "esc":
				m.siteAction.pendingAction = ""
				m.siteAction.pendingSite = ""
				m.siteAction.feedback = "site action canceled"
				return m, nil
			case "enter":
				return m.executePendingSiteAction()
			}
		}

		switch msg.String() {
		case "ctrl+r":
			if m.ActiveScreen() == ScreenSites && !m.createState.showWizard && !m.editState.showEditor {
				m.refreshSiteLogs()
			}
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "tab", "right", "l":
			m.active = (m.active + 1) % len(m.screens)
		case "shift+tab", "left", "h":
			m.active = (m.active - 1 + len(m.screens)) % len(m.screens)
		case "down", "j":
			if m.ActiveScreen() == ScreenInstall {
				m.installState.selected = (m.installState.selected + 1) % 9
			}
			if m.ActiveScreen() == ScreenSites && m.createState.showWizard {
				m.createState.selected = (m.createState.selected + 1) % 7
			} else if m.ActiveScreen() == ScreenSites && m.editState.showEditor {
				m.editState.selected = (m.editState.selected + 1) % 6
			} else if m.ActiveScreen() == ScreenSites {
				m.siteSelection++
			}
			if m.ActiveScreen() == ScreenDatabase {
				m.databaseState.selected = (m.databaseState.selected + 1) % 7
			}
			if m.ActiveScreen() == ScreenDoctor {
				report, err := m.doctorSvc.Run(context.Background())
				checks := m.filterDoctorChecks(report.Checks)
				if err == nil && len(checks) > 0 {
					m.doctorState.selected = (m.doctorState.selected + 1) % len(checks)
				}
			}
			if m.ActiveScreen() == ScreenHistory {
				entries, err := rollback.List(m.config.Paths.HistoryDir, 20)
				entries = m.filterHistoryEntries(entries)
				if err == nil && len(entries) > 0 {
					m.historyState.selected = (m.historyState.selected + 1) % len(entries)
				}
			}
		case "up", "k":
			if m.ActiveScreen() == ScreenInstall {
				m.installState.selected = (m.installState.selected - 1 + 9) % 9
			}
			if m.ActiveScreen() == ScreenSites && m.createState.showWizard {
				m.createState.selected = (m.createState.selected - 1 + 7) % 7
			} else if m.ActiveScreen() == ScreenSites && m.editState.showEditor {
				m.editState.selected = (m.editState.selected - 1 + 6) % 6
			} else if m.ActiveScreen() == ScreenSites && m.siteSelection > 0 {
				m.siteSelection--
			}
			if m.ActiveScreen() == ScreenDatabase {
				m.databaseState.selected = (m.databaseState.selected - 1 + 7) % 7
			}
			if m.ActiveScreen() == ScreenDoctor {
				report, err := m.doctorSvc.Run(context.Background())
				checks := m.filterDoctorChecks(report.Checks)
				if err == nil && len(checks) > 0 {
					m.doctorState.selected = (m.doctorState.selected - 1 + len(checks)) % len(checks)
				}
			}
			if m.ActiveScreen() == ScreenHistory {
				entries, err := rollback.List(m.config.Paths.HistoryDir, 20)
				entries = m.filterHistoryEntries(entries)
				if err == nil && len(entries) > 0 {
					m.historyState.selected = (m.historyState.selected - 1 + len(entries)) % len(entries)
				}
			}
		case "p":
			if m.ActiveScreen() == ScreenInstall {
				m.installState.showPreview = !m.installState.showPreview
			}
			if m.ActiveScreen() == ScreenDatabase {
				m.databaseState.showPreview = !m.databaseState.showPreview
			}
			if m.ActiveScreen() == ScreenSites && m.createState.showWizard {
				m.createState.showPreview = !m.createState.showPreview
			}
			if m.ActiveScreen() == ScreenSites && m.editState.showEditor {
				m.editState.showPreview = !m.editState.showPreview
			}
			if m.ActiveScreen() == ScreenDoctor {
				m.doctorState.showRepairPlan = !m.doctorState.showRepairPlan
			}
			if m.ActiveScreen() == ScreenHistory {
				m.historyState.showRollbackPlan = !m.historyState.showRollbackPlan
			}
		case "f":
			if m.ActiveScreen() == ScreenDoctor {
				m.doctorState.filterMode = (m.doctorState.filterMode + 1) % 3
				m.doctorState.selected = 0
				m.doctorState.feedback = "doctor filter: " + m.doctorFilterLabel()
			}
			if m.ActiveScreen() == ScreenHistory {
				m.historyState.filterMode = (m.historyState.filterMode + 1) % 3
				m.historyState.selected = 0
				m.historyState.feedback = "history filter: " + m.historyFilterLabel()
			}
		case "g":
			if m.ActiveScreen() == ScreenSites && !m.createState.showWizard && !m.editState.showEditor {
				m.toggleSiteLogs()
			}
		case "]":
			if m.ActiveScreen() == ScreenSites && !m.createState.showWizard && !m.editState.showEditor {
				m.adjustSiteLogLines(5)
			}
		case "[":
			if m.ActiveScreen() == ScreenSites && !m.createState.showWizard && !m.editState.showEditor {
				m.adjustSiteLogLines(-5)
			}
		case "w":
			if m.ActiveScreen() == ScreenSites && !m.createState.showWizard && !m.editState.showEditor {
				m.siteAction.logFollow = !m.siteAction.logFollow
				if m.siteAction.logFollow {
					m.siteAction.feedback = "log follow enabled (auto-refresh every 2s)"
					return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return logFollowTickMsg{} })
				} else {
					m.siteAction.feedback = "log follow disabled"
				}
			}
		case "/":
			if m.ActiveScreen() == ScreenSites && !m.createState.showWizard && !m.editState.showEditor && m.siteAction.logKind != "" {
				m.siteAction.logGrepEdit = !m.siteAction.logGrepEdit
				if !m.siteAction.logGrepEdit && m.siteAction.logGrep == "" {
					m.siteAction.feedback = "grep filter cleared"
				}
			}
		case "c":
			if m.ActiveScreen() == ScreenSites {
				m.toggleCreateWizard()
			}
		case "m":
			if m.ActiveScreen() == ScreenSites {
				m.toggleEditWizard()
			}
		case "n":
			if m.ActiveScreen() == ScreenSites && m.createState.showWizard {
				m.stageCreateSiteAction()
			}
			if m.ActiveScreen() == ScreenSites && m.editState.showEditor {
				m.stageEditSiteAction()
			}
		case "u":
			if m.ActiveScreen() == ScreenSites && !m.createState.showWizard && !m.editState.showEditor {
				m.stageSitePHPApplyAction()
			}
		case "a":
			if m.ActiveScreen() == ScreenInstall {
				m.stageInstallApplyAction()
			}
			if m.ActiveScreen() == ScreenDatabase {
				m.stageDatabaseApplyAction()
			}
			if m.ActiveScreen() == ScreenSites && !m.createState.showWizard && !m.editState.showEditor {
				m.stageSiteTLSApplyAction()
			}
			if m.ActiveScreen() == ScreenDoctor {
				m.stageDoctorRepairAction()
			}
			if m.ActiveScreen() == ScreenHistory {
				m.stageHistoryRollbackAction()
			}
			if m.ActiveScreen() == ScreenSSL {
				m.sslFeedback = "renewing expiring certificates..."
				lm := sslprovider.NewLifecycleManager(m.config, m.exec)
				manifests, _ := m.siteManager.List()
				var sites []sslprovider.SiteInfo
				for _, mf := range manifests {
					if mf.Site.TLS.Enabled {
						sites = append(sites, sslprovider.SiteInfo{
							Name: mf.Site.Name, CertFile: mf.Site.TLS.CertificateFile,
							Domain: mf.Site.Domain.ServerName, Docroot: mf.Site.DocumentRoot,
						})
					}
				}
				plans, err := lm.RenewExpiring(context.Background(), sites, sslprovider.RenewAllOptions{ThresholdDays: sslprovider.ExpiryThresholdDays})
				if err != nil {
					m.sslFeedback = "renew error: " + err.Error()
				} else {
					m.sslFeedback = fmt.Sprintf("renewed %d certificate(s)", len(plans))
				}
			}
			if m.ActiveScreen() == ScreenCron {
				m.cronFeedback = "use CLI: llstack cron:add <site> --preset wp-cron"
			}
		case "r":
			if m.ActiveScreen() == ScreenSites && !m.createState.showWizard && !m.editState.showEditor {
				m.stageSiteAction("reload")
			}
		case "x":
			if m.ActiveScreen() == ScreenSites && !m.createState.showWizard && !m.editState.showEditor {
				m.stageSiteAction("restart")
			}
		case "s":
			if m.ActiveScreen() == ScreenSites && !m.createState.showWizard && !m.editState.showEditor {
				m.stageSiteToggleAction()
			}
		case "v":
			if m.ActiveScreen() == ScreenSites && !m.createState.showWizard && !m.editState.showEditor {
				m.cycleSitePHPVersion()
			}
		case "t":
			if m.ActiveScreen() == ScreenSites && !m.createState.showWizard && !m.editState.showEditor {
				m.siteAction.showTLSPlan = !m.siteAction.showTLSPlan
			}
		case " ", "enter":
			if m.ActiveScreen() == ScreenInstall {
				m.toggleInstallField(1)
			}
			if m.ActiveScreen() == ScreenDatabase {
				m.toggleDatabaseField(1)
			}
			if m.ActiveScreen() == ScreenSites && m.createState.showWizard {
				m.toggleCreateSiteField(1)
			}
			if m.ActiveScreen() == ScreenSites && m.editState.showEditor {
				m.toggleEditSiteField(1)
			}
		case "e":
			if m.ActiveScreen() == ScreenDatabase && m.databaseState.selected >= 2 {
				m.databaseState.editing = !m.databaseState.editing
			}
			if m.ActiveScreen() == ScreenSites && m.createState.showWizard && (m.createState.selected == 0 || m.createState.selected == 1 || m.createState.selected == 2 || m.createState.selected == 6) {
				m.createState.editing = !m.createState.editing
			}
			if m.ActiveScreen() == ScreenSites && m.editState.showEditor {
				m.editState.editing = !m.editState.editing
			}
		case "L":
			if m.ActiveScreen() == ScreenInstall {
				m.toggleInstallField(1)
			}
			if m.ActiveScreen() == ScreenDatabase {
				m.toggleDatabaseField(1)
			}
			if m.ActiveScreen() == ScreenSites && m.createState.showWizard {
				m.toggleCreateSiteField(1)
			}
		case "H":
			if m.ActiveScreen() == ScreenInstall {
				m.toggleInstallField(-1)
			}
			if m.ActiveScreen() == ScreenDatabase {
				m.toggleDatabaseField(-1)
			}
			if m.ActiveScreen() == ScreenSites && m.createState.showWizard {
				m.toggleCreateSiteField(-1)
			}
		}
	}

	return m, nil
}

// View renders the shell and the active page.
func (m Model) View() string {
	if m.quitting {
		return "LLStack TUI closed.\n"
	}

	content := m.renderContent()
	nav := m.renderNav()
	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("tab/h/l switch  j/k move  enter confirm/toggle  e edit  p preview  f filter  c create  m edit-site  n apply-wizard  s stop/start  r reload  x restart  g logs  [/ ] log-lines  ctrl+r refresh  t tls-plan  a apply-tls  v php-target  u apply-php  q quit")

	frame := lipgloss.NewStyle().
		Padding(1, 2).
		Width(max(m.width, 80)).
		Render(nav + "\n\n" + content + "\n\n" + footer)

	return frame
}

func (m Model) renderNav() string {
	var items []string

	selected := lipgloss.NewStyle().
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("62")).
		Padding(0, 1)
	normal := lipgloss.NewStyle().
		Foreground(lipgloss.Color("250")).
		Padding(0, 1)

	for idx, screen := range m.screens {
		label := string(screen)
		if idx == m.active {
			items = append(items, selected.Render(label))
			continue
		}
		items = append(items, normal.Render(label))
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("81")).
		Render(fmt.Sprintf("LLStack %s", m.version))

	return title + "\n" + strings.Join(items, " ")
}

func (m Model) renderContent() string {
	switch m.ActiveScreen() {
	case ScreenDashboard:
		return views.RenderDashboard(m.config)
	case ScreenInstall:
		return m.renderInstallScreen()
	case ScreenServices:
		return views.RenderServices(m.config)
	case ScreenSites:
		return m.renderSitesScreen()
	case ScreenDatabase:
		return m.renderDatabaseScreen()
	case ScreenPHP:
		return views.RenderPHP(m.config)
	case ScreenLogs:
		return views.RenderLogs(m.config)
	case ScreenDoctor:
		return m.renderDoctorScreen()
	case ScreenHistory:
		return m.renderHistoryScreen()
	case ScreenSSL:
		return m.renderSSLScreen()
	case ScreenCron:
		return m.renderCronScreen()
	case ScreenSecurity:
		return m.renderSecurityScreen()
	default:
		return "Unknown screen"
	}
}

func (m *Model) toggleInstallField(direction int) {
	switch m.installState.selected {
	case 0:
		m.installState.scenario = cycleIndex(m.installState.scenario, len(installScenarioOptions), direction)
		m.applyInstallScenario()
	case 1:
		m.installState.backend = cycleIndex(m.installState.backend, len(installBackendOptions), direction)
	case 2:
		m.installState.phpVersion = cycleIndex(m.installState.phpVersion, len(installPHPOptions), direction)
	case 3:
		m.installState.dbProvider = cycleIndex(m.installState.dbProvider, len(installDBOptions), direction)
	case 4:
		m.installState.dbTLS = cycleIndex(m.installState.dbTLS, len(installDBTLSOptions), direction)
	case 5:
		m.installState.withMemcached = !m.installState.withMemcached
	case 6:
		m.installState.withRedis = !m.installState.withRedis
	case 7:
		m.installState.siteName = cycleIndex(m.installState.siteName, len(installSiteOptions), direction)
	case 8:
		m.installState.siteProfile = cycleIndex(m.installState.siteProfile, len(installProfileOptions), direction)
	}
}

func (m *Model) applyInstallScenario() {
	name := installScenarioOptions[m.installState.scenario]
	switch name {
	case "wordpress":
		m.installState.phpVersion = indexOf(installPHPOptions, "8.3")
		m.installState.dbProvider = indexOf(installDBOptions, "mariadb")
		m.installState.dbTLS = indexOf(installDBTLSOptions, "enabled")
		m.installState.withMemcached = true
		m.installState.withRedis = false
		m.installState.siteProfile = indexOf(installProfileOptions, site.ProfileWordPress)
	case "laravel":
		m.installState.phpVersion = indexOf(installPHPOptions, "8.3")
		m.installState.dbProvider = indexOf(installDBOptions, "postgresql")
		m.installState.dbTLS = indexOf(installDBTLSOptions, "enabled")
		m.installState.withMemcached = false
		m.installState.withRedis = true
		m.installState.siteProfile = indexOf(installProfileOptions, site.ProfileLaravel)
	case "api":
		m.installState.phpVersion = indexOf(installPHPOptions, "8.3")
		m.installState.dbProvider = indexOf(installDBOptions, "postgresql")
		m.installState.withMemcached = false
		m.installState.withRedis = true
		m.installState.siteProfile = indexOf(installProfileOptions, site.ProfileGeneric)
	case "static":
		m.installState.phpVersion = 0
		m.installState.dbProvider = 0
		m.installState.dbTLS = 0
		m.installState.withMemcached = false
		m.installState.withRedis = false
		m.installState.siteProfile = indexOf(installProfileOptions, site.ProfileStatic)
	case "reverse-proxy":
		m.installState.phpVersion = 0
		m.installState.dbProvider = 0
		m.installState.dbTLS = 0
		m.installState.withMemcached = false
		m.installState.withRedis = false
		m.installState.siteProfile = indexOf(installProfileOptions, site.ProfileGeneric)
	}
}

func indexOf(options []string, value string) int {
	for i, opt := range options {
		if opt == value {
			return i
		}
	}
	return 0
}

func (m *Model) toggleDatabaseField(direction int) {
	switch m.databaseState.selected {
	case 0:
		m.databaseState.provider = cycleIndex(m.databaseState.provider, len(databaseProviderOptions), direction)
	case 1:
		m.databaseState.tlsMode = cycleIndex(m.databaseState.tlsMode, len(databaseTLSOptions), direction)
	default:
		m.databaseState.editing = !m.databaseState.editing
	}
}

func (m *Model) toggleCreateSiteField(direction int) {
	switch m.createState.selected {
	case 0, 1, 2, 6:
		m.createState.editing = !m.createState.editing
	case 3:
		m.createState.backend = cycleIndex(m.createState.backend, len(createBackendOptions), direction)
	case 4:
		m.createState.profile = cycleIndex(m.createState.profile, len(createProfileOptions), direction)
	case 5:
		m.createState.phpVersion = cycleIndex(m.createState.phpVersion, len(createPHPOptions), direction)
	}
}

func (m *Model) toggleEditSiteField(direction int) {
	switch m.editState.selected {
	case 0, 1, 2, 3:
		m.editState.editing = !m.editState.editing
	case 4:
		m.editState.phpVersion = cycleIndex(m.editState.phpVersion, 5, direction)
	case 5:
		m.editState.tlsEnabled = !m.editState.tlsEnabled
	}
}

func (m Model) renderInstallScreen() string {
	lines := []string{"Install Wizard", ""}
	fields := []struct {
		label string
		value string
	}{
		{"scenario", installScenarioOptions[m.installState.scenario]},
		{"backend", installBackendOptions[m.installState.backend]},
		{"php_version", emptyFallback(installPHPOptions[m.installState.phpVersion], "disabled")},
		{"db_provider", emptyFallback(installDBOptions[m.installState.dbProvider], "none")},
		{"db_tls", installDBTLSOptions[m.installState.dbTLS]},
		{"memcached", boolLabel(m.installState.withMemcached)},
		{"redis", boolLabel(m.installState.withRedis)},
		{"first_site", emptyFallback(installSiteOptions[m.installState.siteName], "skip")},
		{"site_profile", installProfileOptions[m.installState.siteProfile]},
	}
	for idx, field := range fields {
		marker := " "
		if idx == m.installState.selected {
			marker = ">"
		}
		lines = append(lines, fmt.Sprintf("%s %-12s %s", marker, field.label, field.value))
	}
	lines = append(lines, "", "Keys: j/k select  enter/space toggle  p preview  a apply")
	if m.installState.pendingApply {
		lines = append(lines, "", "pending: install current plan", "press enter to confirm or esc to cancel")
	}
	if m.installState.feedback != "" {
		lines = append(lines, "", "last action: "+m.installState.feedback)
	}

	if !m.installState.showPreview {
		lines = append(lines, "", "Plan preview hidden. Press `p` to render the current install plan.")
		return strings.Join(lines, "\n")
	}

	plan, err := m.installSvc.Execute(context.Background(), installsvc.Options{
		Backend:       installBackendOptions[m.installState.backend],
		PHPVersion:    installPHPOptions[m.installState.phpVersion],
		DBProvider:    installDBOptions[m.installState.dbProvider],
		DBTLS:         installDBTLSOptions[m.installState.dbTLS],
		WithMemcached: m.installState.withMemcached,
		WithRedis:     m.installState.withRedis,
		Site:          installSiteOptions[m.installState.siteName],
		SiteProfile:   installProfileOptions[m.installState.siteProfile],
		DryRun:        true,
	})
	lines = append(lines, "", "Plan Preview", "")
	if err != nil {
		lines = append(lines, "error: "+err.Error())
		return strings.Join(lines, "\n")
	}
	lines = append(lines, plan.Summary)
	for _, warning := range plan.Warnings {
		lines = append(lines, "warning: "+warning)
	}
	lines = append(lines, renderPlanOperations(plan.Operations, 12, "")...)
	return strings.Join(lines, "\n")
}

func (m Model) renderDatabaseScreen() string {
	lines := []string{"Database Setup", ""}
	fields := []struct {
		label string
		value string
	}{
		{"provider", databaseProviderOptions[m.databaseState.provider]},
		{"tls_mode", databaseTLSOptions[m.databaseState.tlsMode]},
		{"admin_user", editableValue(m.databaseState.adminUser)},
		{"admin_password", maskedValue(m.databaseState.adminPassword)},
		{"database_name", editableValue(m.databaseState.databaseName)},
		{"app_user", editableValue(m.databaseState.appUser)},
		{"app_password", maskedValue(m.databaseState.appPassword)},
	}
	for idx, field := range fields {
		marker := " "
		if idx == m.databaseState.selected {
			marker = ">"
		}
		suffix := ""
		if m.databaseState.editing && idx == m.databaseState.selected && idx >= 2 {
			suffix = "  [editing]"
		}
		lines = append(lines, fmt.Sprintf("%s %-14s %s%s", marker, field.label, field.value, suffix))
	}

	lines = append(lines, "", "Keys: j/k select  enter toggle/edit  e edit text  p preview  a apply")
	if m.databaseState.pendingApply {
		lines = append(lines, "", "pending: apply current database setup", "press enter to confirm or esc to cancel")
	}
	if m.databaseState.feedback != "" {
		lines = append(lines, "", "last action: "+m.databaseState.feedback)
	}

	managedProviders, err := m.dbManager.List()
	if err != nil {
		lines = append(lines, "", "Managed providers", "", "error: "+err.Error())
	} else {
		lines = append(lines, "", "Managed providers", "")
		if len(managedProviders) == 0 {
			lines = append(lines, "none")
		} else {
			for _, manifest := range managedProviders {
				lines = append(lines, fmt.Sprintf("- %s  status=%s  tls=%s", manifest.Provider, manifest.Status, manifest.TLS.Mode))
			}
		}
	}

	if !m.databaseState.showPreview {
		lines = append(lines, "", "Plan preview hidden. Press `p` to render the current database setup plan.")
		return strings.Join(lines, "\n")
	}

	lines = append(lines, "", "Plan Preview", "")
	previewLines, err := m.renderDatabasePlanPreview()
	if err != nil {
		lines = append(lines, "error: "+err.Error())
		return strings.Join(lines, "\n")
	}
	lines = append(lines, previewLines...)
	return strings.Join(lines, "\n")
}

func (m Model) renderDoctorScreen() string {
	lines := []string{"Doctor / Repair", ""}
	report, err := m.doctorSvc.Run(context.Background())
	if err != nil {
		lines = append(lines, "error: "+err.Error())
		return strings.Join(lines, "\n")
	}
	checks := m.filterDoctorChecks(report.Checks)
	passCount, warnCount, failCount := doctorCheckCounts(report.Checks)
	repairable, manual := doctorRepairCoverage(report.Checks)
	lines = append(lines, fmt.Sprintf("status=%s  generated_at=%s", report.Status, report.GeneratedAt.Format("2006-01-02 15:04:05Z")))
	lines = append(lines, fmt.Sprintf("filter=%s  visible=%d  counts(pass=%d warn=%d fail=%d)", m.doctorFilterLabel(), len(checks), passCount, warnCount, failCount))
	lines = append(lines, fmt.Sprintf("repair_coverage=auto:%d manual:%d", len(repairable), len(manual)))
	if len(repairable) > 0 {
		lines = append(lines, "auto_repair: "+strings.Join(limitStrings(repairable, 4), ", "))
	}
	if len(manual) > 0 {
		lines = append(lines, "manual_only: "+strings.Join(limitStrings(manual, 4), ", "))
	}
	lines = append(lines, "", "Checks", "")
	if len(checks) == 0 {
		lines = append(lines, "no checks match the current filter")
		lines = append(lines, "", "Keys: j/k select  f cycle filter  p repair preview  a apply repair")
		if m.doctorState.pendingApply {
			lines = append(lines, "", "pending: apply current repair plan", "press enter to confirm or esc to cancel")
		}
		if m.doctorState.feedback != "" {
			lines = append(lines, "", "last action: "+m.doctorState.feedback)
		}
		if !m.doctorState.showRepairPlan {
			lines = append(lines, "", "Repair plan hidden. Press `p` to render the current repair plan.")
			return strings.Join(lines, "\n")
		}
		p, err := m.doctorSvc.Repair(context.Background(), doctor.RepairOptions{
			DryRun:     true,
			SkipReload: true,
		})
		lines = append(lines, "", "Repair Plan Preview", "")
		if err != nil {
			lines = append(lines, "error: "+err.Error())
			return strings.Join(lines, "\n")
		}
		lines = append(lines, p.Summary)
		for _, warning := range p.Warnings {
			lines = append(lines, "warning: "+warning)
		}
		lines = append(lines, renderPlanOperations(p.Operations, 10, "")...)
		return strings.Join(lines, "\n")
	}
	if len(report.Checks) == 0 {
		lines = append(lines, "no checks")
	} else {
		if m.doctorState.selected >= len(checks) {
			m.doctorState.selected = len(checks) - 1
		}
		for idx, check := range checks {
			marker := " "
			if idx == m.doctorState.selected {
				marker = ">"
			}
			lines = append(lines, fmt.Sprintf("%s %-24s %-5s %s", marker, check.Name, check.Status, check.Summary))
		}

		if m.doctorState.selected >= len(checks) {
			m.doctorState.selected = 0
		}
		selected := checks[m.doctorState.selected]
		lines = append(lines, "", "Selected Check", "", fmt.Sprintf("name=%s", selected.Name), fmt.Sprintf("status=%s", selected.Status))
		if selected.Repairable {
			lines = append(lines, "repairable=yes")
		}
		if len(selected.Details) > 0 {
			keys := make([]string, 0, len(selected.Details))
			for key := range selected.Details {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				lines = append(lines, fmt.Sprintf("%s=%s", key, selected.Details[key]))
			}
		}
	}

	lines = append(lines, "", "Keys: j/k select  f cycle filter  p repair preview  a apply repair")
	if m.doctorState.pendingApply {
		lines = append(lines, "", "pending: apply current repair plan", "press enter to confirm or esc to cancel")
	}
	if m.doctorState.feedback != "" {
		lines = append(lines, "", "last action: "+m.doctorState.feedback)
	}

	if !m.doctorState.showRepairPlan {
		lines = append(lines, "", "Repair plan hidden. Press `p` to render the current repair plan.")
		return strings.Join(lines, "\n")
	}

	p, err := m.doctorSvc.Repair(context.Background(), doctor.RepairOptions{
		DryRun:     true,
		SkipReload: true,
	})
	lines = append(lines, "", "Repair Plan Preview", "")
	if err != nil {
		lines = append(lines, "error: "+err.Error())
		return strings.Join(lines, "\n")
	}
	lines = append(lines, p.Summary)
	for _, warning := range p.Warnings {
		lines = append(lines, "warning: "+warning)
	}
	lines = append(lines, renderPlanOperations(p.Operations, 10, "")...)
	return strings.Join(lines, "\n")
}

func (m Model) renderHistoryScreen() string {
	lines := []string{"Rollback History", ""}
	entries, err := rollback.List(m.config.Paths.HistoryDir, 20)
	if err != nil {
		lines = append(lines, "error: "+err.Error())
		return strings.Join(lines, "\n")
	}
	if len(entries) == 0 {
		lines = append(lines, "no rollback history available")
		return strings.Join(lines, "\n")
	}
	entries = m.filterHistoryEntries(entries)
	if len(entries) == 0 {
		lines = append(lines, "filter="+m.historyFilterLabel(), "", "no rollback history entries match the current filter")
		return strings.Join(lines, "\n")
	}
	if m.historyState.selected >= len(entries) {
		m.historyState.selected = len(entries) - 1
	}
	if m.historyState.selected < 0 {
		m.historyState.selected = 0
	}

	var latestPendingPath string
	latestPending, err := rollback.LoadLatestPending(m.config.Paths.HistoryDir)
	if err == nil {
		latestPendingPath = latestPending.Path
	}

	lines = append(lines, fmt.Sprintf("filter=%s  visible=%d", m.historyFilterLabel(), len(entries)), "", "Entries", "")
	for idx, entry := range entries {
		marker := " "
		if idx == m.historyState.selected {
			marker = ">"
		}
		state := "rolled-back"
		if !entry.RolledBack {
			state = "pending"
		}
		if entry.Path == latestPendingPath {
			state += " latest"
		}
		lines = append(lines, fmt.Sprintf("%s %s  %s  %s  %s", marker, entry.Timestamp.Format("2006-01-02 15:04:05"), entry.Action, entry.Resource, state))
	}

	if m.historyState.selected >= len(entries) {
		m.historyState.selected = 0
	}
	selected := entries[m.historyState.selected]
	lines = append(lines, "", "Selected Entry", "")
	lines = append(lines,
		fmt.Sprintf("id=%s", selected.ID),
		fmt.Sprintf("action=%s", selected.Action),
		fmt.Sprintf("resource=%s", selected.Resource),
		fmt.Sprintf("backend=%s", emptyFallback(selected.Backend, "(none)")),
		fmt.Sprintf("timestamp=%s", selected.Timestamp.Format("2006-01-02 15:04:05Z07:00")),
		fmt.Sprintf("rolled_back=%t", selected.RolledBack),
		fmt.Sprintf("changes=%d", len(selected.Changes)),
	)
	if selected.Path == latestPendingPath {
		lines = append(lines, "rollbackable=yes (latest pending entry)")
	} else if !selected.RolledBack {
		lines = append(lines, "rollbackable=no (only latest pending entry can be rolled back)")
	}

	lines = append(lines, "", "Keys: j/k select  f cycle filter  p rollback preview  a apply rollback")
	if m.historyState.pendingApply {
		lines = append(lines, "", "pending: apply latest rollback plan", "press enter to confirm or esc to cancel")
	}
	if m.historyState.feedback != "" {
		lines = append(lines, "", "last action: "+m.historyState.feedback)
	}

	if !m.historyState.showRollbackPlan {
		lines = append(lines, "", "Rollback plan hidden. Press `p` to render the latest rollback plan.")
		return strings.Join(lines, "\n")
	}

	lines = append(lines, "", "Rollback Plan Preview", "")
	if selected.Path != latestPendingPath {
		if selected.RolledBack {
			lines = append(lines, "selected entry is already rolled back")
		} else {
			lines = append(lines, "selected entry is not the latest pending rollback target")
		}
		return strings.Join(lines, "\n")
	}

	p, err := m.siteManager.RollbackLast(context.Background(), site.RollbackOptions{
		DryRun:     true,
		SkipReload: true,
	})
	if err != nil {
		lines = append(lines, "error: "+err.Error())
		return strings.Join(lines, "\n")
	}
	lines = append(lines, p.Summary)
	for _, warning := range p.Warnings {
		lines = append(lines, "warning: "+warning)
	}
	lines = append(lines, renderPlanOperations(p.Operations, 10, "")...)
	return strings.Join(lines, "\n")
}

func (m *Model) stageInstallApplyAction() {
	m.installState.pendingApply = true
	m.installState.feedback = ""
}

func (m *Model) stageDatabaseApplyAction() {
	m.databaseState.pendingApply = true
	m.databaseState.feedback = ""
}

func (m *Model) stageDoctorRepairAction() {
	m.doctorState.pendingApply = true
	m.doctorState.feedback = ""
}

func (m *Model) stageHistoryRollbackAction() {
	selected, ok := m.selectedHistoryEntry()
	if !ok {
		m.historyState.feedback = "no rollback history available"
		return
	}
	if selected.RolledBack {
		m.historyState.feedback = "selected history entry is already rolled back"
		return
	}
	latestPending, ok := m.latestPendingHistoryEntry()
	if !ok {
		m.historyState.feedback = "no pending rollback entry available"
		return
	}
	if selected.Path != latestPending.Path {
		m.historyState.feedback = "only the latest pending history entry can be rolled back"
		return
	}
	m.historyState.pendingApply = true
	m.historyState.feedback = ""
}

func (m *Model) executeInstallAction() (tea.Model, tea.Cmd) {
	m.installState.pendingApply = false
	_, err := m.installSvc.Execute(context.Background(), installsvc.Options{
		Backend:       installBackendOptions[m.installState.backend],
		PHPVersion:    installPHPOptions[m.installState.phpVersion],
		DBProvider:    installDBOptions[m.installState.dbProvider],
		DBTLS:         installDBTLSOptions[m.installState.dbTLS],
		WithMemcached: m.installState.withMemcached,
		WithRedis:     m.installState.withRedis,
		Site:          installSiteOptions[m.installState.siteName],
		SiteProfile:   installProfileOptions[m.installState.siteProfile],
	})
	if err != nil {
		m.installState.feedback = "install failed: " + err.Error()
		return *m, nil
	}
	m.installState.feedback = "install plan applied"
	return *m, nil
}

func (m *Model) executeDatabaseAction() (tea.Model, tea.Cmd) {
	m.databaseState.pendingApply = false
	plans, err := m.databaseWorkflow(context.Background(), false)
	if err != nil {
		m.databaseState.feedback = "database setup failed: " + err.Error()
		return *m, nil
	}
	if len(plans) == 0 {
		m.databaseState.feedback = "database setup had no operations"
		return *m, nil
	}
	m.databaseState.feedback = "database setup applied"
	return *m, nil
}

func (m *Model) executeDoctorRepairAction() (tea.Model, tea.Cmd) {
	m.doctorState.pendingApply = false
	p, err := m.doctorSvc.Repair(context.Background(), doctor.RepairOptions{})
	if err != nil {
		m.doctorState.feedback = "repair failed: " + err.Error()
		return *m, nil
	}
	if len(p.Operations) == 0 {
		m.doctorState.feedback = "repair had no operations"
		return *m, nil
	}
	m.doctorState.feedback = "repair plan applied"
	return *m, nil
}

func (m *Model) executeHistoryRollbackAction() (tea.Model, tea.Cmd) {
	m.historyState.pendingApply = false
	p, err := m.siteManager.RollbackLast(context.Background(), site.RollbackOptions{})
	if err != nil {
		m.historyState.feedback = "rollback failed: " + err.Error()
		return *m, nil
	}
	if len(p.Operations) == 0 {
		m.historyState.feedback = "rollback had no operations"
		return *m, nil
	}
	m.historyState.feedback = "rollback applied"
	return *m, nil
}

func (m Model) renderDatabasePlanPreview() ([]string, error) {
	plans, err := m.databaseWorkflow(context.Background(), true)
	if err != nil {
		return nil, err
	}
	blocks := make([]planBlock, 0, len(plans))
	for _, p := range plans {
		blocks = append(blocks, planBlock{
			title:      planBlockTitle(p.Kind),
			summary:    p.Summary,
			warnings:   p.Warnings,
			operations: p.Operations,
		})
	}
	return renderPlanBlocks(blocks), nil
}

func (m Model) databaseWorkflow(ctx context.Context, dryRun bool) ([]plan.Plan, error) {
	provider := db.ProviderName(databaseProviderOptions[m.databaseState.provider])
	tlsMode := db.TLSMode(databaseTLSOptions[m.databaseState.tlsMode])
	plans := make([]plan.Plan, 0, 4)

	installPlan, err := m.dbManager.Install(ctx, db.InstallOptions{
		Provider: provider,
		TLSMode:  tlsMode,
		DryRun:   dryRun,
	})
	if err != nil {
		return nil, err
	}
	plans = append(plans, installPlan)

	initPlan, err := m.dbManager.Init(ctx, db.InitOptions{
		Provider:      provider,
		AdminUser:     strings.TrimSpace(m.databaseState.adminUser),
		AdminPassword: m.databaseState.adminPassword,
		TLSMode:       tlsMode,
		DryRun:        dryRun,
	})
	if err != nil {
		return nil, err
	}
	plans = append(plans, initPlan)

	if name := strings.TrimSpace(m.databaseState.databaseName); name != "" {
		createPlan, err := m.dbManager.CreateDatabase(ctx, db.CreateDatabaseOptions{
			Provider: provider,
			Name:     name,
			Owner:    strings.TrimSpace(m.databaseState.appUser),
			DryRun:   dryRun,
		})
		if err != nil {
			return nil, err
		}
		plans = append(plans, createPlan)
	}

	appUser := strings.TrimSpace(m.databaseState.appUser)
	if appUser != "" {
		if strings.TrimSpace(m.databaseState.databaseName) == "" {
			return nil, fmt.Errorf("database_name is required before planning app_user creation")
		}
		if m.databaseState.appPassword == "" {
			return nil, fmt.Errorf("app_password is required before planning app_user creation")
		}
		userPlan, err := m.dbManager.CreateUser(ctx, db.CreateUserOptions{
			Provider: provider,
			Name:     appUser,
			Password: m.databaseState.appPassword,
			Database: strings.TrimSpace(m.databaseState.databaseName),
			TLSMode:  tlsMode,
			DryRun:   dryRun,
		})
		if err != nil {
			return nil, err
		}
		plans = append(plans, userPlan)
	}
	return plans, nil
}

func renderPlanBlocks(blocks []planBlock) []string {
	lines := make([]string, 0, 24)
	for _, block := range blocks {
		lines = append(lines, block.title)
		lines = append(lines, "  "+block.summary)
		for _, warning := range block.warnings {
			lines = append(lines, "  warning: "+warning)
		}
		lines = append(lines, renderPlanOperations(block.operations, 6, "  ")...)
		lines = append(lines, "")
	}
	if len(lines) == 0 {
		return []string{"no plan generated"}
	}
	return lines[:len(lines)-1]
}

func planBlockTitle(kind string) string {
	switch kind {
	case "db.install":
		return "Install Provider"
	case "db.init":
		return "Initialize Provider"
	case "db.create":
		return "Create Database"
	case "db.user.create":
		return "Create App User"
	default:
		return kind
	}
}

type planBlock struct {
	title      string
	summary    string
	warnings   []string
	operations []plan.Operation
}

func (m Model) activeDatabaseText() string {
	switch m.databaseState.selected {
	case 2:
		return m.databaseState.adminUser
	case 3:
		return m.databaseState.adminPassword
	case 4:
		return m.databaseState.databaseName
	case 5:
		return m.databaseState.appUser
	case 6:
		return m.databaseState.appPassword
	default:
		return ""
	}
}

func (m *Model) updateDatabaseTextField(value string) {
	switch m.databaseState.selected {
	case 2:
		m.databaseState.adminUser = value
	case 3:
		m.databaseState.adminPassword = value
	case 4:
		m.databaseState.databaseName = value
	case 5:
		m.databaseState.appUser = value
	case 6:
		m.databaseState.appPassword = value
	}
}

func cycleIndex(current int, length int, direction int) int {
	if length == 0 {
		return 0
	}
	if direction >= 0 {
		return (current + 1) % length
	}
	return (current - 1 + length) % length
}

func emptyFallback(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func boolLabel(value bool) string {
	if value {
		return "enabled"
	}
	return "disabled"
}

func editableValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(empty)"
	}
	return value
}

func maskedValue(value string) string {
	if value == "" {
		return "(empty)"
	}
	return strings.Repeat("*", utf8.RuneCountInString(value))
}

func removeLastRune(value string) string {
	if value == "" {
		return ""
	}
	_, size := utf8.DecodeLastRuneInString(value)
	return value[:len(value)-size]
}

func (m Model) renderSitesScreen() string {
	if m.createState.showWizard {
		return m.renderCreateSiteWizard()
	}
	if m.editState.showEditor {
		return m.renderEditSiteWizard()
	}

	sites, err := m.siteManager.List()
	if err != nil {
		return "Sites\n\nerror: " + err.Error()
	}
	if len(sites) == 0 {
		return views.RenderSites(m.config)
	}
	if m.siteSelection >= len(sites) {
		m.siteSelection = len(sites) - 1
	}
	if m.siteSelection < 0 {
		m.siteSelection = 0
	}

	var lines []string
	lines = append(lines, "Sites", "")
	for idx, item := range sites {
		marker := " "
		if idx == m.siteSelection {
			marker = ">"
		}
		lines = append(lines, fmt.Sprintf("%s %s  backend=%s  state=%s  profile=%s  tls=%t", marker, item.Site.Name, item.Site.Backend, item.Site.State, item.Site.Profile, item.Site.TLS.Enabled))
	}

	if m.siteSelection >= len(sites) {
		m.siteSelection = 0
	}
	selected := sites[m.siteSelection]
	lines = append(lines, "", "Detail", "")
	lines = append(lines,
		fmt.Sprintf("docroot: %s", selected.Site.DocumentRoot),
		fmt.Sprintf("server_name: %s", selected.Site.Domain.ServerName),
		fmt.Sprintf("aliases: %s", csvDisplay(selected.Site.Domain.Aliases)),
		fmt.Sprintf("index_files: %s", csvDisplay(selected.Site.IndexFiles)),
		fmt.Sprintf("upstream: %s", emptyFallback(primaryUpstream(selected.Site), "(none)")),
		fmt.Sprintf("php: enabled=%t version=%s", selected.Site.PHP.Enabled, selected.Site.PHP.Version),
		fmt.Sprintf("managed assets: %d", len(selected.ManagedAssetPaths)),
	)

	report, err := m.siteManager.Diff(context.Background(), selected.Site.Name)
	if err != nil {
		lines = append(lines, "", "Diff", "", "error: "+err.Error())
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "", "Diff", "")
	if len(report.Entries) == 0 {
		lines = append(lines, "no drift detected")
	} else {
		for idx, entry := range report.Entries {
			lines = append(lines, fmt.Sprintf("%s  %s", entry.Status, entry.Path))
			if idx == 0 && entry.Preview != "" {
				lines = append(lines, entry.Preview)
			}
			if idx >= 2 {
				lines = append(lines, fmt.Sprintf("... %d more", len(report.Entries)-idx-1))
				break
			}
		}
	}
	lines = append(lines, "", "Actions", "", "m open edit site wizard", "s start/stop selected site", "r reload selected site", "x restart selected backend", "g cycle access/error logs", "[ / ] decrease/increase log lines", "ctrl+r refresh current logs panel", "t toggle TLS plan preview", "a apply displayed TLS plan", "v cycle PHP target version", "u apply selected PHP target")
	if m.siteAction.pendingAction != "" {
		lines = append(lines, fmt.Sprintf("pending: %s %s", m.siteAction.pendingAction, m.siteAction.pendingSite))
		lines = append(lines, "press enter to confirm or esc to cancel")
	}
	if m.siteAction.feedback != "" {
		lines = append(lines, "last action: "+m.siteAction.feedback)
	}
	if m.siteAction.logKind != "" {
		lines = append(lines, "", "Logs", "")
		lines = append(lines, m.renderSelectedSiteLogs(selected)...)
	}
	if m.siteAction.showTLSPlan {
		lines = append(lines, "", "TLS Plan Preview", "")
		lines = append(lines, m.renderSelectedSiteTLSPlan(selected)...)
	}
	lines = append(lines, "", "PHP Runtime", "")
	lines = append(lines, m.renderSelectedSitePHP(selected)...)
	return strings.Join(lines, "\n")
}

func (m Model) renderEditSiteWizard() string {
	lines := []string{"Sites", "", "Edit Site Wizard", ""}
	editPHPOptions := []string{"(unchanged)", "8.2", "8.3", "8.4", "disabled"}
	fields := []struct {
		label string
		value string
	}{
		{"docroot", editableValue(m.editState.docroot)},
		{"aliases", editableValue(m.editState.aliases)},
		{"index_files", editableValue(m.editState.indexFiles)},
		{"upstream", editableValue(m.editState.upstream)},
		{"php_version", editPHPOptions[m.editState.phpVersion]},
		{"tls", boolLabel(m.editState.tlsEnabled)},
	}
	for idx, field := range fields {
		marker := " "
		if idx == m.editState.selected {
			marker = ">"
		}
		suffix := ""
		if m.editState.editing && idx == m.editState.selected && idx < 4 {
			suffix = "  [editing]"
		}
		lines = append(lines, fmt.Sprintf("%s %-12s %s%s", marker, field.label, field.value, suffix))
	}
	lines = append(lines, "", "Keys: j/k select  enter/e edit text  p preview  n apply  m close")
	if m.siteAction.pendingAction != "" {
		lines = append(lines, fmt.Sprintf("pending: %s %s", m.siteAction.pendingAction, m.siteAction.pendingSite))
		lines = append(lines, "press enter to confirm or esc to cancel")
	}
	if m.siteAction.feedback != "" {
		lines = append(lines, "last action: "+m.siteAction.feedback)
	}
	if !m.editState.showPreview {
		lines = append(lines, "", "Plan preview hidden. Press `p` to render the current site:update plan.")
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "", "Plan Preview", "")
	previewLines, err := m.renderEditSitePlan()
	if err != nil {
		lines = append(lines, "error: "+err.Error())
		return strings.Join(lines, "\n")
	}
	lines = append(lines, previewLines...)
	return strings.Join(lines, "\n")
}

func (m Model) renderCreateSiteWizard() string {
	lines := []string{"Sites", "", "Create Site Wizard", ""}
	fields := []struct {
		label string
		value string
	}{
		{"server_name", editableValue(m.createState.serverName)},
		{"aliases", editableValue(m.createState.aliases)},
		{"docroot", editableValue(m.createState.docroot)},
		{"backend", createBackendOptions[m.createState.backend]},
		{"profile", createProfileOptions[m.createState.profile]},
		{"php_version", emptyFallback(createPHPOptions[m.createState.phpVersion], "default")},
		{"upstream", editableValue(m.createState.upstream)},
	}
	for idx, field := range fields {
		marker := " "
		if idx == m.createState.selected {
			marker = ">"
		}
		suffix := ""
		if m.createState.editing && idx == m.createState.selected && (idx == 0 || idx == 1 || idx == 2 || idx == 6) {
			suffix = "  [editing]"
		}
		lines = append(lines, fmt.Sprintf("%s %-12s %s%s", marker, field.label, field.value, suffix))
	}
	lines = append(lines, "", "Keys: j/k select  enter toggle/edit  e edit text  p preview  n create  c close")
	if m.siteAction.pendingAction != "" {
		lines = append(lines, fmt.Sprintf("pending: %s %s", m.siteAction.pendingAction, m.siteAction.pendingSite))
		lines = append(lines, "press enter to confirm or esc to cancel")
	}
	if m.siteAction.feedback != "" {
		lines = append(lines, "last action: "+m.siteAction.feedback)
	}
	if !m.createState.showPreview {
		lines = append(lines, "", "Plan preview hidden. Press `p` to render the current site:create plan.")
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "", "Plan Preview", "")
	previewLines, err := m.renderCreateSitePlan()
	if err != nil {
		lines = append(lines, "error: "+err.Error())
		return strings.Join(lines, "\n")
	}
	lines = append(lines, previewLines...)
	return strings.Join(lines, "\n")
}

func (m Model) renderCreateSitePlan() ([]string, error) {
	siteSpec, err := m.buildCreateSiteSpec()
	if err != nil {
		return nil, err
	}
	p, err := m.siteManager.Create(context.Background(), site.CreateOptions{
		Site:       siteSpec,
		DryRun:     true,
		SkipReload: true,
	})
	if err != nil {
		return nil, err
	}
	lines := []string{p.Summary}
	for _, warning := range p.Warnings {
		lines = append(lines, "warning: "+warning)
	}
	lines = append(lines, renderPlanOperations(p.Operations, 8, "")...)
	return lines, nil
}

func (m Model) renderEditSitePlan() ([]string, error) {
	opts, err := m.buildEditSiteOptions()
	if err != nil {
		return nil, err
	}
	p, err := m.siteManager.UpdateSettings(context.Background(), opts)
	if err != nil {
		return nil, err
	}
	lines := []string{p.Summary}
	for _, warning := range p.Warnings {
		lines = append(lines, "warning: "+warning)
	}
	lines = append(lines, renderPlanOperations(p.Operations, 8, "")...)
	return lines, nil
}

func (m *Model) stageSiteToggleAction() {
	selected, ok := m.selectedSite()
	if !ok {
		return
	}
	action := "stop"
	if selected.Site.State == "disabled" {
		action = "start"
	}
	m.siteAction.pendingAction = action
	m.siteAction.pendingSite = selected.Site.Name
	m.siteAction.feedback = ""
}

func (m *Model) toggleCreateWizard() {
	m.createState.showWizard = !m.createState.showWizard
	m.editState.showEditor = false
	m.createState.showPreview = false
	m.createState.editing = false
	m.createState.selected = 0
	m.siteAction.pendingAction = ""
	m.siteAction.pendingSite = ""
	if !m.createState.showWizard {
		m.siteAction.feedback = ""
	}
}

func (m *Model) toggleEditWizard() {
	if m.createState.showWizard {
		m.createState.showWizard = false
	}
	m.editState.showEditor = !m.editState.showEditor
	m.editState.showPreview = false
	m.editState.editing = false
	m.editState.selected = 0
	m.siteAction.pendingAction = ""
	m.siteAction.pendingSite = ""
	if !m.editState.showEditor {
		m.siteAction.feedback = ""
		return
	}
	selected, ok := m.selectedSite()
	if !ok {
		m.editState.showEditor = false
		return
	}
	m.editState.docroot = selected.Site.DocumentRoot
	m.editState.aliases = strings.Join(selected.Site.Domain.Aliases, ",")
	m.editState.indexFiles = strings.Join(selected.Site.IndexFiles, ",")
	m.editState.upstream = primaryUpstream(selected.Site)
}

func (m *Model) stageCreateSiteAction() {
	siteSpec, err := m.buildCreateSiteSpec()
	if err != nil {
		m.siteAction.feedback = err.Error()
		return
	}
	m.siteAction.pendingAction = "create"
	m.siteAction.pendingSite = siteSpec.Name
	m.siteAction.feedback = ""
}

func (m *Model) stageEditSiteAction() {
	opts, err := m.buildEditSiteOptions()
	if err != nil {
		m.siteAction.feedback = err.Error()
		return
	}
	m.siteAction.pendingAction = "update"
	m.siteAction.pendingSite = opts.Name
	m.siteAction.feedback = ""
}

func (m *Model) toggleSiteLogs() {
	if m.siteAction.logLines <= 0 {
		m.siteAction.logLines = 10
	}
	switch m.siteAction.logKind {
	case "":
		m.siteAction.logKind = "access"
		m.siteAction.feedback = fmt.Sprintf("showing access logs (%d lines)", m.siteAction.logLines)
	case "access":
		m.siteAction.logKind = "error"
		m.siteAction.feedback = fmt.Sprintf("showing error logs (%d lines)", m.siteAction.logLines)
	default:
		m.siteAction.logKind = ""
		m.siteAction.feedback = "logs panel hidden"
	}
}

func (m *Model) adjustSiteLogLines(delta int) {
	if m.siteAction.logKind == "" {
		m.siteAction.feedback = "enable logs first with `g`"
		return
	}
	if m.siteAction.logLines <= 0 {
		m.siteAction.logLines = 10
	}
	next := m.siteAction.logLines + delta
	if next < 5 {
		next = 5
	}
	if next > 200 {
		next = 200
	}
	m.siteAction.logLines = next
	m.siteAction.feedback = fmt.Sprintf("showing %s logs (%d lines)", m.siteAction.logKind, m.siteAction.logLines)
}

func (m *Model) refreshSiteLogs() {
	if m.siteAction.logKind == "" {
		m.siteAction.feedback = "enable logs first with `g`"
		return
	}
	m.siteAction.feedback = fmt.Sprintf("refreshed %s logs (%d lines)", m.siteAction.logKind, m.siteAction.logLines)
}

func (m *Model) stageSiteTLSApplyAction() {
	if !m.siteAction.showTLSPlan {
		m.siteAction.feedback = "enable TLS plan preview first with `t`"
		return
	}
	selected, ok := m.selectedSite()
	if !ok {
		return
	}
	m.siteAction.pendingAction = "tls"
	m.siteAction.pendingSite = selected.Site.Name
	m.siteAction.feedback = ""
}

func (m *Model) stageSitePHPApplyAction() {
	selected, ok := m.selectedSite()
	if !ok {
		return
	}
	if !selected.Site.PHP.Enabled {
		m.siteAction.feedback = "php is disabled for the selected site"
		return
	}
	target := m.effectiveSitePHPTarget(selected)
	if target == "" {
		m.siteAction.feedback = "no php target available"
		return
	}
	if target == selected.Site.PHP.Version {
		m.siteAction.feedback = "selected php target matches current version"
		return
	}
	m.siteAction.pendingAction = "php"
	m.siteAction.pendingSite = selected.Site.Name
	m.siteAction.feedback = ""
}

func (m *Model) stageSiteAction(action string) {
	selected, ok := m.selectedSite()
	if !ok {
		return
	}
	m.siteAction.pendingAction = action
	m.siteAction.pendingSite = selected.Site.Name
	m.siteAction.feedback = ""
}

func (m *Model) executePendingSiteAction() (tea.Model, tea.Cmd) {
	action := m.siteAction.pendingAction
	name := m.siteAction.pendingSite
	m.siteAction.pendingAction = ""
	m.siteAction.pendingSite = ""

	ctx := context.Background()
	var err error
	switch action {
	case "create":
		siteSpec, buildErr := m.buildCreateSiteSpec()
		if buildErr != nil {
			m.siteAction.feedback = buildErr.Error()
			return *m, nil
		}
		_, err = m.siteManager.Create(ctx, site.CreateOptions{
			Site:       siteSpec,
			SkipReload: true,
		})
		if err == nil {
			m.createState.showWizard = false
			m.createState.showPreview = false
			m.createState.editing = false
			m.createState.serverName = ""
			m.createState.upstream = ""
		}
	case "update":
		opts, buildErr := m.buildEditSiteOptions()
		if buildErr != nil {
			m.siteAction.feedback = buildErr.Error()
			return *m, nil
		}
		opts.DryRun = false
		opts.PlanOnly = false
		opts.SkipReload = false
		_, err = m.siteManager.UpdateSettings(ctx, opts)
		if err == nil {
			m.editState.showEditor = false
			m.editState.showPreview = false
			m.editState.editing = false
		}
	case "start":
		_, err = m.siteManager.SetState(ctx, site.StateChangeOptions{Name: name, State: "enabled"})
	case "stop":
		_, err = m.siteManager.SetState(ctx, site.StateChangeOptions{Name: name, State: "disabled"})
	case "reload":
		_, err = m.siteManager.Reload(ctx, name)
	case "restart":
		_, err = m.siteManager.Restart(ctx, name)
	case "php":
		selected, ok := m.selectedSite()
		if !ok {
			m.siteAction.feedback = "site selection lost"
			return *m, nil
		}
		_, err = m.siteManager.UpdatePHPVersion(ctx, site.UpdatePHPOptions{
			Name:    name,
			Version: m.effectiveSitePHPTarget(selected),
		})
	case "tls":
		selected, ok := m.selectedSite()
		if !ok {
			m.siteAction.feedback = "site selection lost"
			return *m, nil
		}
		mode, certFile, certKey, email := m.siteTLSIntent(selected)
		_, err = m.siteManager.UpdateTLS(ctx, site.UpdateTLSOptions{
			Name:            name,
			Mode:            mode,
			CertificateFile: certFile,
			CertificateKey:  certKey,
			Email:           email,
		})
	default:
		m.siteAction.feedback = "unknown site action"
		return *m, nil
	}
	if err != nil {
		m.siteAction.feedback = fmt.Sprintf("%s %s failed: %v", action, name, err)
		return *m, nil
	}
	m.siteAction.feedback = fmt.Sprintf("%s %s completed", action, name)
	return *m, nil
}

func (m Model) renderSelectedSiteLogs(selected coremodel.SiteManifest) []string {
	lines, err := m.siteManager.ReadLogs(site.LogReadOptions{
		Name:  selected.Site.Name,
		Kind:  m.siteAction.logKind,
		Lines: m.siteAction.logLines,
	})
	if err != nil {
		return []string{"error: " + err.Error()}
	}
	if len(lines) == 0 {
		return []string{fmt.Sprintf("%s log  lines=%d", m.siteAction.logKind, m.siteAction.logLines), fmt.Sprintf("no %s log lines available", m.siteAction.logKind)}
	}

	// Apply grep filter if set
	grepPattern := strings.TrimSpace(m.siteAction.logGrep)
	if grepPattern != "" {
		filtered := make([]string, 0, len(lines))
		for _, line := range lines {
			if strings.Contains(line, grepPattern) {
				filtered = append(filtered, line)
			}
		}
		lines = filtered
	}

	header := fmt.Sprintf("%s log  lines=%d", m.siteAction.logKind, m.siteAction.logLines)
	if m.siteAction.logFollow {
		header += "  [follow]"
	}
	if grepPattern != "" {
		header += fmt.Sprintf("  grep=%q (%d matches)", grepPattern, len(lines))
	}
	if m.siteAction.logGrepEdit {
		header += "  [editing grep]"
	}
	return append([]string{header}, lines...)
}

func (m *Model) cycleSitePHPVersion() {
	selected, ok := m.selectedSite()
	if !ok {
		return
	}
	if !selected.Site.PHP.Enabled {
		m.siteAction.feedback = "php is disabled for the selected site"
		return
	}
	options := m.availablePHPVersions()
	if len(options) == 0 {
		m.siteAction.feedback = "no php versions available"
		return
	}
	current := m.effectiveSitePHPTarget(selected)
	index := 0
	for idx, version := range options {
		if version == current {
			index = idx
			break
		}
	}
	m.siteAction.phpTarget = options[(index+1)%len(options)]
	m.siteAction.showPHPPlan = true
	m.siteAction.feedback = ""
}

func (m Model) activeCreateSiteText() string {
	switch m.createState.selected {
	case 0:
		return m.createState.serverName
	case 1:
		return m.createState.aliases
	case 2:
		return m.createState.docroot
	case 6:
		return m.createState.upstream
	default:
		return ""
	}
}

func (m *Model) updateCreateSiteTextField(value string) {
	switch m.createState.selected {
	case 0:
		m.createState.serverName = value
	case 1:
		m.createState.aliases = value
	case 2:
		m.createState.docroot = value
	case 6:
		m.createState.upstream = value
	}
}

func (m Model) activeEditSiteText() string {
	switch m.editState.selected {
	case 0:
		return m.editState.docroot
	case 1:
		return m.editState.aliases
	case 2:
		return m.editState.indexFiles
	case 3:
		return m.editState.upstream
	default:
		return ""
	}
}

func (m *Model) updateEditSiteTextField(value string) {
	switch m.editState.selected {
	case 0:
		m.editState.docroot = value
	case 1:
		m.editState.aliases = value
	case 2:
		m.editState.indexFiles = value
	case 3:
		m.editState.upstream = value
	}
}

func (m Model) renderSelectedSiteTLSPlan(selected coremodel.SiteManifest) []string {
	mode, certFile, certKey, email := m.siteTLSIntent(selected)
	p, err := m.siteManager.UpdateTLS(context.Background(), site.UpdateTLSOptions{
		Name:            selected.Site.Name,
		Mode:            mode,
		CertificateFile: certFile,
		CertificateKey:  certKey,
		Email:           email,
		DryRun:          true,
		SkipReload:      true,
	})
	if err != nil {
		return []string{"error: " + err.Error()}
	}

	lines := []string{fmt.Sprintf("mode=%s", mode), p.Summary}
	for _, warning := range p.Warnings {
		lines = append(lines, "warning: "+warning)
	}
	lines = append(lines, renderPlanOperations(p.Operations, 6, "")...)
	return lines
}

func (m Model) renderSelectedSitePHP(selected coremodel.SiteManifest) []string {
	if !selected.Site.PHP.Enabled {
		return []string{"php disabled for this site"}
	}
	target := m.effectiveSitePHPTarget(selected)
	lines := []string{
		fmt.Sprintf("current=%s", selected.Site.PHP.Version),
		fmt.Sprintf("target=%s", target),
	}
	if target == selected.Site.PHP.Version && !m.siteAction.showPHPPlan {
		lines = append(lines, "press `v` to cycle PHP version target")
		return lines
	}
	if !m.siteAction.showPHPPlan {
		lines = append(lines, "press `v` to cycle PHP version target and render preview")
		return lines
	}
	p, err := m.siteManager.UpdatePHPVersion(context.Background(), site.UpdatePHPOptions{
		Name:       selected.Site.Name,
		Version:    target,
		DryRun:     true,
		SkipReload: true,
	})
	if err != nil {
		return append(lines, "error: "+err.Error())
	}
	lines = append(lines, "PHP Switch Preview", p.Summary)
	for _, warning := range p.Warnings {
		lines = append(lines, "warning: "+warning)
	}
	lines = append(lines, renderPlanOperations(p.Operations, 6, "")...)
	return lines
}

func renderPlanOperations(operations []plan.Operation, limit int, prefix string) []string {
	lines := make([]string, 0, limit+1)
	if limit <= 0 {
		limit = len(operations)
	}
	for idx, op := range operations {
		lines = append(lines, prefix+formatPlanOperation(op))
		if idx >= limit-1 {
			if remaining := len(operations) - idx - 1; remaining > 0 {
				lines = append(lines, fmt.Sprintf("%s... %d more", prefix, remaining))
			}
			break
		}
	}
	return lines
}

func formatPlanOperation(op plan.Operation) string {
	line := fmt.Sprintf("- %s %s", op.Kind, op.Target)
	if len(op.Details) == 0 {
		return line
	}
	keys := make([]string, 0, len(op.Details))
	for key := range op.Details {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := op.Details[key]
		if shouldRedactPlanDetail(key) {
			value = "[redacted]"
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	return fmt.Sprintf("%s (%s)", line, strings.Join(parts, ", "))
}

func shouldRedactPlanDetail(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	return key == "sql" || strings.Contains(key, "password") || strings.Contains(key, "secret")
}

func (m Model) buildCreateSiteSpec() (coremodel.Site, error) {
	serverName := strings.TrimSpace(m.createState.serverName)
	if serverName == "" {
		return coremodel.Site{}, fmt.Errorf("server_name is required")
	}
	siteSpec := coremodel.Site{
		Name:    serverName,
		Backend: createBackendOptions[m.createState.backend],
		Domain: coremodel.DomainBinding{
			ServerName: serverName,
			Aliases:    csvValues(m.createState.aliases),
		},
	}
	if docroot := strings.TrimSpace(m.createState.docroot); docroot != "" {
		siteSpec.DocumentRoot = docroot
	}
	if version := createPHPOptions[m.createState.phpVersion]; version != "" {
		siteSpec.PHP.Enabled = true
		siteSpec.PHP.Version = version
	}
	if err := site.ApplyProfile(&siteSpec, createProfileOptions[m.createState.profile], strings.TrimSpace(m.createState.upstream)); err != nil {
		return coremodel.Site{}, err
	}
	return siteSpec, nil
}

func (m Model) buildEditSiteOptions() (site.UpdateSettingsOptions, error) {
	selected, ok := m.selectedSite()
	if !ok {
		return site.UpdateSettingsOptions{}, fmt.Errorf("site selection lost")
	}
	docroot := strings.TrimSpace(m.editState.docroot)
	if docroot == "" {
		return site.UpdateSettingsOptions{}, fmt.Errorf("docroot is required")
	}
	return site.UpdateSettingsOptions{
		Name:         selected.Site.Name,
		DocumentRoot: docroot,
		Aliases:      csvValues(m.editState.aliases),
		IndexFiles:   csvValues(m.editState.indexFiles),
		Upstream:     strings.TrimSpace(m.editState.upstream),
		DryRun:       true,
		SkipReload:   true,
	}, nil
}

func csvValues(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func csvDisplay(values []string) string {
	if len(values) == 0 {
		return "(none)"
	}
	return strings.Join(values, ",")
}

func primaryUpstream(site coremodel.Site) string {
	if len(site.ReverseProxyRules) == 0 {
		return ""
	}
	return site.ReverseProxyRules[0].Upstream
}

func (m Model) siteTLSIntent(selected coremodel.SiteManifest) (mode string, certFile string, certKey string, email string) {
	mode = "letsencrypt"
	email = m.siteAction.tlsEmail
	if selected.Site.TLS.Enabled && selected.Site.TLS.Mode == "custom" && selected.Site.TLS.CertificateFile != "" && selected.Site.TLS.CertificateKey != "" {
		mode = "custom"
		email = ""
		certFile = selected.Site.TLS.CertificateFile
		certKey = selected.Site.TLS.CertificateKey
	}
	return mode, certFile, certKey, email
}

func (m Model) effectiveSitePHPTarget(selected coremodel.SiteManifest) string {
	if m.siteAction.phpTarget != "" {
		return m.siteAction.phpTarget
	}
	return selected.Site.PHP.Version
}

func (m Model) availablePHPVersions() []string {
	runtimes, err := m.phpManager.List()
	if err == nil && len(runtimes) > 0 {
		out := make([]string, 0, len(runtimes))
		for _, runtime := range runtimes {
			if runtime.Version != "" {
				out = append(out, runtime.Version)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return append([]string{}, m.config.PHP.SupportedVersions...)
}

func (m Model) selectedSite() (coremodel.SiteManifest, bool) {
	sites, err := m.siteManager.List()
	if err != nil || len(sites) == 0 {
		return coremodel.SiteManifest{}, false
	}
	index := m.siteSelection
	if index < 0 {
		index = 0
	}
	if index >= len(sites) {
		index = len(sites) - 1
	}
	return sites[index], true
}

func (m Model) selectedHistoryEntry() (rollback.StoredEntry, bool) {
	entries, err := rollback.List(m.config.Paths.HistoryDir, 20)
	entries = m.filterHistoryEntries(entries)
	if err != nil || len(entries) == 0 {
		return rollback.StoredEntry{}, false
	}
	index := m.historyState.selected
	if index < 0 {
		index = 0
	}
	if index >= len(entries) {
		index = len(entries) - 1
	}
	return entries[index], true
}

func (m Model) latestPendingHistoryEntry() (rollback.StoredEntry, bool) {
	entry, err := rollback.LoadLatestPending(m.config.Paths.HistoryDir)
	if err != nil {
		return rollback.StoredEntry{}, false
	}
	return entry, true
}

func (m Model) historyFilterLabel() string {
	switch m.historyState.filterMode {
	case 1:
		return "pending"
	case 2:
		return "rolled-back"
	default:
		return "all"
	}
}

func (m Model) doctorFilterLabel() string {
	switch m.doctorState.filterMode {
	case 1:
		return "warn"
	case 2:
		return "pass"
	default:
		return "all"
	}
}

func (m Model) filterDoctorChecks(checks []doctor.Check) []doctor.Check {
	if len(checks) == 0 {
		return checks
	}
	filtered := make([]doctor.Check, 0, len(checks))
	for _, check := range checks {
		switch m.doctorState.filterMode {
		case 1:
			if check.Status != doctor.StatusWarn && check.Status != doctor.StatusFail {
				continue
			}
		case 2:
			if check.Status != doctor.StatusPass {
				continue
			}
		}
		filtered = append(filtered, check)
	}
	return filtered
}

func doctorCheckCounts(checks []doctor.Check) (passCount int, warnCount int, failCount int) {
	for _, check := range checks {
		switch check.Status {
		case doctor.StatusPass:
			passCount++
		case doctor.StatusWarn:
			warnCount++
		case doctor.StatusFail:
			failCount++
		}
	}
	return passCount, warnCount, failCount
}

func doctorRepairCoverage(checks []doctor.Check) (repairable []string, manual []string) {
	for _, check := range checks {
		if check.Status != doctor.StatusWarn && check.Status != doctor.StatusFail {
			continue
		}
		if check.Repairable {
			repairable = append(repairable, check.Name)
			continue
		}
		manual = append(manual, check.Name)
	}
	sort.Strings(repairable)
	sort.Strings(manual)
	return repairable, manual
}

func limitStrings(values []string, limit int) []string {
	if len(values) == 0 {
		return nil
	}
	if limit <= 0 || len(values) <= limit {
		return append([]string{}, values...)
	}
	out := append([]string{}, values[:limit]...)
	out = append(out, fmt.Sprintf("+%d more", len(values)-limit))
	return out
}

func (m Model) filterHistoryEntries(entries []rollback.StoredEntry) []rollback.StoredEntry {
	if len(entries) == 0 {
		return entries
	}
	filtered := make([]rollback.StoredEntry, 0, len(entries))
	for _, entry := range entries {
		switch m.historyState.filterMode {
		case 1:
			if entry.RolledBack {
				continue
			}
		case 2:
			if !entry.RolledBack {
				continue
			}
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func max(a, b int) int {
	if a > b {
		return a
	}

	return b
}

func (m Model) renderSSLScreen() string {
	lines := []string{"SSL Certificate Status", ""}

	manifests, err := m.siteManager.List()
	if err != nil {
		return "SSL Status\n\nerror: " + err.Error()
	}

	var sslSites []sslprovider.SiteInfo
	for _, manifest := range manifests {
		if !manifest.Site.TLS.Enabled {
			continue
		}
		sslSites = append(sslSites, sslprovider.SiteInfo{
			Name:     manifest.Site.Name,
			CertFile: manifest.Site.TLS.CertificateFile,
			Domain:   manifest.Site.Domain.ServerName,
			Docroot:  manifest.Site.DocumentRoot,
		})
	}

	if len(sslSites) == 0 {
		lines = append(lines, "No sites with TLS enabled.")
		lines = append(lines, "", "Enable TLS: llstack site:ssl <site> --letsencrypt --email admin@example.com")
		return strings.Join(lines, "\n")
	}

	lm := sslprovider.NewLifecycleManager(m.config, m.exec)
	statuses := lm.Status(sslSites)

	lines = append(lines, fmt.Sprintf("%-30s  %-10s  %-10s  %s", "SITE", "STATUS", "DAYS LEFT", "ISSUER"))
	lines = append(lines, strings.Repeat("-", 80))
	for _, s := range statuses {
		statusMark := s.Status
		if s.Status == "expiring" {
			statusMark = "⚠ expiring"
		} else if s.Status == "expired" {
			statusMark = "✗ expired"
		}
		lines = append(lines, fmt.Sprintf("%-30s  %-10s  %-10d  %s", s.Site, statusMark, s.DaysLeft, s.Issuer))
	}

	expiring := sslprovider.CertsExpiringSoon(statuses, sslprovider.ExpiryThresholdDays)
	if len(expiring) > 0 {
		lines = append(lines, "", fmt.Sprintf("⚠ %d certificate(s) expiring within %d days", len(expiring), sslprovider.ExpiryThresholdDays))
	}

	lines = append(lines, "", "Keys: q back  a renew-all-expiring")
	if m.sslFeedback != "" {
		lines = append(lines, "last action: "+m.sslFeedback)
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderCronScreen() string {
	lines := []string{"Cron Task Management", ""}

	mgr := cronpkg.NewManager("/etc/cron.d", "/etc/llstack/cron")
	jobs, err := mgr.List("")
	if err != nil {
		return "Cron Tasks\n\nerror: " + err.Error()
	}

	if len(jobs) == 0 {
		lines = append(lines, "No managed cron jobs.")
		lines = append(lines, "", "Add: llstack cron:add <site> --preset wp-cron")
		return strings.Join(lines, "\n")
	}

	lines = append(lines, fmt.Sprintf("%-10s  %-25s  %-20s  %s", "ID", "SITE", "SCHEDULE", "COMMAND"))
	lines = append(lines, strings.Repeat("-", 90))
	for _, job := range jobs {
		cmd := job.Command
		if len(cmd) > 40 {
			cmd = cmd[:37] + "..."
		}
		lines = append(lines, fmt.Sprintf("%-10s  %-25s  %-20s  %s", job.ID, job.Site, job.Schedule, cmd))
	}

	lines = append(lines, "", fmt.Sprintf("Total: %d jobs", len(jobs)))
	lines = append(lines, "", "Keys: q back  a add-hint")
	if m.cronFeedback != "" {
		lines = append(lines, m.cronFeedback)
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderSecurityScreen() string {
	lines := []string{"Security Overview", ""}

	// Firewall status
	lines = append(lines, "Firewall:")
	secMgr := securitypkg.NewManager(m.exec)
	fwInfo, err := secMgr.FirewallStatus(context.Background())
	if err != nil {
		lines = append(lines, "  state: unavailable")
	} else {
		for k, v := range fwInfo {
			lines = append(lines, fmt.Sprintf("  %-10s %s", k+":", v))
		}
	}

	// Blocked IPs
	lines = append(lines, "", "Blocked IPs:")
	blocked, err := secMgr.BlockList(context.Background())
	if err != nil || len(blocked) == 0 {
		lines = append(lines, "  none")
	} else {
		for _, ip := range blocked {
			lines = append(lines, "  "+ip)
		}
	}

	// fail2ban hint
	lines = append(lines, "", "fail2ban:")
	f2bStatus, err := secMgr.Fail2banStatus(context.Background())
	if err != nil {
		lines = append(lines, "  not installed or not running")
	} else {
		for _, line := range strings.Split(f2bStatus, "\n") {
			if strings.TrimSpace(line) != "" {
				lines = append(lines, "  "+strings.TrimSpace(line))
			}
		}
	}

	lines = append(lines, "", "Keys: q back  Use CLI for actions: security:fail2ban / security:block / firewall:open")
	if m.securityFeedback != "" {
		lines = append(lines, m.securityFeedback)
	}
	return strings.Join(lines, "\n")
}
