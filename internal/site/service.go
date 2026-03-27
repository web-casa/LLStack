package site

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/web-casa/llstack/internal/backend/apache"
	"github.com/web-casa/llstack/internal/backend/lsws"
	"github.com/web-casa/llstack/internal/backend/ols"
	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/core/apply"
	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/core/plan"
	"github.com/web-casa/llstack/internal/core/render"
	"github.com/web-casa/llstack/internal/core/validate"
	verifycore "github.com/web-casa/llstack/internal/core/verify"
	"github.com/web-casa/llstack/internal/logging"
	phpruntime "github.com/web-casa/llstack/internal/php"
	"github.com/web-casa/llstack/internal/rollback"
	sslprovider "github.com/web-casa/llstack/internal/ssl"
	"github.com/web-casa/llstack/internal/system"
)

// Manager coordinates canonical site operations for Phase 2.
type Manager struct {
	cfg     config.RuntimeConfig
	logger  logging.Logger
	applier apply.FileApplier
	exec    system.Executor
}

// NewManager constructs a site manager.
func NewManager(cfg config.RuntimeConfig, logger logging.Logger, exec system.Executor) Manager {
	return Manager{
		cfg:     cfg,
		logger:  logger,
		applier: apply.NewFileApplier(cfg.Paths.BackupsDir),
		exec:    exec,
	}
}

// CreateOptions controls site creation behavior.
type CreateOptions struct {
	Site       model.Site
	DryRun     bool
	PlanOnly   bool
	SkipReload bool
}

// DeleteOptions controls site deletion behavior.
type DeleteOptions struct {
	Name       string
	DryRun     bool
	PlanOnly   bool
	PurgeRoot  bool
	SkipReload bool
}

// RollbackOptions controls rollback behavior.
type RollbackOptions struct {
	DryRun     bool
	PlanOnly   bool
	SkipReload bool
}

// ReconcileOptions controls canonical site re-application from the stored manifest.
type ReconcileOptions struct {
	Name       string
	DryRun     bool
	PlanOnly   bool
	SkipReload bool
}

// UpdatePHPOptions controls per-site PHP version changes.
type UpdatePHPOptions struct {
	Name       string
	Version    string
	DryRun     bool
	PlanOnly   bool
	SkipReload bool
}

// UpdateTLSOptions controls per-site TLS changes.
type UpdateTLSOptions struct {
	Name            string
	Mode            string
	CertificateFile string
	CertificateKey  string
	Email           string
	DryRun          bool
	PlanOnly        bool
	SkipReload      bool
}

// UpdateSettingsOptions controls editable site settings changes.
type UpdateSettingsOptions struct {
	Name         string
	DocumentRoot string
	Aliases      []string
	IndexFiles   []string
	Upstream     string
	DryRun       bool
	PlanOnly     bool
	SkipReload   bool
}

// LogReadOptions controls log viewing behavior.
type LogReadOptions struct {
	Name  string
	Kind  string
	Lines int
}

// StateChangeOptions controls site state transitions.
type StateChangeOptions struct {
	Name       string
	State      string
	DryRun     bool
	PlanOnly   bool
	SkipReload bool
}

// Create creates a site plan and optionally applies it.
func (m Manager) Create(ctx context.Context, opts CreateOptions) (plan.Plan, error) {
	site, err := m.withDefaults(opts.Site)
	if err := validate.Site(site); err != nil {
		return plan.Plan{}, err
	}
	if _, err := m.loadManifest(site.Name); err == nil {
		return plan.Plan{}, fmt.Errorf("site %q already exists", site.Name)
	}

	renderer, verifier, renderOpts, err := m.backendComponents(ctx, site)
	if err != nil {
		return plan.Plan{}, err
	}

	// Set per-site isolation options for rendering (informational only, does not mutate site model)
	renderOpts.SystemUser = system.SiteUsername(site.Name)

	manifestPath := m.manifestPath(site.Name)
	rendered, err := renderer.RenderSite(site, renderOpts)
	if err != nil {
		return plan.Plan{}, err
	}
	rendered.Assets = append(rendered.Assets, ScaffoldAssets(site)...)

	// Per-site FPM pool is managed separately via `llstack php:tune` to avoid
	// conflicts with the default shared www pool during site lifecycle operations.

	p := plan.New("site.create", "Create "+strings.ToUpper(site.Backend)+"-managed site "+site.Name)
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly
	p.Warnings = append(p.Warnings, rendered.Warnings...)
	p.AddOperation(plan.Operation{ID: "ensure-sites-root", Kind: "mkdir", Target: site.DocumentRoot})
	p.AddOperation(plan.Operation{ID: "ensure-site-logs", Kind: "mkdir", Target: filepath.Dir(site.Logs.AccessLog)})
	for idx, asset := range rendered.Assets {
		p.AddOperation(plan.Operation{ID: fmt.Sprintf("write-asset-%d", idx+1), Kind: "write_file", Target: asset.Path})
	}
	p.AddOperation(plan.Operation{ID: "write-manifest", Kind: "write_file", Target: manifestPath})
	if !opts.SkipReload {
		p.AddOperation(plan.Operation{ID: "backend-configtest", Kind: "service.verify", Target: renderVerifierName(site.Backend, true, m.cfg)})
		p.AddOperation(plan.Operation{ID: "backend-reload", Kind: "service.reload", Target: renderVerifierName(site.Backend, false, m.cfg)})
	}

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	changes, err := m.applyCreate(ctx, site, manifestPath, rendered, verifier, !opts.SkipReload)
	if err != nil {
		return p, err
	}
	if _, err := rollback.Save(m.cfg.Paths.HistoryDir, rollback.Entry{
		ID:        "site-create-" + site.Name,
		Action:    "site.create",
		Resource:  site.Name,
		Backend:   site.Backend,
		Timestamp: time.Now().UTC(),
		Changes:   changes,
	}); err != nil {
		return p, err
	}

	m.logger.Info("site created", "site", site.Name, "backend", site.Backend)
	return p, nil
}

