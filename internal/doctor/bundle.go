package doctor

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/web-casa/llstack/internal/rollback"
	"github.com/web-casa/llstack/internal/system"
)

// BundleResult captures the generated diagnostics bundle metadata.
type BundleResult struct {
	Path        string    `json:"path"`
	GeneratedAt time.Time `json:"generated_at"`
	Entries     int       `json:"entries"`
}

// Bundle creates a diagnostics archive containing doctor outputs and managed metadata snapshots.
func (s Service) Bundle(ctx context.Context, outputPath string) (BundleResult, error) {
	report, err := s.Run(ctx)
	if err != nil {
		return BundleResult{}, err
	}
	if strings.TrimSpace(outputPath) == "" {
		outputPath = filepath.Join(s.cfg.Paths.StateDir, "diagnostics", "llstack-diagnostics-"+s.now().Format("20060102T150405Z")+".tar.gz")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return BundleResult{}, err
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return BundleResult{}, err
	}
	defer file.Close()

	gzw := gzip.NewWriter(file)
	defer gzw.Close()
	tw := tar.NewWriter(gzw)
	defer tw.Close()

	entries := 0
	writeJSON := func(name string, value any) error {
		raw, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		raw = append(raw, '\n')
		if err := writeTarFile(tw, name, raw, 0o644, s.now()); err != nil {
			return err
		}
		entries++
		return nil
	}

	if err := writeJSON("report.json", report); err != nil {
		return BundleResult{}, err
	}
	if err := writeJSON("summary.json", s.bundleSummary()); err != nil {
		return BundleResult{}, err
	}
	if err := writeJSON("host/runtime.json", s.bundleHostSnapshot()); err != nil {
		return BundleResult{}, err
	}
	if err := writeJSON("sites/summary.json", s.bundleSiteSnapshot()); err != nil {
		return BundleResult{}, err
	}
	if err := writeJSON("php/runtimes.json", s.bundlePHPSnapshot()); err != nil {
		return BundleResult{}, err
	}
	if err := writeJSON("db/providers.json", s.bundleDatabaseSnapshot()); err != nil {
		return BundleResult{}, err
	}
	if err := writeJSON("cache/providers.json", s.bundleCacheSnapshot()); err != nil {
		return BundleResult{}, err
	}
	if err := writeJSON("status/summary.json", s.bundleStatusSnapshot()); err != nil {
		return BundleResult{}, err
	}
	if err := writeJSON("services/managed.json", s.bundleManagedServicesSnapshot(ctx)); err != nil {
		return BundleResult{}, err
	}
	if err := writeJSON("probes/checks.json", s.bundleProbeSnapshot(report)); err != nil {
		return BundleResult{}, err
	}
	if err := s.writeCommandSnapshots(ctx, tw, "commands", &entries); err != nil {
		return BundleResult{}, err
	}
	if err := s.writeManagedServiceJournalSnapshots(ctx, tw, "journal/services", &entries); err != nil {
		return BundleResult{}, err
	}
	if err := s.writeOptionalFile(tw, "host/os-release", s.osRelease, &entries); err != nil {
		return BundleResult{}, err
	}
	if err := s.writeTree(tw, "sites", s.cfg.Paths.ManagedSitesDir(), &entries); err != nil {
		return BundleResult{}, err
	}
	if err := s.writeTree(tw, "db/providers", s.cfg.DB.ManagedProvidersDir, &entries); err != nil {
		return BundleResult{}, err
	}
	if err := s.writeTree(tw, "db/connections", s.cfg.DB.ManagedConnectionsDir, &entries); err != nil {
		return BundleResult{}, err
	}
	if err := s.writeTree(tw, "cache/providers", s.cfg.Cache.ManagedProvidersDir, &entries); err != nil {
		return BundleResult{}, err
	}
	if err := s.writeProviderConfigSnapshots(tw, "config", &entries); err != nil {
		return BundleResult{}, err
	}
	if err := s.writeTree(tw, "php/runtimes", s.cfg.PHP.ManagedRuntimesDir, &entries); err != nil {
		return BundleResult{}, err
	}
	if err := s.writeRecentHistory(tw, "history", &entries); err != nil {
		return BundleResult{}, err
	}
	if err := s.writeManagedLogTails(tw, "logs/sites", 20, &entries); err != nil {
		return BundleResult{}, err
	}
	if err := s.writeTLSCertSnapshots(ctx, tw, "tls/certs", &entries); err != nil {
		return BundleResult{}, err
	}

	return BundleResult{
		Path:        outputPath,
		GeneratedAt: s.now(),
		Entries:     entries,
	}, nil
}

