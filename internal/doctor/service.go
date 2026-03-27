package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/web-casa/llstack/internal/cache"
	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/core/plan"
	"github.com/web-casa/llstack/internal/db"
	"github.com/web-casa/llstack/internal/logging"
	"github.com/web-casa/llstack/internal/php"
	"github.com/web-casa/llstack/internal/rollback"
	"github.com/web-casa/llstack/internal/site"
	sslprovider "github.com/web-casa/llstack/internal/ssl"
	"github.com/web-casa/llstack/internal/system"
)

const (
	StatusPass = "pass"
	StatusWarn = "warn"
	StatusFail = "fail"
)

// Check captures a single doctor diagnostic result.
type Check struct {
	Name       string            `json:"name"`
	Status     string            `json:"status"`
	Summary    string            `json:"summary"`
	Details    map[string]string `json:"details,omitempty"`
	Repairable bool              `json:"repairable,omitempty"`
}

// Report is the machine-readable doctor output.
type Report struct {
	Status      string    `json:"status"`
	GeneratedAt time.Time `json:"generated_at"`
	Checks      []Check   `json:"checks"`
}

// RepairOptions controls repair execution.
type RepairOptions struct {
	DryRun     bool
	PlanOnly   bool
	SkipReload bool
}

// Service runs diagnostics and repairs against the LLStack-managed host state.
type Service struct {
	cfg       config.RuntimeConfig
	logger    logging.Logger
	exec      system.Executor
	siteMgr   site.Manager
	phpMgr    php.Manager
	dbMgr     db.Manager
	cacheMgr  cache.Manager
	now       func() time.Time
	osRelease string
}

// NewService constructs a doctor service.
func NewService(cfg config.RuntimeConfig, logger logging.Logger, exec system.Executor) Service {
	return Service{
		cfg:       cfg,
		logger:    logger,
		exec:      exec,
		siteMgr:   site.NewManager(cfg, logger, exec),
		phpMgr:    php.NewManager(cfg, logger, exec),
		dbMgr:     db.NewManager(cfg, logger, exec),
		cacheMgr:  cache.NewManager(cfg, logger, exec),
		now:       func() time.Time { return time.Now().UTC() },
		osRelease: "/etc/os-release",
	}
}

// Run executes the available doctor checks.
func (s Service) Run(ctx context.Context) (Report, error) {
	checks := []Check{
		s.checkOSSupport(),
		s.checkManagedDirectories(),
		s.checkManagedSites(),
		s.checkRuntimeBinaries(),
		s.checkSELinux(ctx),
		s.checkFirewalld(ctx),
		s.checkListeningPorts(ctx),
		s.checkPHPSockets(),
		s.checkPHPFPMProcessHealth(ctx),
		s.checkPHPConfigDrift(),
		s.checkManagedPathOwnership(),
		s.checkManagedPathPermissions(),
		s.checkManagedSELinuxContexts(ctx),
		s.checkDatabaseTLSState(),
		s.checkManagedDatabaseArtifacts(),
		s.checkManagedServices(ctx),
		s.checkManagedProviderPorts(ctx),
		s.checkManagedDatabaseLiveProbe(ctx),
		s.checkManagedDatabaseAuthProbe(ctx),
		s.checkManagedCacheLiveProbe(ctx),
		s.checkDatabaseConnectionSaturation(ctx),
		s.checkCacheMemorySaturation(ctx),
		s.checkSSLCertificateExpiry(),
		s.checkOLSHtaccessCompat(),
		s.checkRollbackHistory(),
	}
	return Report{
		Status:      overallStatus(checks),
		GeneratedAt: s.now(),
		Checks:      checks,
	}, nil
}

// Repair repairs the actionable issues covered by Phase 8 initial repair support.
func (s Service) Repair(ctx context.Context, opts RepairOptions) (plan.Plan, error) {
	report, err := s.Run(ctx)
	if err != nil {
		return plan.Plan{}, err
	}

	p := plan.New("repair", "Repair LLStack managed directories, services, and site assets")
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly
	p.Warnings = append(p.Warnings, summarizeRepairWarnings(report)...)

	requiredDirs := s.requiredDirs()
	missingDirs := make([]string, 0, len(requiredDirs))
	for _, dir := range requiredDirs {
		if !pathExists(dir) {
			missingDirs = append(missingDirs, dir)
			p.AddOperation(plan.Operation{
				ID:     "mkdir-" + sanitizeID(dir),
				Kind:   "mkdir",
				Target: dir,
				Details: map[string]string{
					"reason": "managed directory is missing",
					"scope":  "llstack-control-plane",
				},
			})
		}
	}

	permissionFixes := s.managedPermissionRepairs()
	for _, fix := range permissionFixes {
		p.AddOperation(plan.Operation{
			ID:     "chmod-" + sanitizeID(fix.Path),
			Kind:   "chmod",
			Target: fix.Path,
			Details: map[string]string{
				"mode":         fmt.Sprintf("%#o", fix.Mode),
				"current_mode": fmt.Sprintf("%#o", fix.CurrentMode),
				"reason":       "managed path is missing owner access bits",
				"scope":        "llstack-control-plane",
			},
		})
	}

	ownershipFixes, ownershipWarnings := s.managedOwnershipRepairs()
	p.Warnings = append(p.Warnings, ownershipWarnings...)
	for _, fix := range ownershipFixes {
		p.AddOperation(plan.Operation{
			ID:     "chown-" + sanitizeID(fix.Path),
			Kind:   "chown",
			Target: fix.Path,
			Details: map[string]string{
				"uid":         fmt.Sprintf("%d", fix.UID),
				"gid":         fmt.Sprintf("%d", fix.GID),
				"current_uid": fmt.Sprintf("%d", fix.CurrentUID),
				"current_gid": fmt.Sprintf("%d", fix.CurrentGID),
				"reason":      "managed path owner/group drifted from llstack runtime user",
				"scope":       "llstack-control-plane",
			},
		})
	}

	dbRepairTargets, dbRepairWarnings := s.managedDatabaseRepairs()
	p.Warnings = append(p.Warnings, dbRepairWarnings...)
	for _, provider := range dbRepairTargets {
		p.AddOperation(plan.Operation{
			ID:     "db-reconcile-" + sanitizeID(string(provider)),
			Kind:   "db.reconcile",
			Target: string(provider),
			Details: map[string]string{
				"reason": "managed database provider metadata or TLS config is missing",
				"scope":  "managed-db-provider",
			},
		})
	}

	phpDriftTargets := s.managedPHPConfigDriftTargets()
	for _, version := range phpDriftTargets {
		p.AddOperation(plan.Operation{
			ID:     "php-profile-rewrite-" + sanitizeID(version),
			Kind:   "php.profile.rewrite",
			Target: version,
			Details: map[string]string{
				"reason": "managed PHP profile has drifted from expected state",
				"scope":  "managed-php-runtime",
			},
		})
	}

	selinuxRestoreconPaths, selinuxWarnings := s.managedSELinuxRepairPaths(ctx)
	p.Warnings = append(p.Warnings, selinuxWarnings...)
	for _, path := range selinuxRestoreconPaths {
		p.AddOperation(plan.Operation{
			ID:     "restorecon-" + sanitizeID(path),
			Kind:   "selinux.restorecon",
			Target: path,
			Details: map[string]string{
				"reason": "doctor detected a suspicious SELinux label on a managed path",
				"scope":  "llstack-control-plane",
			},
		})
	}

	firewalldPorts, firewalldWarnings := s.missingFirewalldPorts(ctx)
	p.Warnings = append(p.Warnings, firewalldWarnings...)
	for _, port := range firewalldPorts {
		p.AddOperation(plan.Operation{
			ID:     "firewalld-add-port-" + sanitizeID(port),
			Kind:   "firewalld.add-port",
			Target: port + "/tcp",
			Details: map[string]string{
				"reason": "doctor detected a required web port missing from firewalld",
				"scope":  "network-firewall",
			},
		})
	}
	if len(firewalldPorts) > 0 {
		p.AddOperation(plan.Operation{
			ID:     "firewalld-reload",
			Kind:   "firewalld.reload",
			Target: "firewalld",
			Details: map[string]string{
				"reason": "reload firewalld after managed port updates",
				"scope":  "network-firewall",
			},
		})
	}

	type reconcileTarget struct {
		name string
	}
	var reconcileSites []reconcileTarget
	sites, err := s.siteMgr.List()
	if err == nil {
		for _, manifest := range sites {
			needsRepair := !pathExists(manifest.Site.DocumentRoot) || !pathExists(filepath.Dir(manifest.Site.Logs.AccessLog))
			if !needsRepair {
				for _, asset := range manifest.ManagedAssetPaths {
					if !pathExists(asset) {
						needsRepair = true
						break
					}
				}
			}
			if needsRepair {
				reconcileSites = append(reconcileSites, reconcileTarget{name: manifest.Site.Name})
				p.AddOperation(plan.Operation{
					ID:     "reconcile-" + sanitizeID(manifest.Site.Name),
					Kind:   "site.reconcile",
					Target: manifest.Site.Name,
					Details: map[string]string{
						"reason": "managed site assets or docroot/log paths are missing",
						"scope":  "canonical-site-assets",
					},
				})
			}
		}
	}

	serviceProbe, probeErr := s.probeManagedServices(ctx)
	restartServices := append([]string{}, serviceProbe.Restartable...)
	if probeErr == nil {
		for _, serviceName := range restartServices {
			p.AddOperation(plan.Operation{
				ID:     "start-" + sanitizeID(serviceName),
				Kind:   "service.start",
				Target: serviceName,
				Details: map[string]string{
					"reason": "doctor detected a managed service in inactive or failed state",
					"scope":  "managed-service",
				},
			})
		}
	}

	if len(missingDirs) == 0 && len(permissionFixes) == 0 && len(ownershipFixes) == 0 && len(dbRepairTargets) == 0 && len(selinuxRestoreconPaths) == 0 && len(firewalldPorts) == 0 && len(reconcileSites) == 0 && len(restartServices) == 0 && report.Status == StatusPass {
		p.Summary = "Repair LLStack managed directories, services, and site assets (nothing to do)"
		return p, nil
	}

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	for _, fix := range permissionFixes {
		if err := os.Chmod(fix.Path, fix.Mode); err != nil {
			return p, err
		}
	}
	for _, fix := range ownershipFixes {
		if err := os.Chown(fix.Path, fix.UID, fix.GID); err != nil {
			return p, err
		}
	}
	for _, dir := range missingDirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return p, err
		}
	}
	for _, provider := range dbRepairTargets {
		if _, err := s.dbMgr.Reconcile(ctx, db.ReconcileOptions{Provider: provider}); err != nil {
			return p, err
		}
	}
	for _, version := range phpDriftTargets {
		runtimes, _ := s.phpMgr.List()
		for _, rt := range runtimes {
			if rt.Version != version || strings.TrimSpace(rt.ProfilePath) == "" {
				continue
			}
			expected, err := php.RenderProfile(rt.Profile)
			if err != nil {
				break
			}
			if err := os.WriteFile(rt.ProfilePath, []byte(expected), 0o644); err != nil {
				return p, fmt.Errorf("rewrite PHP profile %s: %w", rt.ProfilePath, err)
			}
			break
		}
	}
	for _, path := range selinuxRestoreconPaths {
		result, err := s.exec.Run(ctx, system.Command{Name: "restorecon", Args: []string{"-Rv", path}})
		if err != nil {
			return p, fmt.Errorf("restorecon %s: %w (%s)", path, err, result.Stderr)
		}
		if result.ExitCode != 0 {
			return p, fmt.Errorf("restorecon %s exited with %d: %s", path, result.ExitCode, result.Stderr)
		}
	}
	for _, port := range firewalldPorts {
		result, err := s.exec.Run(ctx, system.Command{Name: "firewall-cmd", Args: []string{"--permanent", "--add-port=" + port + "/tcp"}})
		if err != nil {
			return p, fmt.Errorf("firewall-cmd add-port %s/tcp: %w (%s)", port, err, result.Stderr)
		}
		if result.ExitCode != 0 {
			return p, fmt.Errorf("firewall-cmd add-port %s/tcp exited with %d: %s", port, result.ExitCode, result.Stderr)
		}
	}
	if len(firewalldPorts) > 0 {
		result, err := s.exec.Run(ctx, system.Command{Name: "firewall-cmd", Args: []string{"--reload"}})
		if err != nil {
			return p, fmt.Errorf("firewall-cmd --reload: %w (%s)", err, result.Stderr)
		}
		if result.ExitCode != 0 {
			return p, fmt.Errorf("firewall-cmd --reload exited with %d: %s", result.ExitCode, result.Stderr)
		}
	}
	for _, target := range reconcileSites {
		if _, err := s.siteMgr.Reconcile(ctx, site.ReconcileOptions{Name: target.name, SkipReload: opts.SkipReload}); err != nil {
			return p, err
		}
	}
	for _, serviceName := range restartServices {
		if err := s.runSystemctl(ctx, "start", serviceName); err != nil {
			return p, err
		}
	}

	// Auto-compile OLS .htaccess incompatibilities
	olsHtaccessRepaired := 0
	manifests, _ := s.siteMgr.List()
	for _, manifest := range manifests {
		if manifest.Site.Backend != "ols" {
			continue
		}
		htPath := filepath.Join(manifest.Site.DocumentRoot, ".htaccess")
		raw, err := os.ReadFile(htPath)
		if err != nil {
			continue
		}
		hasIssue := false
		for _, line := range strings.Split(string(raw), "\n") {
			lower := strings.ToLower(strings.TrimSpace(line))
			if strings.HasPrefix(lower, "php_value") || strings.HasPrefix(lower, "php_flag") ||
				strings.HasPrefix(lower, "php_admin_value") || strings.HasPrefix(lower, "php_admin_flag") {
				hasIssue = true
				break
			}
		}
		if hasIssue {
			// Import ols package would create circular dependency, so inline the conversion
			var userIniLines []string
			lines := strings.Split(string(raw), "\n")
			for i, line := range lines {
				trimmed := strings.TrimSpace(line)
				lower := strings.ToLower(trimmed)
				if strings.HasPrefix(lower, "php_value") || strings.HasPrefix(lower, "php_flag") ||
					strings.HasPrefix(lower, "php_admin_value") || strings.HasPrefix(lower, "php_admin_flag") {
					parts := strings.Fields(trimmed)
					if len(parts) >= 3 {
						userIniLines = append(userIniLines, fmt.Sprintf("%s = %s", parts[1], strings.Join(parts[2:], " ")))
					}
					lines[i] = "# Converted by LLStack repair: " + trimmed
				}
			}
			if len(userIniLines) > 0 {
				userIniPath := filepath.Join(manifest.Site.DocumentRoot, ".user.ini")
				existing, _ := os.ReadFile(userIniPath)
				content := string(existing)
				if !strings.Contains(content, "; Converted by LLStack") {
					content += "\n; Converted by LLStack repair\n"
				}
				for _, line := range userIniLines {
					content += line + "\n"
				}
				if err := os.WriteFile(userIniPath, []byte(content), 0o644); err != nil {
					return p, fmt.Errorf("write .user.ini for %s: %w", manifest.Site.Name, err)
				}
				if err := os.WriteFile(htPath, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
					return p, fmt.Errorf("write .htaccess for %s: %w", manifest.Site.Name, err)
				}
				olsHtaccessRepaired++
			}
		}
	}

	s.logger.Info("repair completed", "missing_dirs", len(missingDirs), "permission_repairs", len(permissionFixes), "ownership_repairs", len(ownershipFixes), "db_repairs", len(dbRepairTargets), "selinux_repairs", len(selinuxRestoreconPaths), "firewalld_repairs", len(firewalldPorts), "restarted_services", len(restartServices), "reconciled_sites", len(reconcileSites), "ols_htaccess_repaired", olsHtaccessRepaired)
	return p, nil
}