// Delete deletes a site and optionally purges the docroot.
func (m Manager) Delete(ctx context.Context, opts DeleteOptions) (plan.Plan, error) {
	manifest, err := m.loadManifest(opts.Name)
	if err != nil {
		return plan.Plan{}, err
	}

	p := plan.New("site.delete", "Delete managed site "+opts.Name)
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly
	for idx, path := range manifest.ManagedAssetPaths {
		p.AddOperation(plan.Operation{ID: fmt.Sprintf("delete-asset-%d", idx+1), Kind: "delete_file", Target: path})
	}
	p.AddOperation(plan.Operation{ID: "delete-manifest", Kind: "delete_file", Target: m.manifestPath(opts.Name)})
	if opts.PurgeRoot {
		p.AddOperation(plan.Operation{ID: "purge-docroot", Kind: "delete_dir", Target: manifest.Site.DocumentRoot})
	}
	if !opts.SkipReload {
		p.AddOperation(plan.Operation{ID: "backend-configtest", Kind: "service.verify", Target: renderVerifierName(manifest.Site.Backend, true, m.cfg)})
		p.AddOperation(plan.Operation{ID: "backend-reload", Kind: "service.reload", Target: renderVerifierName(manifest.Site.Backend, false, m.cfg)})
	}

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	_, verifier, _, err := m.backendComponents(ctx, manifest.Site)
	if err != nil {
		return p, err
	}
	changes, err := m.applyDelete(ctx, manifest, opts.PurgeRoot, verifier, !opts.SkipReload)
	if err != nil {
		return p, err
	}
	if _, err := rollback.Save(m.cfg.Paths.HistoryDir, rollback.Entry{
		ID:        "site-delete-" + opts.Name,
		Action:    "site.delete",
		Resource:  opts.Name,
		Backend:   manifest.Site.Backend,
		Timestamp: time.Now().UTC(),
		Changes:   changes,
	}); err != nil {
		return p, err
	}

	m.logger.Info("site deleted", "site", opts.Name)
	return p, nil
}

// List returns all managed site manifests.
func (m Manager) List() ([]model.SiteManifest, error) {
	dir := m.cfg.Paths.ManagedSitesDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	out := make([]model.SiteManifest, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		manifest, err := m.loadManifest(strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())))
		if err != nil {
			return nil, err
		}
		out = append(out, manifest)
	}
	return out, nil
}

// Show returns a managed site manifest.
func (m Manager) Show(name string) (model.SiteManifest, error) {
	return m.loadManifest(name)
}

// RollbackLast rolls back the latest unrolled-back site operation.
func (m Manager) RollbackLast(ctx context.Context, opts RollbackOptions) (plan.Plan, error) {
	entry, err := rollback.LoadLatestPending(m.cfg.Paths.HistoryDir)
	if err != nil {
		return plan.Plan{}, err
	}

	p := plan.New("rollback", "Rollback last operation "+entry.Action+" on "+entry.Resource)
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly
	for i := len(entry.Changes) - 1; i >= 0; i-- {
		change := entry.Changes[i]
		p.AddOperation(plan.Operation{
			ID:     fmt.Sprintf("rollback-%d", i),
			Kind:   "rollback." + change.Kind,
			Target: change.Path,
		})
	}
	if !opts.SkipReload {
		p.AddOperation(plan.Operation{ID: "backend-configtest", Kind: "service.verify", Target: "configtest for backend of rollback target"})
		p.AddOperation(plan.Operation{ID: "backend-reload", Kind: "service.reload", Target: "reload for backend of rollback target"})
	}

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	for i := len(entry.Changes) - 1; i >= 0; i-- {
		if err := m.applier.Rollback(entry.Changes[i]); err != nil {
			return p, err
		}
	}
	if !opts.SkipReload {
		verifier, err := m.verifierForBackend(entry.Backend)
		if err != nil {
			return p, err
		}
		if err := verifier.ConfigTest(ctx); err != nil {
			return p, err
		}
		if err := verifier.Reload(ctx); err != nil {
			return p, err
		}
	}
	if err := rollback.MarkRolledBack(entry); err != nil {
		return p, err
	}

	m.logger.Info("site rollback completed", "resource", entry.Resource, "action", entry.Action)
	return p, nil
}