func (s Service) bundleSummary() map[string]any {
	sites, _ := s.siteMgr.List()
	dbProviders, _ := s.dbMgr.List()
	cacheProviders, _ := s.cacheMgr.Status()
	phpRuntimes, _ := s.phpMgr.List()
	return map[string]any{
		"generated_at": s.now(),
		"counts": map[string]int{
			"sites":           len(sites),
			"db_providers":    len(dbProviders),
			"cache_providers": len(cacheProviders),
			"php_runtimes":    len(phpRuntimes),
		},
	}
}

func (s Service) bundleHostSnapshot() map[string]any {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = ""
	}
	return map[string]any{
		"generated_at": s.now(),
		"hostname":     hostname,
		"goos":         runtime.GOOS,
		"goarch":       runtime.GOARCH,
		"runtime_uid":  os.Geteuid(),
		"runtime_gid":  os.Getegid(),
		"paths": map[string]string{
			"config_dir":   s.cfg.Paths.ConfigDir,
			"state_dir":    s.cfg.Paths.StateDir,
			"history_dir":  s.cfg.Paths.HistoryDir,
			"backups_dir":  s.cfg.Paths.BackupsDir,
			"log_dir":      s.cfg.Paths.LogDir,
			"sites_root":   s.cfg.Paths.SitesRootDir,
			"managed_site": s.cfg.Paths.ManagedSitesDir(),
		},
	}
}

func (s Service) bundleSiteSnapshot() map[string]any {
	sites, _ := s.siteMgr.List()
	items := make([]map[string]any, 0, len(sites))
	for _, manifest := range sites {
		items = append(items, map[string]any{
			"name":        manifest.Site.Name,
			"backend":     manifest.Site.Backend,
			"state":       manifest.Site.State,
			"profile":     manifest.Site.Profile,
			"server_name": manifest.Site.Domain.ServerName,
			"docroot":     manifest.Site.DocumentRoot,
			"php_enabled": manifest.Site.PHP.Enabled,
			"php_version": manifest.Site.PHP.Version,
			"tls_enabled": manifest.Site.TLS.Enabled,
		})
	}
	return map[string]any{
		"generated_at": s.now(),
		"count":        len(items),
		"sites":        items,
	}
}

func (s Service) bundlePHPSnapshot() map[string]any {
	runtimes, _ := s.phpMgr.List()
	return map[string]any{
		"generated_at": s.now(),
		"count":        len(runtimes),
		"runtimes":     runtimes,
	}
}

func (s Service) bundleDatabaseSnapshot() map[string]any {
	providers, _ := s.dbMgr.List()
	return map[string]any{
		"generated_at": s.now(),
		"count":        len(providers),
		"providers":    providers,
	}
}

func (s Service) bundleCacheSnapshot() map[string]any {
	providers, _ := s.cacheMgr.Status()
	return map[string]any{
		"generated_at": s.now(),
		"count":        len(providers),
		"providers":    providers,
	}
}