func (s Service) checkOSSupport() Check {
	info, err := parseOSRelease(s.osRelease)
	if err != nil {
		return Check{
			Name:    "os_support",
			Status:  StatusWarn,
			Summary: "unable to inspect os-release",
			Details: map[string]string{
				"path":  s.osRelease,
				"error": err.Error(),
				"goos":  runtime.GOOS,
				"arch":  runtime.GOARCH,
			},
		}
	}
	status := StatusWarn
	summary := fmt.Sprintf("%s %s is outside the primary EL9/EL10 support target", info["ID"], info["VERSION_ID"])
	if info["ID"] == "rhel" || info["ID"] == "rocky" || info["ID"] == "almalinux" {
		if strings.HasPrefix(info["VERSION_ID"], "9") || strings.HasPrefix(info["VERSION_ID"], "10") {
			status = StatusPass
			summary = fmt.Sprintf("%s %s matches the primary support target", info["ID"], info["VERSION_ID"])
		}
	}
	return Check{
		Name:    "os_support",
		Status:  status,
		Summary: summary,
		Details: info,
	}
}

func (s Service) checkManagedDirectories() Check {
	missing := make([]string, 0)
	for _, dir := range s.requiredDirs() {
		if !pathExists(dir) {
			missing = append(missing, dir)
		}
	}
	if len(missing) == 0 {
		return Check{
			Name:    "managed_directories",
			Status:  StatusPass,
			Summary: "all managed directories exist",
			Details: map[string]string{"count": fmt.Sprintf("%d", len(s.requiredDirs()))},
		}
	}
	return Check{
		Name:       "managed_directories",
		Status:     StatusWarn,
		Summary:    fmt.Sprintf("%d managed directories are missing", len(missing)),
		Repairable: true,
		Details: map[string]string{
			"missing": strings.Join(missing, ","),
		},
	}
}

func (s Service) checkManagedSites() Check {
	manifests, err := s.siteMgr.List()
	if err != nil {
		return Check{
			Name:    "managed_sites",
			Status:  StatusWarn,
			Summary: "unable to enumerate managed sites",
			Details: map[string]string{"error": err.Error()},
		}
	}
	if len(manifests) == 0 {
		return Check{
			Name:    "managed_sites",
			Status:  StatusPass,
			Summary: "no managed sites yet",
			Details: map[string]string{"count": "0"},
		}
	}

	missingAssets := 0
	missingDocroots := 0
	backendSet := map[string]struct{}{}
	for _, manifest := range manifests {
		backendSet[manifest.Site.Backend] = struct{}{}
		if !pathExists(manifest.Site.DocumentRoot) {
			missingDocroots++
		}
		for _, asset := range manifest.ManagedAssetPaths {
			if !pathExists(asset) {
				missingAssets++
			}
		}
	}
	backends := make([]string, 0, len(backendSet))
	for backend := range backendSet {
		backends = append(backends, backend)
	}
	sort.Strings(backends)
	if missingAssets == 0 && missingDocroots == 0 {
		return Check{
			Name:    "managed_sites",
			Status:  StatusPass,
			Summary: fmt.Sprintf("%d managed sites look healthy", len(manifests)),
			Details: map[string]string{
				"count":    fmt.Sprintf("%d", len(manifests)),
				"backends": strings.Join(backends, ","),
			},
		}
	}
	return Check{
		Name:       "managed_sites",
		Status:     StatusWarn,
		Summary:    "some managed site assets are missing",
		Repairable: true,
		Details: map[string]string{
			"count":            fmt.Sprintf("%d", len(manifests)),
			"backends":         strings.Join(backends, ","),
			"missing_assets":   fmt.Sprintf("%d", missingAssets),
			"missing_docroots": fmt.Sprintf("%d", missingDocroots),
		},
	}
}

func (s Service) checkRuntimeBinaries() Check {
	missing := make([]string, 0)
	for name, command := range map[string][]string{
		"apache_configtest": s.cfg.Apache.ConfigTestCmd,
		"apache_reload":     s.cfg.Apache.ReloadCmd,
		"ols_configtest":    s.cfg.OLS.ConfigTestCmd,
		"lsws_configtest":   s.cfg.LSWS.ConfigTestCmd,
	} {
		if !commandAvailable(command) {
			missing = append(missing, name)
		}
	}
	certbotInfo := sslprovider.DetectCertbot(s.cfg)
	if !certbotInfo.Found {
		missing = append(missing, "certbot")
	}
	runtimes, _ := s.phpMgr.List()
	status := StatusPass
	summary := "core runtime commands were detected"
	if len(missing) > 0 {
		status = StatusWarn
		summary = "some runtime commands are missing from the current host"
	}
	details := map[string]string{
		"missing":        strings.Join(missing, ","),
		"php_runtimes":   fmt.Sprintf("%d", len(runtimes)),
		"certbot_binary": certbotInfo.Binary,
		"certbot_source": certbotInfo.Source,
	}
	return Check{
		Name:    "runtime_binaries",
		Status:  status,
		Summary: summary,
		Details: details,
	}
}

func (s Service) checkSELinux(ctx context.Context) Check {
	result, err := s.exec.Run(ctx, system.Command{Name: "getenforce"})
	if err != nil || result.ExitCode != 0 {
		return Check{
			Name:    "selinux_state",
			Status:  StatusWarn,
			Summary: "unable to determine SELinux mode",
			Details: map[string]string{
				"error":     errorString(err),
				"stderr":    strings.TrimSpace(result.Stderr),
				"exit_code": fmt.Sprintf("%d", result.ExitCode),
			},
		}
	}
	mode := strings.TrimSpace(result.Stdout)
	status := StatusPass
	summary := "SELinux mode is " + mode
	if strings.EqualFold(mode, "Enforcing") {
		status = StatusWarn
		summary = "SELinux is enforcing; ensure contexts and booleans are configured"
	}
	return Check{
		Name:    "selinux_state",
		Status:  status,
		Summary: summary,
		Details: map[string]string{"mode": mode},
	}
}

func (s Service) checkFirewalld(ctx context.Context) Check {
	result, err := s.exec.Run(ctx, system.Command{Name: "firewall-cmd", Args: []string{"--state"}})
	if err != nil || result.ExitCode != 0 {
		return Check{
			Name:    "firewalld_state",
			Status:  StatusWarn,
			Summary: "unable to determine firewalld state",
			Details: map[string]string{
				"error":     errorString(err),
				"stderr":    strings.TrimSpace(result.Stderr),
				"exit_code": fmt.Sprintf("%d", result.ExitCode),
			},
		}
	}
	state := strings.TrimSpace(result.Stdout)
	status := StatusPass
	summary := "firewalld state is " + state
	details := map[string]string{"state": state}
	if state != "running" {
		return Check{
			Name:    "firewalld_state",
			Status:  StatusWarn,
			Summary: "firewalld is not running",
			Details: details,
		}
	}

	required := s.requiredFirewallPorts()
	portResult, portErr := s.exec.Run(ctx, system.Command{Name: "firewall-cmd", Args: []string{"--list-ports"}})
	if portErr != nil || portResult.ExitCode != 0 {
		details["required_ports"] = strings.Join(required, ",")
		details["error"] = errorString(portErr)
		details["stderr"] = strings.TrimSpace(portResult.Stderr)
		details["exit_code"] = fmt.Sprintf("%d", portResult.ExitCode)
		return Check{
			Name:    "firewalld_state",
			Status:  StatusWarn,
			Summary: "firewalld is running but port list could not be inspected",
			Details: details,
		}
	}
	openPorts := strings.Fields(strings.TrimSpace(portResult.Stdout))
	details["open_ports"] = strings.Join(openPorts, ",")
	details["required_ports"] = strings.Join(required, ",")
	missing := missingPorts(required, openPorts)
	if len(missing) > 0 {
		details["missing_ports"] = strings.Join(missing, ",")
		status = StatusWarn
		summary = "firewalld is running but required web ports are missing"
	}
	return Check{
		Name:       "firewalld_state",
		Status:     status,
		Summary:    summary,
		Repairable: len(missing) > 0,
		Details:    details,
	}
}