// Reconcile re-renders and rewrites the managed assets for an existing site manifest.
func (m Manager) Reconcile(ctx context.Context, opts ReconcileOptions) (plan.Plan, error) {
	manifest, err := m.loadManifest(opts.Name)
	if err != nil {
		return plan.Plan{}, err
	}

	updated, err := m.withDefaults(manifest.Site)
	if err != nil {
		return plan.Plan{}, err
	}
	renderer, verifier, renderOpts, err := m.backendComponents(ctx, updated)
	if err != nil {
		return plan.Plan{}, err
	}
	rendered, err := renderer.RenderSite(updated, renderOpts)
	if err != nil {
		return plan.Plan{}, err
	}
	rendered.Assets = append(rendered.Assets, ScaffoldAssets(updated)...)

	p := plan.New("site.reconcile", "Reconcile managed site "+opts.Name)
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly
	p.Warnings = append(p.Warnings, rendered.Warnings...)
	p.AddOperation(plan.Operation{ID: "ensure-sites-root", Kind: "mkdir", Target: updated.DocumentRoot})
	p.AddOperation(plan.Operation{ID: "ensure-site-logs", Kind: "mkdir", Target: filepath.Dir(updated.Logs.AccessLog)})
	for idx, asset := range rendered.Assets {
		p.AddOperation(plan.Operation{ID: fmt.Sprintf("rewrite-asset-%d", idx+1), Kind: "write_file", Target: asset.Path})
	}
	p.AddOperation(plan.Operation{ID: "write-manifest", Kind: "write_file", Target: m.manifestPath(opts.Name)})
	if !opts.SkipReload {
		p.AddOperation(plan.Operation{ID: "backend-configtest", Kind: "service.verify", Target: renderVerifierName(updated.Backend, true, m.cfg)})
		p.AddOperation(plan.Operation{ID: "backend-reload", Kind: "service.reload", Target: renderVerifierName(updated.Backend, false, m.cfg)})
	}

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	changes, err := m.applyCreate(ctx, updated, m.manifestPath(opts.Name), rendered, verifier, !opts.SkipReload)
	if err != nil {
		return p, err
	}
	if _, err := rollback.Save(m.cfg.Paths.HistoryDir, rollback.Entry{
		ID:        "site-reconcile-" + opts.Name,
		Action:    "site.reconcile",
		Resource:  opts.Name,
		Backend:   updated.Backend,
		Timestamp: time.Now().UTC(),
		Changes:   changes,
	}); err != nil {
		return p, err
	}
	return p, nil
}

// UpdateSettings updates editable canonical site settings and re-renders managed assets.
func (m Manager) UpdateSettings(ctx context.Context, opts UpdateSettingsOptions) (plan.Plan, error) {
	manifest, err := m.loadManifest(opts.Name)
	if err != nil {
		return plan.Plan{}, err
	}

	updated := manifest.Site
	docrootChanged := false
	if strings.TrimSpace(opts.DocumentRoot) != "" && strings.TrimSpace(opts.DocumentRoot) != updated.DocumentRoot {
		updated.DocumentRoot = strings.TrimSpace(opts.DocumentRoot)
		docrootChanged = true
	}
	if opts.Aliases != nil {
		updated.Domain.Aliases = cleanedAliases(opts.Aliases)
	}
	if opts.IndexFiles != nil {
		updated.IndexFiles = cleanedAliases(opts.IndexFiles)
	}
	if strings.TrimSpace(opts.Upstream) != "" {
		if len(updated.ReverseProxyRules) == 0 {
			updated.ReverseProxyRules = []model.ReverseProxyRule{
				{
					PathPrefix:   "/",
					Upstream:     strings.TrimSpace(opts.Upstream),
					PreserveHost: true,
				},
			}
		} else {
			updated.ReverseProxyRules[0].Upstream = strings.TrimSpace(opts.Upstream)
		}
	}
	updated.UpdatedAt = time.Now().UTC()
	updated, err = m.withDefaults(updated)
	if err != nil {
		return plan.Plan{}, err
	}

	renderer, verifier, renderOpts, err := m.backendComponents(ctx, updated)
	if err != nil {
		return plan.Plan{}, err
	}
	rendered, err := renderer.RenderSite(updated, renderOpts)
	if err != nil {
		return plan.Plan{}, err
	}

	p := plan.New("site.update", "Update managed site "+opts.Name)
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly
	p.Warnings = append(p.Warnings, rendered.Warnings...)
	if docrootChanged {
		p.Warnings = append(p.Warnings,
			fmt.Sprintf("document root changed to %s; existing site files are NOT automatically migrated — copy them manually if needed", updated.DocumentRoot))
	}
	p.AddOperation(plan.Operation{ID: "ensure-sites-root", Kind: "mkdir", Target: updated.DocumentRoot})
	p.AddOperation(plan.Operation{ID: "ensure-site-logs", Kind: "mkdir", Target: filepath.Dir(updated.Logs.AccessLog)})
	for idx, asset := range rendered.Assets {
		p.AddOperation(plan.Operation{
			ID:     fmt.Sprintf("rewrite-asset-%d", idx+1),
			Kind:   "write_file",
			Target: asset.Path,
		})
	}
	p.AddOperation(plan.Operation{ID: "write-manifest", Kind: "write_file", Target: m.manifestPath(opts.Name)})
	if !opts.SkipReload {
		p.AddOperation(plan.Operation{ID: "backend-configtest", Kind: "service.verify", Target: renderVerifierName(updated.Backend, true, m.cfg)})
		p.AddOperation(plan.Operation{ID: "backend-reload", Kind: "service.reload", Target: renderVerifierName(updated.Backend, false, m.cfg)})
	}

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	changes, err := m.applyCreate(ctx, updated, m.manifestPath(opts.Name), rendered, verifier, !opts.SkipReload)
	if err != nil {
		return p, err
	}
	if _, err := rollback.Save(m.cfg.Paths.HistoryDir, rollback.Entry{
		ID:        "site-update-" + opts.Name,
		Action:    "site.update",
		Resource:  opts.Name,
		Backend:   updated.Backend,
		Timestamp: time.Now().UTC(),
		Changes:   changes,
	}); err != nil {
		return p, err
	}
	return p, nil
}