func (s Service) bundleStatusSnapshot() map[string]any {
	sites, _ := s.siteMgr.List()
	phpRuntimes, _ := s.phpMgr.List()
	backendSet := map[string]struct{}{}
	enabledSites := 0
	disabledSites := 0
	for _, manifest := range sites {
		backendSet[manifest.Site.Backend] = struct{}{}
		if manifest.Site.State == "disabled" {
			disabledSites++
		} else {
			enabledSites++
		}
	}
	backends := make([]string, 0, len(backendSet))
	for backend := range backendSet {
		backends = append(backends, backend)
	}
	sort.Strings(backends)
	if len(backends) == 0 {
		backends = append(backends, "unconfigured")
	}
	pendingRollback := false
	if _, err := rollback.LoadLatestPending(s.cfg.Paths.HistoryDir); err == nil {
		pendingRollback = true
	}
	return map[string]any{
		"generated_at":      s.now(),
		"backends":          backends,
		"default_site_root": s.cfg.Paths.SitesRootDir,
		"managed_sites":     len(sites),
		"enabled_sites":     enabledSites,
		"disabled_sites":    disabledSites,
		"php_runtimes":      len(phpRuntimes),
		"pending_rollback":  pendingRollback,
	}
}

func (s Service) bundleManagedServicesSnapshot(ctx context.Context) map[string]any {
	probe, err := s.probeManagedServices(ctx)
	snapshot := map[string]any{
		"generated_at": s.now(),
	}
	if err != nil {
		snapshot["error"] = err.Error()
		return snapshot
	}
	snapshot["active"] = probe.Active
	snapshot["restartable"] = probe.Restartable
	snapshot["problematic"] = probe.Problematic
	snapshot["unprobed"] = probe.Unprobed
	snapshot["counts"] = map[string]int{
		"active":      len(probe.Active),
		"restartable": len(probe.Restartable),
		"problematic": len(probe.Problematic),
		"unprobed":    len(probe.Unprobed),
	}
	return snapshot
}

func (s Service) bundleProbeSnapshot(report Report) map[string]any {
	checks := make(map[string]Check, len(report.Checks))
	for _, check := range report.Checks {
		checks[check.Name] = check
	}
	return map[string]any{
		"generated_at": report.GeneratedAt,
		"status":       report.Status,
		"checks":       checks,
	}
}

func (s Service) writeTree(tw *tar.Writer, tarRoot string, sourceDir string, entries *int) error {
	if strings.TrimSpace(sourceDir) == "" {
		return nil
	}
	info, err := os.Stat(sourceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := writeTarFile(tw, filepath.ToSlash(filepath.Join(tarRoot, rel)), raw, info.Mode().Perm(), info.ModTime()); err != nil {
			return err
		}
		*entries = *entries + 1
		return nil
	})
}

func (s Service) writeOptionalFile(tw *tar.Writer, name string, path string, entries *int) error {
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := writeTarFile(tw, name, raw, 0o644, s.now()); err != nil {
		return err
	}
	*entries = *entries + 1
	return nil
}