func (s Service) checkListeningPorts(ctx context.Context) Check {
	result, err := s.exec.Run(ctx, system.Command{Name: "ss", Args: []string{"-ltn"}})
	if err != nil || result.ExitCode != 0 {
		return Check{
			Name:    "listening_ports",
			Status:  StatusWarn,
			Summary: "unable to inspect listening TCP ports",
			Details: map[string]string{
				"error":     errorString(err),
				"stderr":    strings.TrimSpace(result.Stderr),
				"exit_code": fmt.Sprintf("%d", result.ExitCode),
			},
		}
	}
	openPorts := parseListeningPorts(result.Stdout)
	required := s.requiredFirewallPorts()
	missing := missingPorts(required, openPorts)
	details := map[string]string{
		"open_ports":     strings.Join(openPorts, ","),
		"required_ports": strings.Join(required, ","),
	}
	if len(missing) == 0 {
		return Check{
			Name:    "listening_ports",
			Status:  StatusPass,
			Summary: "required listener ports are present",
			Details: details,
		}
	}
	details["missing_ports"] = strings.Join(missing, ",")
	return Check{
		Name:    "listening_ports",
		Status:  StatusWarn,
		Summary: "some expected listener ports are not open",
		Details: details,
	}
}

func (s Service) checkPHPSockets() Check {
	manifests, err := s.siteMgr.List()
	if err != nil {
		return Check{
			Name:    "php_fpm_sockets",
			Status:  StatusWarn,
			Summary: "unable to inspect managed sites for php-fpm sockets",
			Details: map[string]string{"error": err.Error()},
		}
	}
	required := 0
	missing := make([]string, 0)
	for _, manifest := range manifests {
		if !manifest.Site.PHP.Enabled || manifest.Site.PHP.Handler != "php-fpm" {
			continue
		}
		required++
		socket := strings.TrimSpace(manifest.Site.PHP.Socket)
		if socket == "" || !pathExists(socket) {
			missing = append(missing, manifest.Site.Name)
		}
	}
	if required == 0 {
		return Check{
			Name:    "php_fpm_sockets",
			Status:  StatusPass,
			Summary: "no php-fpm sockets are required by managed sites",
			Details: map[string]string{"required": "0"},
		}
	}
	if len(missing) == 0 {
		return Check{
			Name:    "php_fpm_sockets",
			Status:  StatusPass,
			Summary: "all managed php-fpm sockets are present",
			Details: map[string]string{"required": fmt.Sprintf("%d", required)},
		}
	}
	return Check{
		Name:    "php_fpm_sockets",
		Status:  StatusWarn,
		Summary: "some managed php-fpm sockets are missing",
		Details: map[string]string{
			"required":      fmt.Sprintf("%d", required),
			"missing_sites": strings.Join(missing, ","),
		},
	}
}

func (s Service) checkPHPFPMProcessHealth(ctx context.Context) Check {
	runtimes, err := s.phpMgr.List()
	if err != nil || len(runtimes) == 0 {
		return Check{
			Name:    "php_fpm_process_health",
			Status:  StatusPass,
			Summary: "no managed PHP runtimes to check for FPM process health",
			Details: map[string]string{"runtimes": "0"},
		}
	}

	healthy := make([]string, 0)
	warned := make([]string, 0)
	unprobed := make([]string, 0)

	for _, rt := range runtimes {
		version := rt.Version
		serviceName := "php" + strings.ReplaceAll(version, ".", "") + "-php-fpm"

		// Check if service is active first
		statusResult, err := s.exec.Run(ctx, system.Command{
			Name: "systemctl",
			Args: []string{"is-active", serviceName},
		})
		if err != nil || statusResult.ExitCode != 0 {
			// Service not active, skip process check (covered by managed_services)
			continue
		}

		// Count php-fpm worker processes for this version
		result, err := s.exec.Run(ctx, system.Command{
			Name: "pgrep",
			Args: []string{"-c", "-f", "php-fpm: pool.*" + serviceName},
		})
		if isCommandNotFound(err) {
			unprobed = append(unprobed, serviceName+":pgrep-missing")
			continue
		}
		count := 0
		if err == nil {
			if v, parseErr := strconv.Atoi(strings.TrimSpace(result.Stdout)); parseErr == nil {
				count = v
			}
		}
		if count == 0 {
			// pgrep pattern might not match, try broader pattern
			result2, _ := s.exec.Run(ctx, system.Command{
				Name: "pgrep",
				Args: []string{"-c", "-f", serviceName},
			})
			if result2.ExitCode == 0 {
				if v, parseErr := strconv.Atoi(strings.TrimSpace(result2.Stdout)); parseErr == nil {
					count = v
				}
			}
		}

		if count > 0 {
			healthy = append(healthy, serviceName)
		} else {
			warned = append(warned, serviceName+":no-workers")
		}
	}

	details := map[string]string{
		"healthy_count":  fmt.Sprintf("%d", len(healthy)),
		"warned_count":   fmt.Sprintf("%d", len(warned)),
		"unprobed_count": fmt.Sprintf("%d", len(unprobed)),
	}
	if len(warned) > 0 {
		details["no_workers"] = strings.Join(warned, ",")
	}

	if len(healthy) == 0 && len(warned) == 0 && len(unprobed) == 0 {
		return Check{
			Name:    "php_fpm_process_health",
			Status:  StatusPass,
			Summary: "no active PHP-FPM services to check for process health",
			Details: map[string]string{"runtimes": "0"},
		}
	}
	if len(warned) > 0 {
		return Check{
			Name:    "php_fpm_process_health",
			Status:  StatusWarn,
			Summary: "some PHP-FPM services report active but have no worker processes",
			Details: details,
		}
	}
	return Check{
		Name:    "php_fpm_process_health",
		Status:  StatusPass,
		Summary: "all active PHP-FPM services have healthy worker processes",
		Details: details,
	}
}

func (s Service) checkPHPConfigDrift() Check {
	runtimes, err := s.phpMgr.List()
	if err != nil || len(runtimes) == 0 {
		return Check{
			Name:    "php_config_drift",
			Status:  StatusPass,
			Summary: "no managed PHP runtimes to check for config drift",
			Details: map[string]string{"runtimes": "0"},
		}
	}

	clean := make([]string, 0)
	drifted := make([]string, 0)
	missing := make([]string, 0)

	for _, rt := range runtimes {
		profilePath := strings.TrimSpace(rt.ProfilePath)
		if profilePath == "" {
			continue
		}
		diskContent, err := os.ReadFile(profilePath)
		if err != nil {
			if os.IsNotExist(err) {
				missing = append(missing, rt.Version)
			}
			continue
		}

		expected, err := php.RenderProfile(rt.Profile)
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(diskContent)) != strings.TrimSpace(expected) {
			drifted = append(drifted, rt.Version)
		} else {
			clean = append(clean, rt.Version)
		}
	}

	details := map[string]string{
		"clean_count":   fmt.Sprintf("%d", len(clean)),
		"drifted_count": fmt.Sprintf("%d", len(drifted)),
		"missing_count": fmt.Sprintf("%d", len(missing)),
	}
	if len(drifted) > 0 {
		details["drifted"] = strings.Join(drifted, ",")
	}
	if len(missing) > 0 {
		details["missing"] = strings.Join(missing, ",")
	}

	if len(drifted) == 0 && len(missing) == 0 {
		return Check{
			Name:    "php_config_drift",
			Status:  StatusPass,
			Summary: "managed PHP config profiles match expected state",
			Details: details,
		}
	}
	summary := "some managed PHP config profiles have drifted from expected state"
	if len(drifted) == 0 && len(missing) > 0 {
		summary = "some managed PHP config profile files are missing"
	}
	return Check{
		Name:       "php_config_drift",
		Status:     StatusWarn,
		Summary:    summary,
		Repairable: true,
		Details:    details,
	}
}

func (s Service) checkManagedPathOwnership() Check {
	expectedUID := os.Geteuid()
	expectedGID := os.Getegid()
	sampled := 0
	problems := make([]string, 0)

	for _, path := range s.managedOwnershipPaths() {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			continue
		}
		sampled++
		if int(stat.Uid) != expectedUID || int(stat.Gid) != expectedGID {
			problems = append(problems, fmt.Sprintf("%s=%d:%d", path, stat.Uid, stat.Gid))
		}
	}

	if sampled == 0 {
		return Check{
			Name:    "managed_path_ownership",
			Status:  StatusPass,
			Summary: "no managed paths were available for ownership checks",
			Details: map[string]string{"sampled": "0"},
		}
	}
	if len(problems) == 0 {
		return Check{
			Name:    "managed_path_ownership",
			Status:  StatusPass,
			Summary: "sampled managed paths match the current runtime owner/group",
			Details: map[string]string{
				"sampled":      fmt.Sprintf("%d", sampled),
				"expected_uid": fmt.Sprintf("%d", expectedUID),
				"expected_gid": fmt.Sprintf("%d", expectedGID),
			},
		}
	}
	return Check{
		Name:       "managed_path_ownership",
		Status:     StatusWarn,
		Summary:    "some managed paths have unexpected owner/group values",
		Repairable: os.Geteuid() == 0,
		Details: map[string]string{
			"sampled":      fmt.Sprintf("%d", sampled),
			"expected_uid": fmt.Sprintf("%d", expectedUID),
			"expected_gid": fmt.Sprintf("%d", expectedGID),
			"problems":     strings.Join(problems, ","),
		},
	}
}

func (s Service) checkManagedPathPermissions() Check {
	paths := []string{
		s.cfg.Paths.ConfigDir,
		s.cfg.Paths.StateDir,
		s.cfg.Paths.HistoryDir,
		s.cfg.Paths.BackupsDir,
		s.cfg.Paths.LogDir,
		s.cfg.Paths.SitesRootDir,
		s.cfg.Apache.ManagedVhostsDir,
	}
	modeProblems := make([]string, 0)
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		mode := info.Mode().Perm()
		if info.IsDir() {
			if mode&0o700 != 0o700 {
				modeProblems = append(modeProblems, fmt.Sprintf("%s=%#o", path, mode))
			}
			continue
		}
		if mode&0o600 != 0o600 {
			modeProblems = append(modeProblems, fmt.Sprintf("%s=%#o", path, mode))
		}
	}
	if len(modeProblems) == 0 {
		return Check{
			Name:    "managed_path_permissions",
			Status:  StatusPass,
			Summary: "sampled managed paths have owner read/write access",
			Details: map[string]string{"sampled": fmt.Sprintf("%d", len(paths))},
		}
	}
	return Check{
		Name:       "managed_path_permissions",
		Status:     StatusWarn,
		Summary:    "some managed paths have restrictive owner permissions",
		Repairable: true,
		Details:    map[string]string{"problems": strings.Join(modeProblems, ",")},
	}
}