// UpdatePHPVersion updates the runtime bound to a managed site.
func (m Manager) UpdatePHPVersion(ctx context.Context, opts UpdatePHPOptions) (plan.Plan, error) {
	manifest, err := m.loadManifest(opts.Name)
	if err != nil {
		return plan.Plan{}, err
	}

	updated := manifest.Site
	updated.PHP.Enabled = true
	updated.PHP.Version = opts.Version
	updated.UpdatedAt = time.Now().UTC()
	updated, err = m.withDefaults(updated)
	if err != nil {
		return plan.Plan{}, err
	}

	renderer, verifier, renderOpts, err := m.backendComponents(ctx, updated)
	if err != nil {
		return plan.Plan{}, err
	}
	rendered, err := renderer.RenderSite(updated, renderOpts)
	if err != nil {
		return plan.Plan{}, err
	}

	p := plan.New("site.php", "Switch PHP runtime for "+opts.Name+" to "+opts.Version)
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly
	p.Warnings = append(p.Warnings, rendered.Warnings...)
	for idx, asset := range rendered.Assets {
		p.AddOperation(plan.Operation{
			ID:     fmt.Sprintf("rewrite-asset-%d", idx+1),
			Kind:   "write_file",
			Target: asset.Path,
		})
	}
	p.AddOperation(plan.Operation{ID: "write-manifest", Kind: "write_file", Target: m.manifestPath(opts.Name)})
	if !opts.SkipReload {
		p.AddOperation(plan.Operation{ID: "backend-configtest", Kind: "service.verify", Target: renderVerifierName(updated.Backend, true, m.cfg)})
		p.AddOperation(plan.Operation{ID: "backend-reload", Kind: "service.reload", Target: renderVerifierName(updated.Backend, false, m.cfg)})
	}

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	changes, err := m.applyCreate(ctx, updated, m.manifestPath(opts.Name), rendered, verifier, !opts.SkipReload)
	if err != nil {
		return p, err
	}
	if _, err := rollback.Save(m.cfg.Paths.HistoryDir, rollback.Entry{
		ID:        "site-php-" + opts.Name + "-" + strings.ReplaceAll(opts.Version, ".", "-"),
		Action:    "site.php",
		Resource:  opts.Name,
		Backend:   updated.Backend,
		Timestamp: time.Now().UTC(),
		Changes:   changes,
	}); err != nil {
		return p, err
	}
	return p, nil
}

// UpdateTLS updates the TLS mode and certificate paths of a managed site.
func (m Manager) UpdateTLS(ctx context.Context, opts UpdateTLSOptions) (plan.Plan, error) {
	manifest, err := m.loadManifest(opts.Name)
	if err != nil {
		return plan.Plan{}, err
	}
	if opts.Mode == "custom" && (opts.CertificateFile == "" || opts.CertificateKey == "") {
		return plan.Plan{}, fmt.Errorf("custom TLS mode requires certificate_file and certificate_key")
	}

	updated := manifest.Site
	updated.TLS.Enabled = true
	updated.TLS.Mode = opts.Mode
	if opts.Mode == "letsencrypt" {
		updated.TLS.CertificateFile = filepath.Join(m.cfg.SSL.LetsEncryptLiveDir, updated.Domain.ServerName, "fullchain.pem")
		updated.TLS.CertificateKey = filepath.Join(m.cfg.SSL.LetsEncryptLiveDir, updated.Domain.ServerName, "privkey.pem")
	} else {
		updated.TLS.CertificateFile = opts.CertificateFile
		updated.TLS.CertificateKey = opts.CertificateKey
	}
	updated.UpdatedAt = time.Now().UTC()
	updated, err = m.withDefaults(updated)
	if err != nil {
		return plan.Plan{}, err
	}

	renderer, verifier, renderOpts, err := m.backendComponents(ctx, updated)
	if err != nil {
		return plan.Plan{}, err
	}
	rendered, err := renderer.RenderSite(updated, renderOpts)
	if err != nil {
		return plan.Plan{}, err
	}

	p := plan.New("site.ssl", "Configure TLS for "+opts.Name)
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly
	p.Warnings = append(p.Warnings, rendered.Warnings...)
	certbotInfo := sslprovider.CertbotInfo{}
	if opts.Mode == "letsencrypt" {
		certbotInfo = sslprovider.DetectCertbot(m.cfg)
		details := map[string]string{
			"email":   opts.Email,
			"webroot": updated.DocumentRoot,
		}
		if certbotInfo.Found {
			details["binary"] = certbotInfo.Binary
		} else {
			p.Warnings = append(p.Warnings, "certbot binary not detected; apply will fail until certbot is installed or configured")
		}
		p.AddOperation(plan.Operation{
			ID:      "acme-issue",
			Kind:    "acme.issue",
			Target:  updated.Domain.ServerName,
			Details: details,
		})
	}
	for idx, asset := range rendered.Assets {
		p.AddOperation(plan.Operation{
			ID:     fmt.Sprintf("rewrite-asset-%d", idx+1),
			Kind:   "write_file",
			Target: asset.Path,
		})
	}
	p.AddOperation(plan.Operation{ID: "write-manifest", Kind: "write_file", Target: m.manifestPath(opts.Name)})
	if !opts.SkipReload {
		p.AddOperation(plan.Operation{ID: "backend-configtest", Kind: "service.verify", Target: renderVerifierName(updated.Backend, true, m.cfg)})
		p.AddOperation(plan.Operation{ID: "backend-reload", Kind: "service.reload", Target: renderVerifierName(updated.Backend, false, m.cfg)})
	}

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}
	if opts.Mode == "letsencrypt" {
		if err := m.issueLetsEncrypt(ctx, certbotInfo, updated.Domain.ServerName, updated.DocumentRoot, opts.Email); err != nil {
			return p, err
		}
	}

	changes, err := m.applyCreate(ctx, updated, m.manifestPath(opts.Name), rendered, verifier, !opts.SkipReload)
	if err != nil {
		return p, err
	}
	if _, err := rollback.Save(m.cfg.Paths.HistoryDir, rollback.Entry{
		ID:        "site-ssl-" + opts.Name,
		Action:    "site.ssl",
		Resource:  opts.Name,
		Backend:   updated.Backend,
		Timestamp: time.Now().UTC(),
		Changes:   changes,
	}); err != nil {
		return p, err
	}
	return p, nil
}