func (s Service) writeRecentHistory(tw *tar.Writer, tarRoot string, entries *int) error {
	entriesOnDisk, err := os.ReadDir(s.cfg.Paths.HistoryDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	files := make([]string, 0, len(entriesOnDisk))
	for _, entry := range entriesOnDisk {
		if entry.IsDir() {
			continue
		}
		files = append(files, filepath.Join(s.cfg.Paths.HistoryDir, entry.Name()))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	if len(files) > 10 {
		files = files[:10]
	}
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := writeTarFile(tw, filepath.ToSlash(filepath.Join(tarRoot, filepath.Base(path))), raw, 0o644, s.now()); err != nil {
			return err
		}
		*entries = *entries + 1
	}
	return nil
}

func (s Service) writeManagedLogTails(tw *tar.Writer, tarRoot string, lines int, entries *int) error {
	manifests, err := s.siteMgr.List()
	if err != nil {
		return err
	}
	for _, manifest := range manifests {
		for kind, path := range map[string]string{
			"access.log": manifest.Site.Logs.AccessLog,
			"error.log":  manifest.Site.Logs.ErrorLog,
		} {
			raw, ok, err := readLogTail(path, lines)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			name := filepath.ToSlash(filepath.Join(tarRoot, manifest.Site.Name, kind))
			if err := writeTarFile(tw, name, raw, 0o644, s.now()); err != nil {
				return err
			}
			*entries = *entries + 1
		}
	}
	return nil
}

func (s Service) writeCommandSnapshots(ctx context.Context, tw *tar.Writer, tarRoot string, entries *int) error {
	snapshots := []struct {
		Name string
		Cmd  system.Command
	}{
		{
			Name: "getenforce.json",
			Cmd:  system.Command{Name: "getenforce"},
		},
		{
			Name: "firewall-state.json",
			Cmd:  system.Command{Name: "firewall-cmd", Args: []string{"--state"}},
		},
		{
			Name: "firewall-list-ports.json",
			Cmd:  system.Command{Name: "firewall-cmd", Args: []string{"--list-ports"}},
		},
		{
			Name: "ss-ltn.json",
			Cmd:  system.Command{Name: "ss", Args: []string{"-ltn"}},
		},
		{
			Name: "systemctl-failed.json",
			Cmd:  system.Command{Name: "systemctl", Args: []string{"--failed", "--no-pager", "--plain"}},
		},
		{
			Name: "uname-a.json",
			Cmd:  system.Command{Name: "uname", Args: []string{"-a"}},
		},
		{
			Name: "hostnamectl.json",
			Cmd:  system.Command{Name: "hostnamectl"},
		},
		{
			Name: "df-h.json",
			Cmd:  system.Command{Name: "df", Args: []string{"-h"}},
		},
		{
			Name: "free-m.json",
			Cmd:  system.Command{Name: "free", Args: []string{"-m"}},
		},
		{
			Name: "ps-aux.json",
			Cmd:  system.Command{Name: "ps", Args: []string{"aux", "--no-headers"}},
		},
	}

	for _, snapshot := range snapshots {
		value := s.captureCommandSnapshot(ctx, snapshot.Cmd)
		raw, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		raw = append(raw, '\n')
		if err := writeTarFile(tw, filepath.ToSlash(filepath.Join(tarRoot, snapshot.Name)), raw, 0o644, s.now()); err != nil {
			return err
		}
		*entries = *entries + 1
	}

	return nil
}

func (s Service) writeManagedServiceJournalSnapshots(ctx context.Context, tw *tar.Writer, tarRoot string, entries *int) error {
	services, _, err := s.collectManagedServices()
	if err != nil {
		return err
	}
	for _, serviceName := range services {
		value := s.captureCommandSnapshot(ctx, system.Command{
			Name: "journalctl",
			Args: []string{"-u", serviceName, "-n", "50", "--no-pager"},
		})
		raw, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		raw = append(raw, '\n')
		name := filepath.ToSlash(filepath.Join(tarRoot, sanitizeBundleName(serviceName)+".json"))
		if err := writeTarFile(tw, name, raw, 0o644, s.now()); err != nil {
			return err
		}
		*entries = *entries + 1
	}
	return nil
}

func (s Service) writeProviderConfigSnapshots(tw *tar.Writer, tarRoot string, entries *int) error {
	files := map[string]string{
		filepath.ToSlash(filepath.Join(tarRoot, "db", "mariadb-tls.conf")):    s.cfg.DB.MariaDBTLSConfigPath,
		filepath.ToSlash(filepath.Join(tarRoot, "db", "mysql-tls.conf")):      s.cfg.DB.MySQLTLSConfigPath,
		filepath.ToSlash(filepath.Join(tarRoot, "db", "percona-tls.conf")):    s.cfg.DB.PerconaTLSConfigPath,
		filepath.ToSlash(filepath.Join(tarRoot, "db", "postgresql-tls.conf")): s.cfg.DB.PostgreSQLTLSConfigPath,
		filepath.ToSlash(filepath.Join(tarRoot, "cache", "memcached.conf")):   s.cfg.Cache.MemcachedConfigPath,
		filepath.ToSlash(filepath.Join(tarRoot, "cache", "redis.conf")):       s.cfg.Cache.RedisConfigPath,
	}
	for name, path := range files {
		if err := s.writeOptionalFile(tw, name, path, entries); err != nil {
			return err
		}
	}
	return nil
}

func (s Service) captureCommandSnapshot(ctx context.Context, cmd system.Command) map[string]any {
	result, err := s.exec.Run(ctx, cmd)
	snapshot := map[string]any{
		"captured_at": s.now(),
		"command":     append([]string{cmd.Name}, cmd.Args...),
		"stdout":      result.Stdout,
		"stderr":      result.Stderr,
		"exit_code":   result.ExitCode,
	}
	if err != nil {
		snapshot["error"] = err.Error()
		if isCommandNotFound(err) {
			snapshot["status"] = "missing"
		} else {
			snapshot["status"] = "error"
		}
		return snapshot
	}
	snapshot["status"] = "ok"
	return snapshot
}

func sanitizeBundleName(value string) string {
	return strings.NewReplacer("/", "_", " ", "_", ".", "_", ":", "_").Replace(strings.TrimSpace(value))
}

func readLogTail(path string, lines int) ([]byte, bool, error) {
	if strings.TrimSpace(path) == "" {
		return nil, false, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if lines <= 0 {
		lines = 20
	}
	trimmed := strings.TrimRight(string(raw), "\n")
	if trimmed == "" {
		return []byte{}, true, nil
	}
	parts := strings.Split(trimmed, "\n")
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
	}
	return append([]byte(strings.Join(parts, "\n")), '\n'), true, nil
}

func (s Service) writeTLSCertSnapshots(ctx context.Context, tw *tar.Writer, tarRoot string, entries *int) error {
	sites, err := s.siteMgr.List()
	if err != nil || len(sites) == 0 {
		return nil
	}
	type certInfo struct {
		Site     string `json:"site"`
		CertFile string `json:"cert_file"`
		EndDate  string `json:"end_date"`
		Status   string `json:"status"`
	}
	var certs []certInfo
	for _, manifest := range sites {
		if !manifest.Site.TLS.Enabled || strings.TrimSpace(manifest.Site.TLS.CertificateFile) == "" {
			continue
		}
		certPath := manifest.Site.TLS.CertificateFile
		siteName := manifest.Site.Domain.ServerName
		if _, statErr := os.Stat(certPath); statErr != nil {
			certs = append(certs, certInfo{
				Site:     siteName,
				CertFile: certPath,
				Status:   "missing",
			})
			continue
		}
		result, runErr := s.exec.Run(ctx, system.Command{
			Name: "openssl",
			Args: []string{"x509", "-enddate", "-noout", "-in", certPath},
		})
		if runErr != nil || result.ExitCode != 0 {
			certs = append(certs, certInfo{
				Site:     siteName,
				CertFile: certPath,
				Status:   "unreadable",
			})
			continue
		}
		endDate := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(result.Stdout), "notAfter="))
		certs = append(certs, certInfo{
			Site:     siteName,
			CertFile: certPath,
			EndDate:  endDate,
			Status:   "ok",
		})
	}
	if len(certs) == 0 {
		return nil
	}
	raw, err := json.MarshalIndent(map[string]any{
		"captured_at":  s.now().Format(time.RFC3339),
		"certificates": certs,
	}, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := writeTarFile(tw, filepath.ToSlash(filepath.Join(tarRoot, "summary.json")), raw, 0o644, s.now()); err != nil {
		return err
	}
	*entries++
	return nil
}

func writeTarFile(tw *tar.Writer, name string, raw []byte, mode os.FileMode, modTime time.Time) error {
	header := &tar.Header{
		Name:    name,
		Mode:    int64(mode.Perm()),
		Size:    int64(len(raw)),
		ModTime: modTime,
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write(raw)
	return err
}