func (s Service) checkManagedSELinuxContexts(ctx context.Context) Check {
	result, err := s.exec.Run(ctx, system.Command{Name: "getenforce"})
	if err != nil || result.ExitCode != 0 {
		return Check{
			Name:    "managed_selinux_contexts",
			Status:  StatusWarn,
			Summary: "unable to determine whether SELinux context checks should run",
			Details: map[string]string{
				"error":     errorString(err),
				"stderr":    strings.TrimSpace(result.Stderr),
				"exit_code": fmt.Sprintf("%d", result.ExitCode),
			},
		}
	}
	mode := strings.TrimSpace(strings.ToLower(result.Stdout))
	if mode == "disabled" {
		return Check{
			Name:    "managed_selinux_contexts",
			Status:  StatusPass,
			Summary: "SELinux is disabled; managed context checks were skipped",
			Details: map[string]string{"mode": mode},
		}
	}

	sampled := 0
	problems := make([]string, 0)
	unprobed := make([]string, 0)
	for _, path := range s.managedSELinuxPaths() {
		if !pathExists(path) {
			continue
		}
		sampled++
		lsResult, lsErr := s.exec.Run(ctx, system.Command{Name: "ls", Args: []string{"-Zd", path}})
		if lsErr != nil || lsResult.ExitCode != 0 {
			unprobed = append(unprobed, path)
			continue
		}
		context := parseSELinuxContext(lsResult.Stdout)
		if context == "" {
			unprobed = append(unprobed, path)
			continue
		}
		if strings.Contains(context, ":default_t:") || strings.Contains(context, ":unlabeled_t:") {
			problems = append(problems, path+"="+context)
		}
	}

	if sampled == 0 {
		return Check{
			Name:    "managed_selinux_contexts",
			Status:  StatusPass,
			Summary: "no managed paths were available for SELinux context checks",
			Details: map[string]string{"mode": mode, "sampled": "0"},
		}
	}

	details := map[string]string{
		"mode":     mode,
		"sampled":  fmt.Sprintf("%d", sampled),
		"problems": strings.Join(problems, ","),
		"unprobed": strings.Join(unprobed, ","),
	}
	if len(problems) == 0 && len(unprobed) == 0 {
		return Check{
			Name:    "managed_selinux_contexts",
			Status:  StatusPass,
			Summary: "sampled managed paths do not show obviously unsafe SELinux labels",
			Details: details,
		}
	}
	summary := "some managed paths have suspicious SELinux labels"
	if len(problems) == 0 && len(unprobed) > 0 {
		summary = "some managed paths could not be inspected for SELinux labels"
	}
	return Check{
		Name:       "managed_selinux_contexts",
		Status:     StatusWarn,
		Summary:    summary,
		Repairable: os.Geteuid() == 0 && len(problems) > 0,
		Details:    details,
	}
}

func (s Service) checkDatabaseTLSState() Check {
	manifests, err := s.dbMgr.List()
	if err != nil {
		return Check{
			Name:    "db_tls_state",
			Status:  StatusWarn,
			Summary: "unable to inspect managed database providers",
			Details: map[string]string{"error": err.Error()},
		}
	}
	if len(manifests) == 0 {
		return Check{
			Name:    "db_tls_state",
			Status:  StatusPass,
			Summary: "no managed database providers yet",
			Details: map[string]string{"providers": "0"},
		}
	}

	problems := make([]string, 0)
	enabledProviders := 0
	for _, manifest := range manifests {
		if !manifest.TLS.Enabled {
			continue
		}
		enabledProviders++
		if manifest.TLS.ServerConfigPath == "" || !pathExists(manifest.TLS.ServerConfigPath) {
			problems = append(problems, string(manifest.Provider)+":tls-config")
		}
		for _, path := range []string{manifest.TLS.ServerCAFile, manifest.TLS.ServerCertFile, manifest.TLS.ServerKeyFile} {
			if path == "" {
				continue
			}
			if !pathExists(path) {
				problems = append(problems, string(manifest.Provider)+":"+path)
			}
		}
	}
	if enabledProviders == 0 {
		return Check{
			Name:    "db_tls_state",
			Status:  StatusPass,
			Summary: "managed database TLS is not enabled on any provider",
			Details: map[string]string{"providers": fmt.Sprintf("%d", len(manifests))},
		}
	}
	if len(problems) == 0 {
		return Check{
			Name:    "db_tls_state",
			Status:  StatusPass,
			Summary: "managed database TLS assets look present",
			Details: map[string]string{
				"providers":     fmt.Sprintf("%d", len(manifests)),
				"tls_providers": fmt.Sprintf("%d", enabledProviders),
			},
		}
	}
	return Check{
		Name:    "db_tls_state",
		Status:  StatusWarn,
		Summary: "some managed database TLS assets are missing",
		Details: map[string]string{
			"providers":     fmt.Sprintf("%d", len(manifests)),
			"tls_providers": fmt.Sprintf("%d", enabledProviders),
			"problems":      strings.Join(problems, ","),
		},
	}
}

func (s Service) checkManagedDatabaseArtifacts() Check {
	manifests, err := s.dbMgr.List()
	if err != nil {
		return Check{
			Name:    "db_managed_artifacts",
			Status:  StatusWarn,
			Summary: "unable to inspect managed database provider artifacts",
			Details: map[string]string{"error": err.Error()},
		}
	}
	if len(manifests) == 0 {
		return Check{
			Name:    "db_managed_artifacts",
			Status:  StatusPass,
			Summary: "no managed database provider artifacts require reconciliation yet",
			Details: map[string]string{"providers": "0"},
		}
	}

	repairable := make([]string, 0)
	manual := make([]string, 0)
	for _, manifest := range manifests {
		if manifest.TLS.Enabled && strings.TrimSpace(manifest.TLS.ServerConfigPath) != "" && !pathExists(manifest.TLS.ServerConfigPath) {
			repairable = append(repairable, string(manifest.Provider)+":tls-config")
		}
		if manifest.AdminConnection != nil {
			connPath := filepath.Join(s.cfg.DB.ManagedConnectionsDir, string(manifest.Provider)+"-"+sanitizeConnectionName(manifest.AdminConnection.Name)+".json")
			if !pathExists(connPath) {
				repairable = append(repairable, string(manifest.Provider)+":admin-connection")
			}
			if strings.TrimSpace(manifest.AdminConnection.PasswordFile) != "" && !pathExists(manifest.AdminConnection.PasswordFile) {
				manual = append(manual, string(manifest.Provider)+":admin-credential")
			}
		}
	}

	details := map[string]string{
		"repairable": strings.Join(uniqueStrings(repairable), ","),
		"manual":     strings.Join(uniqueStrings(manual), ","),
	}
	if len(repairable) == 0 && len(manual) == 0 {
		return Check{
			Name:    "db_managed_artifacts",
			Status:  StatusPass,
			Summary: "managed database provider artifacts are present",
			Details: details,
		}
	}
	summary := "some managed database provider artifacts are missing"
	if len(repairable) == 0 && len(manual) > 0 {
		summary = "some managed database provider credentials are missing and require manual recovery"
	}
	return Check{
		Name:       "db_managed_artifacts",
		Status:     StatusWarn,
		Summary:    summary,
		Repairable: len(repairable) > 0,
		Details:    details,
	}
}

func (s Service) checkManagedServices(ctx context.Context) Check {
	probe, err := s.probeManagedServices(ctx)
	if err != nil {
		return Check{
			Name:    "managed_services",
			Status:  StatusWarn,
			Summary: "unable to probe managed services",
			Details: map[string]string{"error": err.Error()},
		}
	}
	if len(probe.Active) == 0 && len(probe.Restartable) == 0 && len(probe.Problematic) == 0 && len(probe.Unprobed) == 0 {
		return Check{
			Name:    "managed_services",
			Status:  StatusPass,
			Summary: "no managed services require active probes yet",
			Details: map[string]string{"probed": "0"},
		}
	}

	details := map[string]string{
		"active":         strings.Join(probe.Active, ","),
		"restartable":    strings.Join(probe.Restartable, ","),
		"problematic":    strings.Join(probe.Problematic, ","),
		"unprobed":       strings.Join(probe.Unprobed, ","),
		"active_count":   fmt.Sprintf("%d", len(probe.Active)),
		"probed_count":   fmt.Sprintf("%d", len(probe.Active)+len(probe.Restartable)+len(probe.Problematic)),
		"unprobed_count": fmt.Sprintf("%d", len(probe.Unprobed)),
	}
	if len(probe.Restartable) == 0 && len(probe.Problematic) == 0 && len(probe.Unprobed) == 0 {
		return Check{
			Name:    "managed_services",
			Status:  StatusPass,
			Summary: "all probed managed services are active",
			Details: details,
		}
	}

	summary := "some managed services need attention"
	if len(probe.Restartable) > 0 && len(probe.Problematic) == 0 && len(probe.Unprobed) == 0 {
		summary = "some managed services are inactive or failed"
	}
	if len(probe.Restartable) == 0 && len(probe.Problematic) == 0 && len(probe.Unprobed) > 0 {
		summary = "some managed services could not be actively probed"
	}
	return Check{
		Name:       "managed_services",
		Status:     StatusWarn,
		Summary:    summary,
		Repairable: len(probe.Restartable) > 0,
		Details:    details,
	}
}

func (s Service) checkManagedProviderPorts(ctx context.Context) Check {
	expected, err := s.expectedManagedProviderPorts()
	if err != nil {
		return Check{
			Name:    "managed_provider_ports",
			Status:  StatusWarn,
			Summary: "unable to determine expected managed provider ports",
			Details: map[string]string{"error": err.Error()},
		}
	}
	if len(expected) == 0 {
		return Check{
			Name:    "managed_provider_ports",
			Status:  StatusPass,
			Summary: "no managed database or cache provider ports require probes yet",
			Details: map[string]string{"expected": "0"},
		}
	}

	result, err := s.exec.Run(ctx, system.Command{Name: "ss", Args: []string{"-ltn"}})
	if err != nil || result.ExitCode != 0 {
		return Check{
			Name:    "managed_provider_ports",
			Status:  StatusWarn,
			Summary: "unable to inspect managed provider listener ports",
			Details: map[string]string{
				"error":     errorString(err),
				"stderr":    strings.TrimSpace(result.Stderr),
				"exit_code": fmt.Sprintf("%d", result.ExitCode),
				"expected":  strings.Join(expected.providerLabels(), ","),
			},
		}
	}

	openPorts := parseListeningPorts(result.Stdout)
	openSet := map[string]struct{}{}
	for _, port := range openPorts {
		openSet[port] = struct{}{}
	}

	missing := make([]string, 0)
	for _, item := range expected {
		if _, ok := openSet[item.Port]; !ok {
			missing = append(missing, item.Label())
		}
	}

	details := map[string]string{
		"expected":       strings.Join(expected.providerLabels(), ","),
		"expected_count": fmt.Sprintf("%d", len(expected)),
		"open_ports":     strings.Join(openPorts, ","),
	}
	if len(missing) == 0 {
		return Check{
			Name:    "managed_provider_ports",
			Status:  StatusPass,
			Summary: "managed database and cache provider ports are listening",
			Details: details,
		}
	}
	details["missing"] = strings.Join(missing, ",")
	return Check{
		Name:    "managed_provider_ports",
		Status:  StatusWarn,
		Summary: "some managed database or cache provider ports are not listening",
		Details: details,
	}
}