// Reload re-runs configtest and reload for the site's backend.
func (m Manager) Reload(ctx context.Context, name string) (plan.Plan, error) {
	manifest, err := m.loadManifest(name)
	if err != nil {
		return plan.Plan{}, err
	}
	verifier, err := m.verifierForBackend(manifest.Site.Backend)
	if err != nil {
		return plan.Plan{}, err
	}
	p := plan.New("site.reload", "Reload managed site "+name)
	p.AddOperation(plan.Operation{ID: "backend-configtest", Kind: "service.verify", Target: renderVerifierName(manifest.Site.Backend, true, m.cfg)})
	p.AddOperation(plan.Operation{ID: "backend-reload", Kind: "service.reload", Target: renderVerifierName(manifest.Site.Backend, false, m.cfg)})
	if err := verifier.ConfigTest(ctx); err != nil {
		return p, err
	}
	if err := verifier.Reload(ctx); err != nil {
		return p, err
	}
	return p, nil
}

// Restart re-runs configtest and restarts the site's backend.
func (m Manager) Restart(ctx context.Context, name string) (plan.Plan, error) {
	manifest, err := m.loadManifest(name)
	if err != nil {
		return plan.Plan{}, err
	}
	verifier, err := m.verifierForBackend(manifest.Site.Backend)
	if err != nil {
		return plan.Plan{}, err
	}
	p := plan.New("site.restart", "Restart managed site "+name)
	p.AddOperation(plan.Operation{ID: "backend-configtest", Kind: "service.verify", Target: renderVerifierName(manifest.Site.Backend, true, m.cfg)})
	p.AddOperation(plan.Operation{ID: "backend-restart", Kind: "service.restart", Target: renderRestartName(manifest.Site.Backend, m.cfg)})
	if err := verifier.ConfigTest(ctx); err != nil {
		return p, err
	}
	if err := verifier.Restart(ctx); err != nil {
		return p, err
	}
	return p, nil
}

// ReadLogs reads the latest log lines for a managed site.
func (m Manager) ReadLogs(opts LogReadOptions) ([]string, error) {
	manifest, err := m.loadManifest(opts.Name)
	if err != nil {
		return nil, err
	}
	path := manifest.Site.Logs.AccessLog
	if opts.Kind == "error" {
		path = manifest.Site.Logs.ErrorLog
	}
	if opts.Lines <= 0 {
		opts.Lines = 20
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) > opts.Lines {
		lines = lines[len(lines)-opts.Lines:]
	}
	return lines, nil
}

// SetState toggles a managed site between enabled and disabled.
func (m Manager) SetState(ctx context.Context, opts StateChangeOptions) (plan.Plan, error) {
	manifest, err := m.loadManifest(opts.Name)
	if err != nil {
		return plan.Plan{}, err
	}
	if opts.State != "enabled" && opts.State != "disabled" {
		return plan.Plan{}, fmt.Errorf("unsupported site state %q", opts.State)
	}

	updated := manifest.Site
	updated.State = opts.State
	updated.UpdatedAt = time.Now().UTC()
	updated, err = m.withDefaults(updated)
	if err != nil {
		return plan.Plan{}, err
	}

	renderer, verifier, renderOpts, err := m.backendComponents(ctx, updated)
	if err != nil {
		return plan.Plan{}, err
	}
	rendered, err := renderer.RenderSite(updated, renderOpts)
	if err != nil {
		return plan.Plan{}, err
	}
	newPaths := assetPaths(rendered.Assets)
	oldBackendPaths := filterPaths(manifest.ManagedAssetPaths, func(path string) bool {
		return !strings.HasPrefix(path, updated.DocumentRoot)
	})
	removals := difference(oldBackendPaths, newPaths)

	p := plan.New("site."+opts.State, strings.Title(opts.State)+" managed site "+opts.Name)
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly
	for idx, asset := range rendered.Assets {
		p.AddOperation(plan.Operation{
			ID:     fmt.Sprintf("write-asset-%d", idx+1),
			Kind:   "write_file",
			Target: asset.Path,
		})
	}
	for idx, path := range removals {
		p.AddOperation(plan.Operation{
			ID:     fmt.Sprintf("delete-old-asset-%d", idx+1),
			Kind:   "delete_file",
			Target: path,
		})
	}
	p.AddOperation(plan.Operation{ID: "write-manifest", Kind: "write_file", Target: m.manifestPath(opts.Name)})
	if !opts.SkipReload {
		p.AddOperation(plan.Operation{ID: "backend-configtest", Kind: "service.verify", Target: renderVerifierName(updated.Backend, true, m.cfg)})
		p.AddOperation(plan.Operation{ID: "backend-reload", Kind: "service.reload", Target: renderVerifierName(updated.Backend, false, m.cfg)})
	}

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	changes := make([]apply.Change, 0, len(rendered.Assets)+len(removals)+1)
	for _, asset := range rendered.Assets {
		change, err := m.applier.WriteFile(asset.Path, asset.Content, asset.Mode)
		if err != nil {
			return p, err
		}
		changes = append(changes, change)
	}
	for _, path := range removals {
		change, err := m.applier.DeleteFile(path)
		if err != nil {
			return p, err
		}
		changes = append(changes, change)
	}
	docrootAssets := filterPaths(manifest.ManagedAssetPaths, func(path string) bool {
		return strings.HasPrefix(path, updated.DocumentRoot)
	})
	nextManifest := manifest
	nextManifest.Site = updated
	nextManifest.VHostPath = primaryAssetPath(rendered.Assets)
	nextManifest.ManagedAssetPaths = uniqueAssetPaths(newPaths, docrootAssets)
	nextManifest.UpdatedAt = time.Now().UTC()
	change, err := m.applier.WriteJSON(m.manifestPath(opts.Name), nextManifest, 0o644)
	if err != nil {
		return p, err
	}
	changes = append(changes, change)
	if !opts.SkipReload {
		if err := verifier.ConfigTest(ctx); err != nil {
			return p, err
		}
		if err := verifier.Reload(ctx); err != nil {
			return p, err
		}
	}
	if _, err := rollback.Save(m.cfg.Paths.HistoryDir, rollback.Entry{
		ID:        "site-state-" + opts.Name + "-" + opts.State,
		Action:    "site." + opts.State,
		Resource:  opts.Name,
		Backend:   updated.Backend,
		Timestamp: time.Now().UTC(),
		Changes:   changes,
	}); err != nil {
		return p, err
	}
	return p, nil
}