func (s Service) checkManagedDatabaseLiveProbe(ctx context.Context) Check {
	manifests, err := s.dbMgr.List()
	if err != nil {
		return Check{
			Name:    "managed_db_live_probe",
			Status:  StatusWarn,
			Summary: "unable to inspect managed database providers for live probes",
			Details: map[string]string{"error": err.Error()},
		}
	}
	if len(manifests) == 0 {
		return Check{
			Name:    "managed_db_live_probe",
			Status:  StatusPass,
			Summary: "no managed database providers require live probes yet",
			Details: map[string]string{"providers": "0"},
		}
	}

	passed := make([]string, 0)
	failed := make([]string, 0)
	unprobed := make([]string, 0)

	for _, manifest := range manifests {
		if !statusRequiresActiveProbe(manifest.Status) {
			continue
		}
		spec, err := db.ResolveProvider(s.cfg, manifest.Provider, manifest.Version)
		if err != nil {
			failed = append(failed, string(manifest.Provider)+":spec")
			continue
		}
		cmd, label, ok := buildDatabaseLiveProbeCommand(manifest, spec)
		if !ok {
			unprobed = append(unprobed, string(manifest.Provider)+":unsupported")
			continue
		}
		result, err := s.exec.Run(ctx, cmd)
		if isCommandNotFound(err) {
			unprobed = append(unprobed, label+":binary-missing")
			continue
		}
		if err == nil && result.ExitCode == 0 {
			passed = append(passed, label)
			continue
		}
		reason := fmt.Sprintf("%s:exit=%d", label, result.ExitCode)
		if stderr := strings.TrimSpace(result.Stderr); stderr != "" {
			reason += ":" + stderr
		}
		failed = append(failed, reason)
	}

	details := map[string]string{
		"passed":         strings.Join(passed, ","),
		"failed":         strings.Join(failed, ","),
		"unprobed":       strings.Join(unprobed, ","),
		"passed_count":   fmt.Sprintf("%d", len(passed)),
		"failed_count":   fmt.Sprintf("%d", len(failed)),
		"unprobed_count": fmt.Sprintf("%d", len(unprobed)),
	}
	if len(passed) == 0 && len(failed) == 0 && len(unprobed) == 0 {
		return Check{
			Name:    "managed_db_live_probe",
			Status:  StatusPass,
			Summary: "no managed database providers require live probes yet",
			Details: map[string]string{"providers": "0"},
		}
	}
	if len(failed) == 0 && len(unprobed) == 0 {
		return Check{
			Name:    "managed_db_live_probe",
			Status:  StatusPass,
			Summary: "managed database providers responded to live probes",
			Details: details,
		}
	}
	summary := "some managed database live probes failed"
	if len(failed) == 0 && len(unprobed) > 0 {
		summary = "some managed database providers could not be actively probed"
	}
	return Check{
		Name:    "managed_db_live_probe",
		Status:  StatusWarn,
		Summary: summary,
		Details: details,
	}
}

func (s Service) checkManagedDatabaseAuthProbe(ctx context.Context) Check {
	manifests, err := s.dbMgr.List()
	if err != nil {
		return Check{
			Name:    "managed_db_auth_probe",
			Status:  StatusWarn,
			Summary: "unable to inspect managed database providers for authenticated probes",
			Details: map[string]string{"error": err.Error()},
		}
	}
	if len(manifests) == 0 {
		return Check{
			Name:    "managed_db_auth_probe",
			Status:  StatusPass,
			Summary: "no managed database providers require authenticated probes yet",
			Details: map[string]string{"providers": "0"},
		}
	}

	passed := make([]string, 0)
	failed := make([]string, 0)
	unprobed := make([]string, 0)

	for _, manifest := range manifests {
		if !statusRequiresActiveProbe(manifest.Status) {
			continue
		}
		spec, err := db.ResolveProvider(s.cfg, manifest.Provider, manifest.Version)
		if err != nil {
			failed = append(failed, string(manifest.Provider)+":spec")
			continue
		}
		if manifest.AdminConnection == nil {
			unprobed = append(unprobed, string(manifest.Provider)+":missing-admin-connection")
			continue
		}
		password, ok, err := readCredentialFile(manifest.AdminConnection.PasswordFile)
		if err != nil {
			failed = append(failed, string(manifest.Provider)+":credential-read")
			continue
		}
		if !ok {
			unprobed = append(unprobed, string(manifest.Provider)+":missing-credential")
			continue
		}

		cmd, label, ok := buildCredentialedDatabaseProbeCommand(spec, *manifest.AdminConnection, password)
		if !ok {
			unprobed = append(unprobed, string(manifest.Provider)+":unsupported")
			continue
		}
		result, err := s.exec.Run(ctx, cmd)
		if isCommandNotFound(err) {
			unprobed = append(unprobed, label+":binary-missing")
			continue
		}
		if err == nil && result.ExitCode == 0 {
			passed = append(passed, label)
			continue
		}
		reason := fmt.Sprintf("%s:exit=%d", label, result.ExitCode)
		if stderr := strings.TrimSpace(result.Stderr); stderr != "" {
			reason += ":" + stderr
		}
		failed = append(failed, reason)
	}

	details := map[string]string{
		"passed":         strings.Join(passed, ","),
		"failed":         strings.Join(failed, ","),
		"unprobed":       strings.Join(unprobed, ","),
		"passed_count":   fmt.Sprintf("%d", len(passed)),
		"failed_count":   fmt.Sprintf("%d", len(failed)),
		"unprobed_count": fmt.Sprintf("%d", len(unprobed)),
	}
	if len(passed) == 0 && len(failed) == 0 && len(unprobed) == 0 {
		return Check{
			Name:    "managed_db_auth_probe",
			Status:  StatusPass,
			Summary: "no managed database providers require authenticated probes yet",
			Details: map[string]string{"providers": "0"},
		}
	}
	if len(failed) == 0 && len(unprobed) == 0 {
		return Check{
			Name:    "managed_db_auth_probe",
			Status:  StatusPass,
			Summary: "managed database providers responded to authenticated probes",
			Details: details,
		}
	}
	summary := "some managed database authenticated probes failed"
	if len(failed) == 0 && len(unprobed) > 0 {
		summary = "some managed database providers could not be authenticatedly probed"
	}
	return Check{
		Name:    "managed_db_auth_probe",
		Status:  StatusWarn,
		Summary: summary,
		Details: details,
	}
}

func (s Service) checkManagedCacheLiveProbe(ctx context.Context) Check {
	manifests, err := s.cacheMgr.Status()
	if err != nil {
		return Check{
			Name:    "managed_cache_live_probe",
			Status:  StatusWarn,
			Summary: "unable to inspect managed cache providers for live probes",
			Details: map[string]string{"error": err.Error()},
		}
	}
	if len(manifests) == 0 {
		return Check{
			Name:    "managed_cache_live_probe",
			Status:  StatusPass,
			Summary: "no managed cache providers require live probes yet",
			Details: map[string]string{"providers": "0"},
		}
	}

	passed := make([]string, 0)
	failed := make([]string, 0)
	unprobed := make([]string, 0)

	for _, manifest := range manifests {
		if !statusRequiresActiveProbe(manifest.Status) {
			continue
		}
		cmd, label, ok := buildCacheLiveProbeCommand(manifest)
		if !ok {
			unprobed = append(unprobed, string(manifest.Provider)+":unsupported")
			continue
		}
		result, err := s.exec.Run(ctx, cmd)
		if isCommandNotFound(err) {
			unprobed = append(unprobed, label+":binary-missing")
			continue
		}
		if err == nil && result.ExitCode == 0 {
			passed = append(passed, label)
			continue
		}
		reason := fmt.Sprintf("%s:exit=%d", label, result.ExitCode)
		if stderr := strings.TrimSpace(result.Stderr); stderr != "" {
			reason += ":" + stderr
		}
		if stdout := strings.TrimSpace(result.Stdout); stdout != "" {
			reason += ":" + stdout
		}
		failed = append(failed, reason)
	}

	details := map[string]string{
		"passed":         strings.Join(passed, ","),
		"failed":         strings.Join(failed, ","),
		"unprobed":       strings.Join(unprobed, ","),
		"passed_count":   fmt.Sprintf("%d", len(passed)),
		"failed_count":   fmt.Sprintf("%d", len(failed)),
		"unprobed_count": fmt.Sprintf("%d", len(unprobed)),
	}
	if len(passed) == 0 && len(failed) == 0 && len(unprobed) == 0 {
		return Check{
			Name:    "managed_cache_live_probe",
			Status:  StatusPass,
			Summary: "no managed cache providers require live probes yet",
			Details: map[string]string{"providers": "0"},
		}
	}
	if len(failed) == 0 && len(unprobed) == 0 {
		return Check{
			Name:    "managed_cache_live_probe",
			Status:  StatusPass,
			Summary: "managed cache providers responded to live probes",
			Details: details,
		}
	}
	summary := "some managed cache live probes failed"
	if len(failed) == 0 && len(unprobed) > 0 {
		summary = "some managed cache providers could not be actively probed"
	}
	return Check{
		Name:    "managed_cache_live_probe",
		Status:  StatusWarn,
		Summary: summary,
		Details: details,
	}
}

func (s Service) checkDatabaseConnectionSaturation(ctx context.Context) Check {
	manifests, err := s.dbMgr.List()
	if err != nil {
		return Check{
			Name:    "db_connection_saturation",
			Status:  StatusWarn,
			Summary: "unable to inspect managed database providers for connection saturation",
			Details: map[string]string{"error": err.Error()},
		}
	}
	if len(manifests) == 0 {
		return Check{
			Name:    "db_connection_saturation",
			Status:  StatusPass,
			Summary: "no managed database providers to check for connection saturation",
			Details: map[string]string{"providers": "0"},
		}
	}

	passed := make([]string, 0)
	warned := make([]string, 0)
	unprobed := make([]string, 0)
	details := map[string]string{}

	for _, manifest := range manifests {
		if !statusRequiresActiveProbe(manifest.Status) {
			continue
		}
		spec, err := db.ResolveProvider(s.cfg, manifest.Provider, manifest.Version)
		if err != nil {
			unprobed = append(unprobed, string(manifest.Provider)+":spec")
			continue
		}
		if manifest.AdminConnection == nil {
			unprobed = append(unprobed, string(manifest.Provider)+":missing-admin-connection")
			continue
		}
		password, ok, err := readCredentialFile(manifest.AdminConnection.PasswordFile)
		if err != nil || !ok {
			unprobed = append(unprobed, string(manifest.Provider)+":missing-credential")
			continue
		}
		cmd, label, ok := buildConnectionSaturationCommand(spec, *manifest.AdminConnection, password)
		if !ok {
			unprobed = append(unprobed, string(manifest.Provider)+":unsupported")
			continue
		}
		result, err := s.exec.Run(ctx, cmd)
		if isCommandNotFound(err) {
			unprobed = append(unprobed, label+":binary-missing")
			continue
		}
		if err != nil || result.ExitCode != 0 {
			unprobed = append(unprobed, label+":query-failed")
			continue
		}
		pct, ok := parseConnectionSaturation(spec.Family, result.Stdout)
		if !ok {
			unprobed = append(unprobed, label+":parse-failed")
			continue
		}
		details[label] = fmt.Sprintf("%d%%", pct)
		if pct >= 80 {
			warned = append(warned, fmt.Sprintf("%s:%d%%", label, pct))
		} else {
			passed = append(passed, label)
		}
	}

	details["passed_count"] = fmt.Sprintf("%d", len(passed))
	details["warned_count"] = fmt.Sprintf("%d", len(warned))
	details["unprobed_count"] = fmt.Sprintf("%d", len(unprobed))
	if len(warned) > 0 {
		details["saturated"] = strings.Join(warned, ",")
	}

	if len(passed) == 0 && len(warned) == 0 && len(unprobed) == 0 {
		return Check{
			Name:    "db_connection_saturation",
			Status:  StatusPass,
			Summary: "no managed database providers to check for connection saturation",
			Details: map[string]string{"providers": "0"},
		}
	}
	if len(warned) > 0 {
		return Check{
			Name:    "db_connection_saturation",
			Status:  StatusWarn,
			Summary: "some managed database providers have high connection saturation (>=80%)",
			Details: details,
		}
	}
	return Check{
		Name:    "db_connection_saturation",
		Status:  StatusPass,
		Summary: "managed database connection saturation is within normal range",
		Details: details,
	}
}

func (s Service) checkCacheMemorySaturation(ctx context.Context) Check {
	manifests, err := s.cacheMgr.Status()
	if err != nil {
		return Check{
			Name:    "cache_memory_saturation",
			Status:  StatusWarn,
			Summary: "unable to inspect managed cache providers for memory saturation",
			Details: map[string]string{"error": err.Error()},
		}
	}
	if len(manifests) == 0 {
		return Check{
			Name:    "cache_memory_saturation",
			Status:  StatusPass,
			Summary: "no managed cache providers to check for memory saturation",
			Details: map[string]string{"providers": "0"},
		}
	}

	passed := make([]string, 0)
	warned := make([]string, 0)
	unprobed := make([]string, 0)
	details := map[string]string{}

	for _, manifest := range manifests {
		if !statusRequiresActiveProbe(manifest.Status) {
			continue
		}
		cmd, label, ok := buildCacheMemoryCommand(manifest)
		if !ok {
			unprobed = append(unprobed, string(manifest.Provider)+":unsupported")
			continue
		}
		result, err := s.exec.Run(ctx, cmd)
		if isCommandNotFound(err) {
			unprobed = append(unprobed, label+":binary-missing")
			continue
		}
		if err != nil || result.ExitCode != 0 {
			unprobed = append(unprobed, label+":query-failed")
			continue
		}
		pct, ok := parseCacheMemorySaturation(manifest.Provider, result.Stdout)
		if !ok {
			unprobed = append(unprobed, label+":parse-failed")
			continue
		}
		details[label] = fmt.Sprintf("%d%%", pct)
		if pct >= 80 {
			warned = append(warned, fmt.Sprintf("%s:%d%%", label, pct))
		} else {
			passed = append(passed, label)
		}
	}

	details["passed_count"] = fmt.Sprintf("%d", len(passed))
	details["warned_count"] = fmt.Sprintf("%d", len(warned))
	details["unprobed_count"] = fmt.Sprintf("%d", len(unprobed))
	if len(warned) > 0 {
		details["saturated"] = strings.Join(warned, ",")
	}

	if len(passed) == 0 && len(warned) == 0 && len(unprobed) == 0 {
		return Check{
			Name:    "cache_memory_saturation",
			Status:  StatusPass,
			Summary: "no managed cache providers to check for memory saturation",
			Details: map[string]string{"providers": "0"},
		}
	}
	if len(warned) > 0 {
		return Check{
			Name:    "cache_memory_saturation",
			Status:  StatusWarn,
			Summary: "some managed cache providers have high memory saturation (>=80%)",
			Details: details,
		}
	}
	return Check{
		Name:    "cache_memory_saturation",
		Status:  StatusPass,
		Summary: "managed cache memory saturation is within normal range",
		Details: details,
	}
}

func (s Service) checkSSLCertificateExpiry() Check {
	manifests, err := s.siteMgr.List()
	if err != nil {
		return Check{Name: "ssl_certificate_expiry", Status: StatusPass, Summary: "no managed sites to check for SSL expiry"}
	}
	var sslSites []sslprovider.SiteInfo
	for _, m := range manifests {
		if !m.Site.TLS.Enabled || strings.TrimSpace(m.Site.TLS.CertificateFile) == "" {
			continue
		}
		sslSites = append(sslSites, sslprovider.SiteInfo{
			Name:     m.Site.Name,
			CertFile: m.Site.TLS.CertificateFile,
		})
	}
	if len(sslSites) == 0 {
		return Check{Name: "ssl_certificate_expiry", Status: StatusPass, Summary: "no TLS-enabled sites to check"}
	}

	lm := sslprovider.NewLifecycleManager(s.cfg, s.exec)
	statuses := lm.Status(sslSites)
	expiring := sslprovider.CertsExpiringSoon(statuses, sslprovider.ExpiryThresholdDays)

	if len(expiring) == 0 {
		return Check{
			Name:    "ssl_certificate_expiry",
			Status:  StatusPass,
			Summary: fmt.Sprintf("all %d TLS certificates are within safe expiry range", len(statuses)),
			Details: map[string]string{"checked": fmt.Sprintf("%d", len(statuses))},
		}
	}

	names := make([]string, 0, len(expiring))
	for _, e := range expiring {
		names = append(names, fmt.Sprintf("%s(%dd)", e.Site, e.DaysLeft))
	}
	return Check{
		Name:    "ssl_certificate_expiry",
		Status:  StatusWarn,
		Summary: fmt.Sprintf("%d TLS certificate(s) expiring soon or already expired", len(expiring)),
		Details: map[string]string{
			"expiring": strings.Join(names, ","),
			"count":    fmt.Sprintf("%d", len(expiring)),
		},
	}
}

func (s Service) checkOLSHtaccessCompat() Check {
	manifests, err := s.siteMgr.List()
	if err != nil {
		return Check{
			Name:    "ols_htaccess_compat",
			Status:  StatusPass,
			Summary: "no managed sites to check for OLS .htaccess compatibility",
		}
	}

	issues := 0
	sites := make([]string, 0)
	for _, manifest := range manifests {
		if manifest.Site.Backend != "ols" {
			continue
		}
		htPath := filepath.Join(manifest.Site.DocumentRoot, ".htaccess")
		raw, err := os.ReadFile(htPath)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(line)
			lower := strings.ToLower(trimmed)
			if strings.HasPrefix(lower, "php_value") || strings.HasPrefix(lower, "php_flag") ||
				strings.HasPrefix(lower, "php_admin_value") || strings.HasPrefix(lower, "php_admin_flag") ||
				strings.HasPrefix(lower, "rewritebase") {
				issues++
			}
		}
		if issues > 0 {
			sites = append(sites, manifest.Site.Name)
		}
	}

	if issues == 0 {
		return Check{
			Name:    "ols_htaccess_compat",
			Status:  StatusPass,
			Summary: "no OLS .htaccess compatibility issues found",
		}
	}
	return Check{
		Name:       "ols_htaccess_compat",
		Status:     StatusWarn,
		Summary:    fmt.Sprintf("%d .htaccess compatibility issues found in OLS sites", issues),
		Repairable: true,
		Details: map[string]string{
			"issues":   fmt.Sprintf("%d", issues),
			"affected": strings.Join(sites, ","),
		},
	}
}

func (s Service) checkRollbackHistory() Check {
	entries, err := os.ReadDir(s.cfg.Paths.HistoryDir)
	if err != nil {
		if os.IsNotExist(err) {
			return Check{
				Name:       "rollback_history",
				Status:     StatusWarn,
				Summary:    "rollback history directory does not exist yet",
				Repairable: true,
				Details: map[string]string{
					"path": s.cfg.Paths.HistoryDir,
				},
			}
		}
		return Check{
			Name:    "rollback_history",
			Status:  StatusWarn,
			Summary: "unable to inspect rollback history",
			Details: map[string]string{"error": err.Error()},
		}
	}
	pending := 0
	if stored, err := rollback.LoadLatestPending(s.cfg.Paths.HistoryDir); err == nil && stored.Path != "" {
		pending = 1
	}
	if pending == 0 {
		return Check{
			Name:    "rollback_history",
			Status:  StatusPass,
			Summary: "rollback history is available and no pending rollback entry was found",
			Details: map[string]string{"entries": fmt.Sprintf("%d", len(entries))},
		}
	}
	return Check{
		Name:    "rollback_history",
		Status:  StatusWarn,
		Summary: "a pending rollback entry exists",
		Details: map[string]string{"entries": fmt.Sprintf("%d", len(entries)), "pending": "1"},
	}
}

type managedServiceProbe struct {
	Active      []string
	Restartable []string
	Problematic []string
	Unprobed    []string
}

type managedProviderPort struct {
	Name   string
	Kind   string
	Port   string
	Source string
}

type permissionRepair struct {
	Path        string
	Mode        os.FileMode
	CurrentMode os.FileMode
}

type ownershipRepair struct {
	Path       string
	UID        int
	GID        int
	CurrentUID int
	CurrentGID int
}

type managedProviderPorts []managedProviderPort

func (p managedProviderPorts) providerLabels() []string {
	labels := make([]string, 0, len(p))
	for _, item := range p {
		labels = append(labels, item.Label())
	}
	sort.Strings(labels)
	return labels
}

func (p managedProviderPort) Label() string {
	return p.Kind + ":" + p.Name + ":" + p.Port
}

func (s Service) probeManagedServices(ctx context.Context) (managedServiceProbe, error) {
	services, unprobed, err := s.collectManagedServices()
	if err != nil {
		return managedServiceProbe{}, err
	}

	result := managedServiceProbe{
		Unprobed: append([]string{}, unprobed...),
	}
	for _, serviceName := range services {
		state, repairable := s.inspectSystemdService(ctx, serviceName)
		switch {
		case state == "active":
			result.Active = append(result.Active, serviceName)
		case repairable:
			result.Restartable = append(result.Restartable, serviceName)
		default:
			result.Problematic = append(result.Problematic, serviceName+":"+state)
		}
	}
	sort.Strings(result.Active)
	sort.Strings(result.Restartable)
	sort.Strings(result.Problematic)
	sort.Strings(result.Unprobed)
	return result, nil
}

func (s Service) expectedManagedProviderPorts() (managedProviderPorts, error) {
	var expected managedProviderPorts

	dbProviders, err := s.dbMgr.List()
	if err != nil {
		return nil, err
	}
	for _, manifest := range dbProviders {
		if !statusRequiresActiveProbe(manifest.Status) {
			continue
		}
		port := 0
		if manifest.AdminConnection != nil && manifest.AdminConnection.Port > 0 {
			port = manifest.AdminConnection.Port
		} else {
			spec, err := db.ResolveProvider(s.cfg, manifest.Provider, manifest.Version)
			if err != nil {
				return nil, err
			}
			port = spec.Port
		}
		if port > 0 {
			expected = append(expected, managedProviderPort{
				Name:   string(manifest.Provider),
				Kind:   "db",
				Port:   fmt.Sprintf("%d", port),
				Source: manifest.ServiceName,
			})
		}
	}

	cacheProviders, err := s.cacheMgr.Status()
	if err != nil {
		return nil, err
	}
	for _, manifest := range cacheProviders {
		if !statusRequiresActiveProbe(manifest.Status) {
			continue
		}
		port := manifest.Port
		if port == 0 {
			port = defaultCachePort(manifest.Provider)
		}
		if port > 0 {
			expected = append(expected, managedProviderPort{
				Name:   string(manifest.Provider),
				Kind:   "cache",
				Port:   fmt.Sprintf("%d", port),
				Source: manifest.ServiceName,
			})
		}
	}

	sort.Slice(expected, func(i, j int) bool {
		if expected[i].Kind == expected[j].Kind {
			if expected[i].Name == expected[j].Name {
				return expected[i].Port < expected[j].Port
			}
			return expected[i].Name < expected[j].Name
		}
		return expected[i].Kind < expected[j].Kind
	})
	return expected, nil
}

func (s Service) collectManagedServices() ([]string, []string, error) {
	serviceSources := map[string]map[string]struct{}{}
	unprobed := map[string]struct{}{}

	addService := func(name string, source string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, ok := serviceSources[name]; !ok {
			serviceSources[name] = map[string]struct{}{}
		}
		serviceSources[name][source] = struct{}{}
	}
	addUnprobed := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		unprobed[name] = struct{}{}
	}

	sites, err := s.siteMgr.List()
	if err != nil {
		return nil, nil, err
	}
	for _, manifest := range sites {
		if manifest.Site.State == "disabled" {
			continue
		}
		if serviceName := s.backendServiceName(manifest.Site.Backend); serviceName != "" {
			addService(serviceName, "backend:"+manifest.Site.Backend)
		} else if strings.TrimSpace(manifest.Site.Backend) != "" {
			addUnprobed("backend:" + manifest.Site.Backend)
		}
		if manifest.Site.PHP.Enabled && manifest.Site.PHP.Handler == "php-fpm" {
			addService(manifest.Site.PHP.FPMService, "php:"+manifest.Site.Name)
		}
	}

	dbProviders, err := s.dbMgr.List()
	if err != nil {
		return nil, nil, err
	}
	for _, manifest := range dbProviders {
		if statusRequiresActiveProbe(manifest.Status) {
			addService(manifest.ServiceName, "db:"+string(manifest.Provider))
		}
	}

	cacheProviders, err := s.cacheMgr.Status()
	if err != nil {
		return nil, nil, err
	}
	for _, manifest := range cacheProviders {
		if statusRequiresActiveProbe(manifest.Status) {
			addService(manifest.ServiceName, "cache:"+string(manifest.Provider))
		}
	}

	services := make([]string, 0, len(serviceSources))
	for serviceName := range serviceSources {
		services = append(services, serviceName)
	}
	sort.Strings(services)

	unprobedList := make([]string, 0, len(unprobed))
	for item := range unprobed {
		unprobedList = append(unprobedList, item)
	}
	sort.Strings(unprobedList)
	return services, unprobedList, nil
}