func (m Manager) applyCreate(ctx context.Context, site model.Site, manifestPath string, rendered render.SiteRenderResult, verifier verifycore.Verifier, verifyAndReload bool) ([]apply.Change, error) {
	changes := make([]apply.Change, 0, 8)
	existing, _ := m.loadManifest(site.Name)

	record := func(change *apply.Change) {
		if change != nil {
			changes = append(changes, *change)
		}
	}

	// Create per-site Linux user (per-site isolation model)
	siteUser := system.SiteUsername(site.Name)
	if siteUser != "" {
		if err := system.CreateSiteUser(ctx, m.exec, siteUser, site.DocumentRoot); err != nil {
			m.logger.Info("site user creation skipped", "user", siteUser, "error", err)
			siteUser = "" // fall back to no user isolation
		} else {
			// Add web server user to site group (per-site group model)
			webUser := system.WebServerUser(site.Backend)
			if err := system.AddUserToGroup(ctx, m.exec, webUser, siteUser); err != nil {
				m.logger.Info("web server group membership skipped", "web_user", webUser, "group", siteUser, "error", err)
			}
		}
	}

	// Ensure all required directories exist (safe for first-run without `llstack install`)
	ensureDirs := []string{
		m.cfg.Paths.ConfigDir,
		m.cfg.Paths.ManagedSitesDir(),
		m.cfg.Paths.StateDir,
		m.cfg.Paths.HistoryDir,
		m.cfg.Paths.BackupsDir,
		m.cfg.Paths.LogDir,
		m.cfg.Paths.ParityReportsDir(),
		filepath.Dir(site.Logs.AccessLog),
	}
	if site.Backend == "apache" {
		ensureDirs = append(ensureDirs, m.cfg.Apache.ManagedVhostsDir)
	}
	if site.Backend == "ols" {
		ensureDirs = append(ensureDirs,
			m.cfg.OLS.ManagedVhostsRoot,
			m.cfg.OLS.ManagedListenersDir,
		)
	}
	if site.Backend == "lsws" {
		ensureDirs = append(ensureDirs, m.cfg.LSWS.ManagedIncludesDir)
	}
	for _, dir := range ensureDirs {
		if _, err := m.applier.EnsureDir(dir, 0o755); err != nil {
			return nil, err
		}
	}
	// Ensure sites root is world-searchable (fixes "search permissions missing" on /data/www)
	if chmodErr := os.Chmod(m.cfg.Paths.SitesRootDir, 0o755); chmodErr != nil {
		m.logger.Info("chmod sites root skipped", "path", m.cfg.Paths.SitesRootDir, "error", chmodErr)
	}

	for _, dir := range []string{
		site.DocumentRoot,
	} {
		change, err := m.applier.EnsureDir(dir, 0o750)
		if err != nil {
			return nil, err
		}
		record(change)
	}

	for _, asset := range rendered.Assets {
		change, err := m.applier.WriteFile(asset.Path, asset.Content, asset.Mode)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}

	createdAt := existing.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	managedPaths := uniqueAssetPaths(assetPaths(rendered.Assets), existing.ManagedAssetPaths)
	manifest := model.SiteManifest{
		Site:              site,
		SystemUser:        siteUser,
		VHostPath:         primaryAssetPath(rendered.Assets),
		ManagedAssetPaths: managedPaths,
		ParityReportPath:  parityReportPath(rendered.Assets),
		Capabilities:      rendered.Capabilities,
		CreatedAt:         createdAt,
		UpdatedAt:         time.Now().UTC(),
	}

	// Set site directory ownership to site user (per-site isolation)
	// chmod -R ensures all files are world-readable until per-site group membership is fully active
	if siteUser != "" && system.SiteUserExists(siteUser) {
		m.exec.Run(ctx, system.Command{Name: "chown", Args: []string{"-R", siteUser + ":" + siteUser, site.DocumentRoot}})
		m.exec.Run(ctx, system.Command{Name: "chmod", Args: []string{"-R", "o+rX", site.DocumentRoot}})
	}
	// Restore SELinux context after all file operations
	m.exec.Run(ctx, system.Command{Name: "restorecon", Args: []string{"-Rv", site.DocumentRoot}})

	change, err := m.applier.WriteJSON(manifestPath, manifest, 0o644)
	if err != nil {
		return nil, err
	}
	changes = append(changes, change)

	if site.Backend == "ols" && m.cfg.OLS.MainConfigPath != "" {
		if err := ols.RegisterSiteInMainConfig(m.cfg.OLS.MainConfigPath, site, primaryAssetPath(rendered.Assets)); err != nil {
			m.logger.Info("OLS main config registration skipped", "site", site.Name, "error", err)
		}
	}

	if verifyAndReload {
		if err := verifier.ConfigTest(ctx); err != nil {
			return nil, err
		}
		if err := verifier.Reload(ctx); err != nil {
			return nil, err
		}
	}

	return changes, nil
}