func (s Service) backendServiceName(backend string) string {
	switch backend {
	case "apache":
		return inferSystemdServiceName(s.cfg.Apache.RestartCmd, s.cfg.Apache.ReloadCmd)
	case "ols":
		return inferSystemdServiceName(s.cfg.OLS.RestartCmd, s.cfg.OLS.ReloadCmd)
	case "lsws":
		return inferSystemdServiceName(s.cfg.LSWS.RestartCmd, s.cfg.LSWS.ReloadCmd)
	default:
		return ""
	}
}

func (s Service) inspectSystemdService(ctx context.Context, serviceName string) (string, bool) {
	result, err := s.exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"is-active", serviceName}})
	state := strings.TrimSpace(result.Stdout)
	if state == "" {
		state = strings.TrimSpace(result.Stderr)
	}
	state = strings.ToLower(strings.TrimSpace(state))
	if state == "" {
		if err != nil {
			state = "probe-error"
		} else {
			state = "unknown"
		}
	}
	if state == "active" {
		return state, false
	}
	if state == "inactive" || state == "failed" {
		return state, true
	}
	if strings.Contains(state, "could not be found") {
		return "not-found", false
	}
	return state, false
}

func (s Service) runSystemctl(ctx context.Context, args ...string) error {
	result, err := s.exec.Run(ctx, system.Command{Name: "systemctl", Args: args})
	if err != nil {
		return fmt.Errorf("systemctl %s: %w (%s)", strings.Join(args, " "), err, result.Stderr)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("systemctl %s exited with %d: %s", strings.Join(args, " "), result.ExitCode, result.Stderr)
	}
	return nil
}

func (s Service) requiredDirs() []string {
	return []string{
		s.cfg.Paths.ConfigDir,
		s.cfg.Paths.StateDir,
		s.cfg.Paths.HistoryDir,
		s.cfg.Paths.BackupsDir,
		s.cfg.Paths.LogDir,
		s.cfg.Paths.SitesRootDir,
		s.cfg.Paths.ManagedSitesDir(),
		s.cfg.Paths.SiteLogsDir(),
		s.cfg.Paths.ParityReportsDir(),
		s.cfg.Apache.ManagedVhostsDir,
		s.cfg.OLS.ManagedVhostsRoot,
		s.cfg.OLS.ManagedListenersDir,
		s.cfg.LSWS.ManagedIncludesDir,
		s.cfg.PHP.ManagedRuntimesDir,
		s.cfg.PHP.ManagedProfilesDir,
		s.cfg.DB.ManagedProvidersDir,
		s.cfg.DB.ManagedConnectionsDir,
		filepath.Join(s.cfg.Paths.ConfigDir, "db", "credentials"),
		s.cfg.DB.CertificatesDir,
		s.cfg.Cache.ManagedProvidersDir,
	}
}

func (s Service) managedOwnershipPaths() []string {
	return []string{
		s.cfg.Paths.ConfigDir,
		s.cfg.Paths.StateDir,
		s.cfg.Paths.HistoryDir,
		s.cfg.Paths.BackupsDir,
		s.cfg.Paths.LogDir,
		s.cfg.Paths.ManagedSitesDir(),
		s.cfg.Paths.SiteLogsDir(),
		s.cfg.Paths.ParityReportsDir(),
		s.cfg.Apache.ManagedVhostsDir,
		s.cfg.OLS.ManagedVhostsRoot,
		s.cfg.OLS.ManagedListenersDir,
		s.cfg.LSWS.ManagedIncludesDir,
		s.cfg.PHP.ManagedRuntimesDir,
		s.cfg.PHP.ManagedProfilesDir,
		s.cfg.DB.ManagedProvidersDir,
		s.cfg.DB.ManagedConnectionsDir,
		filepath.Join(s.cfg.Paths.ConfigDir, "db", "credentials"),
		s.cfg.DB.CertificatesDir,
		s.cfg.Cache.ManagedProvidersDir,
	}
}

func (s Service) managedSELinuxPaths() []string {
	return []string{
		s.cfg.Paths.ConfigDir,
		s.cfg.Paths.ManagedSitesDir(),
		s.cfg.Paths.SiteLogsDir(),
		s.cfg.Paths.SitesRootDir,
		s.cfg.Apache.ManagedVhostsDir,
		s.cfg.OLS.ManagedVhostsRoot,
		s.cfg.OLS.ManagedListenersDir,
		s.cfg.LSWS.ManagedIncludesDir,
	}
}

func (s Service) managedPermissionRepairs() []permissionRepair {
	repairs := make([]permissionRepair, 0)
	for _, path := range s.managedOwnershipPaths() {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		mode := info.Mode().Perm()
		if info.IsDir() {
			if mode&0o700 != 0o700 {
				repairs = append(repairs, permissionRepair{Path: path, Mode: mode | 0o700, CurrentMode: mode})
			}
			continue
		}
		if mode&0o600 != 0o600 {
			repairs = append(repairs, permissionRepair{Path: path, Mode: mode | 0o600, CurrentMode: mode})
		}
	}
	return repairs
}

func (s Service) managedOwnershipRepairs() ([]ownershipRepair, []string) {
	if os.Geteuid() != 0 {
		mismatches := s.managedOwnershipMismatches()
		if len(mismatches) == 0 {
			return nil, nil
		}
		return nil, []string{"ownership drift was detected but automatic chown is only enabled when llstack runs as root"}
	}

	repairs := make([]ownershipRepair, 0)
	expectedUID := os.Geteuid()
	expectedGID := os.Getegid()
	for _, path := range s.managedOwnershipPaths() {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			continue
		}
		if int(stat.Uid) != expectedUID || int(stat.Gid) != expectedGID {
			repairs = append(repairs, ownershipRepair{
				Path:       path,
				UID:        expectedUID,
				GID:        expectedGID,
				CurrentUID: int(stat.Uid),
				CurrentGID: int(stat.Gid),
			})
		}
	}
	return repairs, nil
}

func (s Service) managedPHPConfigDriftTargets() []string {
	runtimes, err := s.phpMgr.List()
	if err != nil || len(runtimes) == 0 {
		return nil
	}
	var targets []string
	for _, rt := range runtimes {
		profilePath := strings.TrimSpace(rt.ProfilePath)
		if profilePath == "" {
			continue
		}
		diskContent, err := os.ReadFile(profilePath)
		if err != nil {
			if os.IsNotExist(err) {
				targets = append(targets, rt.Version)
			}
			continue
		}
		expected, err := php.RenderProfile(rt.Profile)
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(diskContent)) != strings.TrimSpace(expected) {
			targets = append(targets, rt.Version)
		}
	}
	return targets
}

func (s Service) managedDatabaseRepairs() ([]db.ProviderName, []string) {
	manifests, err := s.dbMgr.List()
	if err != nil {
		return nil, []string{fmt.Sprintf("database artifact reconciliation is unavailable: %v", err)}
	}
	targets := make([]db.ProviderName, 0)
	warnings := make([]string, 0)
	seen := map[db.ProviderName]struct{}{}
	for _, manifest := range manifests {
		needsRepair := false
		if manifest.TLS.Enabled && strings.TrimSpace(manifest.TLS.ServerConfigPath) != "" && !pathExists(manifest.TLS.ServerConfigPath) {
			needsRepair = true
		}
		if manifest.AdminConnection != nil {
			connPath := filepath.Join(s.cfg.DB.ManagedConnectionsDir, string(manifest.Provider)+"-"+sanitizeConnectionName(manifest.AdminConnection.Name)+".json")
			if !pathExists(connPath) {
				needsRepair = true
			}
			if strings.TrimSpace(manifest.AdminConnection.PasswordFile) != "" && !pathExists(manifest.AdminConnection.PasswordFile) {
				warnings = append(warnings, fmt.Sprintf("database credential file for %s is missing and requires manual recovery", manifest.Provider))
			}
		}
		if needsRepair {
			if _, ok := seen[manifest.Provider]; !ok {
				seen[manifest.Provider] = struct{}{}
				targets = append(targets, manifest.Provider)
			}
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		return string(targets[i]) < string(targets[j])
	})
	sort.Strings(warnings)
	return targets, warnings
}

func (s Service) managedSELinuxRepairPaths(ctx context.Context) ([]string, []string) {
	if os.Geteuid() != 0 {
		for _, path := range s.managedSELinuxPaths() {
			if !pathExists(path) {
				continue
			}
		}
		report := s.checkManagedSELinuxContexts(ctx)
		if report.Status == StatusWarn && strings.TrimSpace(report.Details["problems"]) != "" {
			return nil, []string{"suspicious SELinux labels were detected but automatic restorecon is only enabled when llstack runs as root"}
		}
		return nil, nil
	}

	result, err := s.exec.Run(ctx, system.Command{Name: "getenforce"})
	if err != nil || result.ExitCode != 0 {
		return nil, nil
	}
	if strings.EqualFold(strings.TrimSpace(result.Stdout), "disabled") {
		return nil, nil
	}

	paths := make([]string, 0)
	for _, path := range s.managedSELinuxPaths() {
		if !pathExists(path) {
			continue
		}
		lsResult, lsErr := s.exec.Run(ctx, system.Command{Name: "ls", Args: []string{"-Zd", path}})
		if lsErr != nil || lsResult.ExitCode != 0 {
			continue
		}
		context := parseSELinuxContext(lsResult.Stdout)
		if strings.Contains(context, ":default_t:") || strings.Contains(context, ":unlabeled_t:") {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func (s Service) missingFirewalldPorts(ctx context.Context) ([]string, []string) {
	result, err := s.exec.Run(ctx, system.Command{Name: "firewall-cmd", Args: []string{"--state"}})
	if err != nil || result.ExitCode != 0 {
		return nil, nil
	}
	if strings.TrimSpace(result.Stdout) != "running" {
		return nil, nil
	}

	required := s.requiredFirewallPorts()
	if len(required) == 0 {
		return nil, nil
	}

	portResult, portErr := s.exec.Run(ctx, system.Command{Name: "firewall-cmd", Args: []string{"--list-ports"}})
	if portErr != nil || portResult.ExitCode != 0 {
		return nil, []string{"firewalld is running but repair could not inspect the current port list"}
	}
	openPorts := strings.Fields(strings.TrimSpace(portResult.Stdout))
	missing := missingPorts(required, openPorts)
	if len(missing) == 0 {
		return nil, nil
	}
	return missing, nil
}

func (s Service) managedOwnershipMismatches() []string {
	expectedUID := os.Geteuid()
	expectedGID := os.Getegid()
	mismatches := make([]string, 0)
	for _, path := range s.managedOwnershipPaths() {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			continue
		}
		if int(stat.Uid) != expectedUID || int(stat.Gid) != expectedGID {
			mismatches = append(mismatches, path)
		}
	}
	return mismatches
}

func overallStatus(checks []Check) string {
	status := StatusPass
	for _, check := range checks {
		switch check.Status {
		case StatusFail:
			return StatusFail
		case StatusWarn:
			status = StatusWarn
		}
	}
	return status
}

func parseOSRelease(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[key] = strings.Trim(value, `"`)
	}
	return out, nil
}

func commandAvailable(command []string) bool {
	if len(command) == 0 || command[0] == "" {
		return false
	}
	name := command[0]
	if filepath.IsAbs(name) {
		info, err := os.Stat(name)
		return err == nil && !info.IsDir()
	}
	_, err := exec.LookPath(name)
	return err == nil
}

func pathExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func sanitizeID(value string) string {
	replacer := strings.NewReplacer("/", "-", ".", "-", ":", "-", " ", "-")
	return replacer.Replace(strings.Trim(value, "-"))
}

func inferSystemdServiceName(commands ...[]string) string {
	for _, parts := range commands {
		if len(parts) >= 3 && parts[0] == "systemctl" {
			switch parts[1] {
			case "start", "restart", "reload", "enable", "is-active":
				return strings.TrimSpace(parts[len(parts)-1])
			}
		}
	}
	return ""
}

func statusRequiresActiveProbe(status string) bool {
	status = strings.TrimSpace(strings.ToLower(status))
	return status != "" && status != "planned"
}

func buildDatabaseLiveProbeCommand(manifest db.ProviderManifest, spec db.ProviderSpec) (system.Command, string, bool) {
	host := "127.0.0.1"
	port := spec.Port
	if manifest.AdminConnection != nil {
		if strings.TrimSpace(manifest.AdminConnection.Host) != "" {
			host = manifest.AdminConnection.Host
		}
		if manifest.AdminConnection.Port > 0 {
			port = manifest.AdminConnection.Port
		}
	}
	label := string(manifest.Provider) + "@" + host + ":" + strconv.Itoa(port)
	switch spec.Family {
	case "mysql":
		return system.Command{
			Name: "mysqladmin",
			Args: []string{
				"--protocol=tcp",
				"--host", host,
				"--port", strconv.Itoa(port),
				"ping",
			},
		}, label, true
	case "postgresql":
		return system.Command{
			Name: "pg_isready",
			Args: []string{
				"-h", host,
				"-p", strconv.Itoa(port),
			},
		}, label, true
	default:
		return system.Command{}, label, false
	}
}

func buildCredentialedDatabaseProbeCommand(spec db.ProviderSpec, conn db.ConnectionInfo, password string) (system.Command, string, bool) {
	host := strings.TrimSpace(conn.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	port := conn.Port
	if port <= 0 {
		port = spec.Port
	}
	user := strings.TrimSpace(conn.User)
	if user == "" {
		return system.Command{}, "", false
	}
	label := string(spec.Name) + "@" + host + ":" + strconv.Itoa(port) + "/" + user
	switch spec.Family {
	case "mysql":
		return system.Command{
			Name: "mysql",
			Args: []string{
				"--protocol=tcp",
				"--host", host,
				"--port", strconv.Itoa(port),
				"--user", user,
				"--password=" + password,
				"-e", "SELECT 1;",
			},
		}, label, true
	case "postgresql":
		databaseName := conn.Database
		if strings.TrimSpace(databaseName) == "" {
			databaseName = "postgres"
		}
		command := fmt.Sprintf("PGPASSWORD=%s psql -h %s -p %d -U %s -d %s -tAc 'SELECT 1;'", shellQuote(password), shellQuote(host), port, shellQuote(user), shellQuote(databaseName))
		return system.Command{
			Name: "sh",
			Args: []string{"-c", command},
		}, label, true
	default:
		return system.Command{}, label, false
	}
}

func buildCacheLiveProbeCommand(manifest cache.ProviderManifest) (system.Command, string, bool) {
	host := strings.TrimSpace(manifest.Bind)
	if host == "" {
		host = "127.0.0.1"
	}
	port := manifest.Port
	if port <= 0 {
		port = defaultCachePort(manifest.Provider)
	}
	label := string(manifest.Provider) + "@" + host + ":" + strconv.Itoa(port)
	switch manifest.Provider {
	case cache.ProviderRedis:
		return system.Command{
			Name: "redis-cli",
			Args: []string{"-h", host, "-p", strconv.Itoa(port), "PING"},
		}, label, true
	case cache.ProviderValkey:
		return system.Command{
			Name: "valkey-cli",
			Args: []string{"-h", host, "-p", strconv.Itoa(port), "PING"},
		}, label, true
	case cache.ProviderMemcached:
		command := fmt.Sprintf("exec 3<>/dev/tcp/%s/%d && printf 'version\\r\\n' >&3 && IFS= read -r line <&3 && printf '%%s\\n' \"$line\" | grep -q '^VERSION '", host, port)
		return system.Command{
			Name: "sh",
			Args: []string{"-c", command},
		}, label, true
	default:
		return system.Command{}, label, false
	}
}

func readCredentialFile(path string) (string, bool, error) {
	if strings.TrimSpace(path) == "" {
		return "", false, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return strings.TrimSpace(string(raw)), true, nil
}

func sanitizeConnectionName(value string) string {
	return strings.NewReplacer("@", "_", "/", "_", " ", "_").Replace(value)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func defaultCachePort(provider cache.ProviderName) int {
	switch provider {
	case cache.ProviderMemcached:
		return 11211
	case cache.ProviderRedis, cache.ProviderValkey:
		return 6379
	default:
		return 0
	}
}

func (s Service) requiredFirewallPorts() []string {
	sites, err := s.siteMgr.List()
	if err != nil || len(sites) == 0 {
		return nil
	}
	required := []string{"80"}
	for _, manifest := range sites {
		if manifest.Site.State == "disabled" {
			continue
		}
		if manifest.Site.TLS.Enabled {
			required = append(required, "443")
			break
		}
	}
	return required
}

func missingPorts(required []string, open []string) []string {
	if len(required) == 0 {
		return nil
	}
	openSet := map[string]struct{}{}
	for _, port := range open {
		openSet[strings.TrimSuffix(strings.TrimSpace(port), "/tcp")] = struct{}{}
	}
	var missing []string
	for _, port := range required {
		if _, ok := openSet[port]; !ok {
			missing = append(missing, port)
		}
	}
	return missing
}

func parseListeningPorts(raw string) []string {
	lines := strings.Split(raw, "\n")
	set := map[string]struct{}{}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		local := fields[len(fields)-2]
		port := localPort(local)
		if port == "" {
			continue
		}
		set[port] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for port := range set {
		out = append(out, port)
	}
	sort.Strings(out)
	return out
}

func parseSELinuxContext(raw string) string {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func summarizeRepairWarnings(report Report) []string {
	warnings := make([]string, 0)
	for _, check := range report.Checks {
		if check.Status != StatusWarn {
			continue
		}
		if check.Repairable {
			warnings = append(warnings, fmt.Sprintf("repair will address %s: %s", check.Name, check.Summary))
			continue
		}
		switch check.Name {
		case "managed_provider_ports", "managed_db_live_probe", "managed_db_auth_probe", "managed_selinux_contexts":
			warnings = append(warnings, fmt.Sprintf("repair will not automatically resolve %s: %s", check.Name, check.Summary))
		}
	}
	sort.Strings(warnings)
	return warnings
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func localPort(value string) string {
	if idx := strings.LastIndex(value, ":"); idx >= 0 && idx+1 < len(value) {
		return strings.Trim(value[idx+1:], "[]")
	}
	return ""
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func isCommandNotFound(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "executable file not found") || strings.Contains(message, "file not found")
}

func buildConnectionSaturationCommand(spec db.ProviderSpec, conn db.ConnectionInfo, password string) (system.Command, string, bool) {
	host := strings.TrimSpace(conn.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	port := conn.Port
	if port <= 0 {
		port = spec.Port
	}
	user := strings.TrimSpace(conn.User)
	if user == "" {
		return system.Command{}, "", false
	}
	label := string(spec.Name) + "@" + host + ":" + strconv.Itoa(port) + "/" + user
	switch spec.Family {
	case "mysql":
		return system.Command{
			Name: "mysql",
			Args: []string{
				"--protocol=tcp",
				"--host", host,
				"--port", strconv.Itoa(port),
				"--user", user,
				"--password=" + password,
				"--batch",
				"-e", "SHOW STATUS LIKE 'Threads_connected'; SHOW VARIABLES LIKE 'max_connections';",
			},
		}, label, true
	case "postgresql":
		databaseName := conn.Database
		if strings.TrimSpace(databaseName) == "" {
			databaseName = "postgres"
		}
		command := fmt.Sprintf("PGPASSWORD=%s psql -h %s -p %d -U %s -d %s -tA -F '\\t' -c \"SELECT 'Threads_connected' AS k, count(*) AS v FROM pg_stat_activity UNION ALL SELECT 'max_connections', setting FROM pg_settings WHERE name='max_connections';\"",
			shellQuote(password), shellQuote(host), port, shellQuote(user), shellQuote(databaseName))
		return system.Command{
			Name: "sh",
			Args: []string{"-c", command},
		}, label, true
	default:
		return system.Command{}, label, false
	}
}

func parseConnectionSaturation(family string, stdout string) (int, bool) {
	var connected, max int
	var foundConnected, foundMax bool
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		val := strings.TrimSpace(fields[len(fields)-1])
		switch key {
		case "Threads_connected":
			if v, err := strconv.Atoi(val); err == nil {
				connected = v
				foundConnected = true
			}
		case "max_connections":
			if v, err := strconv.Atoi(val); err == nil {
				max = v
				foundMax = true
			}
		}
	}
	if !foundConnected || !foundMax || max <= 0 {
		return 0, false
	}
	return connected * 100 / max, true
}

func buildCacheMemoryCommand(manifest cache.ProviderManifest) (system.Command, string, bool) {
	host := strings.TrimSpace(manifest.Bind)
	if host == "" {
		host = "127.0.0.1"
	}
	port := manifest.Port
	if port <= 0 {
		port = defaultCachePort(manifest.Provider)
	}
	label := string(manifest.Provider) + "@" + host + ":" + strconv.Itoa(port)
	switch manifest.Provider {
	case cache.ProviderRedis:
		return system.Command{
			Name: "redis-cli",
			Args: []string{"-h", host, "-p", strconv.Itoa(port), "INFO", "memory"},
		}, label, true
	case cache.ProviderValkey:
		return system.Command{
			Name: "valkey-cli",
			Args: []string{"-h", host, "-p", strconv.Itoa(port), "INFO", "memory"},
		}, label, true
	case cache.ProviderMemcached:
		command := fmt.Sprintf("exec 3<>/dev/tcp/%s/%d && printf 'stats\\r\\nquit\\r\\n' >&3 && cat <&3", host, port)
		return system.Command{
			Name: "sh",
			Args: []string{"-c", command},
		}, label, true
	default:
		return system.Command{}, label, false
	}
}

func parseCacheMemorySaturation(provider cache.ProviderName, stdout string) (int, bool) {
	switch provider {
	case cache.ProviderRedis, cache.ProviderValkey:
		var usedMem, maxMem int64
		for _, line := range strings.Split(stdout, "\n") {
			line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
			if strings.HasPrefix(line, "used_memory:") {
				if v, err := strconv.ParseInt(strings.TrimPrefix(line, "used_memory:"), 10, 64); err == nil {
					usedMem = v
				}
			}
			if strings.HasPrefix(line, "maxmemory:") {
				if v, err := strconv.ParseInt(strings.TrimPrefix(line, "maxmemory:"), 10, 64); err == nil {
					maxMem = v
				}
			}
		}
		if maxMem <= 0 {
			return 0, false
		}
		return int(usedMem * 100 / maxMem), true
	case cache.ProviderMemcached:
		var bytes, limitMaxbytes int64
		for _, line := range strings.Split(stdout, "\n") {
			line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
			fields := strings.Fields(line)
			if len(fields) != 3 || fields[0] != "STAT" {
				continue
			}
			switch fields[1] {
			case "bytes":
				if v, err := strconv.ParseInt(fields[2], 10, 64); err == nil {
					bytes = v
				}
			case "limit_maxbytes":
				if v, err := strconv.ParseInt(fields[2], 10, 64); err == nil {
					limitMaxbytes = v
				}
			}
		}
		if limitMaxbytes <= 0 {
			return 0, false
		}
		return int(bytes * 100 / limitMaxbytes), true
	default:
		return 0, false
	}
}