func (m Manager) applyDelete(ctx context.Context, manifest model.SiteManifest, purgeRoot bool, verifier verifycore.Verifier, verifyAndReload bool) ([]apply.Change, error) {
	changes := make([]apply.Change, 0, 4)

	for _, path := range append(append([]string{}, manifest.ManagedAssetPaths...), m.manifestPath(manifest.Site.Name)) {
		change, err := m.applier.DeleteFile(path)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}

	if purgeRoot {
		change, err := m.deleteDir(manifest.Site.DocumentRoot)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}

	// Clean up per-site user (per-site isolation model)
	if manifest.SystemUser != "" {
		webUser := system.WebServerUser(manifest.Site.Backend)
		system.RemoveUserFromGroup(ctx, m.exec, webUser, manifest.SystemUser)
		if err := system.DeleteSiteUser(ctx, m.exec, manifest.SystemUser); err != nil {
			m.logger.Info("site user deletion skipped", "user", manifest.SystemUser, "error", err)
		}
	}

	if manifest.Site.Backend == "ols" && m.cfg.OLS.MainConfigPath != "" {
		if err := ols.UnregisterSiteFromMainConfig(m.cfg.OLS.MainConfigPath, manifest.Site.Name); err != nil {
			m.logger.Info("OLS main config unregistration skipped", "site", manifest.Site.Name, "error", err)
		}
	}

	if verifyAndReload {
		if err := verifier.ConfigTest(ctx); err != nil {
			return nil, err
		}
		if err := verifier.Reload(ctx); err != nil {
			return nil, err
		}
	}

	return changes, nil
}

func (m Manager) deleteDir(path string) (apply.Change, error) {
	if _, err := os.Stat(path); err != nil {
		return apply.Change{}, err
	}
	if err := os.MkdirAll(m.cfg.Paths.BackupsDir, 0o755); err != nil {
		return apply.Change{}, err
	}
	backupPath := filepath.Join(m.cfg.Paths.BackupsDir, fmt.Sprintf("%d-%s.deleted-dir", time.Now().UTC().UnixNano(), filepath.Base(path)))
	if err := os.Rename(path, backupPath); err != nil {
		return apply.Change{}, err
	}
	return apply.Change{Path: path, Kind: "dir_deleted", BackupPath: backupPath}, nil
}

func (m Manager) withDefaults(site model.Site) (model.Site, error) {
	now := time.Now().UTC()
	if site.Name == "" {
		site.Name = site.Domain.ServerName
	}
	if site.Backend == "" {
		site.Backend = "apache"
	}
	if site.Domain.HTTPPort == 0 {
		site.Domain.HTTPPort = 80
	}
	if site.Domain.HTTPSPort == 0 {
		site.Domain.HTTPSPort = 443
	}
	if site.DocumentRoot == "" {
		site.DocumentRoot = filepath.Join(m.cfg.Paths.SitesRootDir, site.Name)
	}
	if site.State == "" {
		site.State = "enabled"
	}
	if len(site.IndexFiles) == 0 {
		site.IndexFiles = []string{"index.php", "index.html", "index.htm"}
	}
	if site.Logs.AccessLog == "" {
		site.Logs.AccessLog = filepath.Join(m.cfg.Paths.SiteLogsDir(), site.Name+".access.log")
	}
	if site.Logs.ErrorLog == "" {
		site.Logs.ErrorLog = filepath.Join(m.cfg.Paths.SiteLogsDir(), site.Name+".error.log")
	}
	if site.PHP.Enabled {
		if site.PHP.Version == "" {
			site.PHP.Version = "8.3"
		}
		if err := phpruntime.BindSiteRuntime(phpruntime.NewResolver(m.cfg), &site); err != nil {
			return model.Site{}, err
		}
	}
	if site.CreatedAt.IsZero() {
		site.CreatedAt = now
	}
	site.UpdatedAt = now
	return site, nil
}

func (m Manager) loadManifest(name string) (model.SiteManifest, error) {
	path := m.manifestPath(name)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return model.SiteManifest{}, fmt.Errorf("managed site %q not found", name)
		}
		return model.SiteManifest{}, err
	}
	var manifest model.SiteManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return model.SiteManifest{}, err
	}
	return manifest, nil
}

func (m Manager) manifestPath(name string) string {
	return filepath.Join(m.cfg.Paths.ManagedSitesDir(), name+".json")
}

func (m Manager) vhostPath(name string, state string) string {
	path := filepath.Join(m.cfg.Apache.ManagedVhostsDir, name+".conf")
	if state == "disabled" {
		return path + ".disabled"
	}
	return path
}

func (m Manager) backendComponents(ctx context.Context, site model.Site) (render.SiteRenderer, verifycore.Verifier, render.SiteRenderOptions, error) {
	switch site.Backend {
	case "apache":
		verifier, _ := m.verifierForBackend("apache")
		return apache.NewRenderer(), verifier, render.SiteRenderOptions{
			VHostPath: m.vhostPath(site.Name, site.State),
		}, nil
	case "ols":
		verifier, _ := m.verifierForBackend("ols")
		return ols.NewCompiler(m.cfg), verifier, render.SiteRenderOptions{
			VHostPath:        olsVHostPath(m.cfg, site.Name, site.State),
			OutputDir:        filepath.Join(m.cfg.OLS.ManagedVhostsRoot, site.Name),
			ListenerMapPath:  olsListenerMapPath(m.cfg, site.Name, site.State),
			ParityReportPath: filepath.Join(m.cfg.Paths.ParityReportsDir(), site.Name+".ols.json"),
		}, nil
	case "lsws":
		verifier, _ := m.verifierForBackend("lsws")
		license := lsws.NewDetector(m.cfg, m.exec).Detect(ctx)
		return lsws.NewRenderer(m.cfg, license), verifier, render.SiteRenderOptions{
			VHostPath:        lswsIncludePath(m.cfg, site.Name, site.State),
			ParityReportPath: filepath.Join(m.cfg.Paths.ParityReportsDir(), site.Name+".lsws.json"),
		}, nil
	default:
		return nil, nil, render.SiteRenderOptions{}, fmt.Errorf("unsupported backend %q", site.Backend)
	}
}

func (m Manager) verifierForBackend(backend string) (verifycore.Verifier, error) {
	switch backend {
	case "apache":
		return apache.NewVerifier(m.cfg, m.exec), nil
	case "ols":
		return ols.NewVerifier(m.cfg, m.exec), nil
	case "lsws":
		return lsws.NewVerifier(m.cfg, m.exec), nil
	default:
		return nil, fmt.Errorf("unsupported backend %q", backend)
	}
}

func (m Manager) issueLetsEncrypt(ctx context.Context, info sslprovider.CertbotInfo, domain string, docroot string, email string) error {
	command, err := sslprovider.BuildCertbotCommand(info, sslprovider.CertbotRequest{
		Domain:  domain,
		Docroot: docroot,
		Email:   email,
	})
	if err != nil {
		return err
	}
	result, err := m.exec.Run(ctx, command)
	if err != nil {
		return fmt.Errorf("certbot: %w (%s)", err, result.Stderr)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("certbot exited with %d: %s", result.ExitCode, result.Stderr)
	}
	return nil
}

func assetPaths(assets []render.Asset) []string {
	out := make([]string, 0, len(assets))
	for _, asset := range assets {
		out = append(out, asset.Path)
	}
	return out
}

func primaryAssetPath(assets []render.Asset) string {
	if len(assets) == 0 {
		return ""
	}
	return assets[0].Path
}

func parityReportPath(assets []render.Asset) string {
	for _, asset := range assets {
		if strings.Contains(asset.Path, ".ols.json") || strings.Contains(asset.Path, ".lsws.json") {
			return asset.Path
		}
	}
	return ""
}

func uniqueAssetPaths(primary []string, extra []string) []string {
	seen := make(map[string]struct{}, len(primary)+len(extra))
	out := make([]string, 0, len(primary)+len(extra))
	for _, collection := range [][]string{primary, extra} {
		for _, path := range collection {
			if path == "" {
				continue
			}
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			out = append(out, path)
		}
	}
	return out
}

func difference(left []string, right []string) []string {
	seen := make(map[string]struct{}, len(right))
	for _, path := range right {
		seen[path] = struct{}{}
	}
	out := make([]string, 0)
	for _, path := range left {
		if _, ok := seen[path]; ok {
			continue
		}
		out = append(out, path)
	}
	return out
}

func filterPaths(paths []string, keep func(string) bool) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if keep(path) {
			out = append(out, path)
		}
	}
	return out
}

func cleanedAliases(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
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
	return out
}

func olsVHostPath(cfg config.RuntimeConfig, name string, state string) string {
	path := filepath.Join(cfg.OLS.ManagedVhostsRoot, name, "vhconf.conf")
	if state == "disabled" {
		return path + ".disabled"
	}
	return path
}

func olsListenerMapPath(cfg config.RuntimeConfig, name string, state string) string {
	path := filepath.Join(cfg.OLS.ManagedListenersDir, name+".map")
	if state == "disabled" {
		return path + ".disabled"
	}
	return path
}

func lswsIncludePath(cfg config.RuntimeConfig, name string, state string) string {
	path := filepath.Join(cfg.LSWS.ManagedIncludesDir, name+".conf")
	if state == "disabled" {
		return path + ".disabled"
	}
	return path
}

func renderVerifierName(backend string, configTest bool, cfg config.RuntimeConfig) string {
	switch backend {
	case "apache":
		if configTest {
			return strings.Join(cfg.Apache.ConfigTestCmd, " ")
		}
		return strings.Join(cfg.Apache.ReloadCmd, " ")
	case "ols":
		if configTest {
			return strings.Join(cfg.OLS.ConfigTestCmd, " ")
		}
		return strings.Join(cfg.OLS.ReloadCmd, " ")
	case "lsws":
		if configTest {
			return strings.Join(cfg.LSWS.ConfigTestCmd, " ")
		}
		return strings.Join(cfg.LSWS.ReloadCmd, " ")
	default:
		return backend
	}
}

func renderRestartName(backend string, cfg config.RuntimeConfig) string {
	switch backend {
	case "apache":
		return strings.Join(cfg.Apache.RestartCmd, " ")
	case "ols":
		return strings.Join(cfg.OLS.RestartCmd, " ")
	case "lsws":
		return strings.Join(cfg.LSWS.RestartCmd, " ")
	default:
		return backend
	}
}
