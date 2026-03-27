package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/web-casa/llstack/internal/cache"
	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/core/plan"
	"github.com/web-casa/llstack/internal/db"
	"github.com/web-casa/llstack/internal/logging"
	phpruntime "github.com/web-casa/llstack/internal/php"
	"github.com/web-casa/llstack/internal/site"
	"github.com/web-casa/llstack/internal/system"
)

// Options captures the unified install flow input.
type Options struct {
	Backend        string
	PHPVersion     string
	PHPVersions    []string
	DBProvider     string
	DBTLS          string
	WithMemcached  bool
	WithRedis      bool
	Site           string
	Email          string
	SiteProfile    string
	SiteUpstream   string
	NonInteractive bool
	DryRun         bool
	PlanOnly       bool
}

// Service orchestrates install flows using existing subsystem managers.
type Service struct {
	cfg    config.RuntimeConfig
	logger logging.Logger
	exec   system.Executor
}

// NewService constructs an install service.
func NewService(cfg config.RuntimeConfig, logger logging.Logger, exec system.Executor) Service {
	return Service{cfg: cfg, logger: logger, exec: exec}
}

// Execute runs the unified install flow and returns the aggregate plan.
func (s Service) Execute(ctx context.Context, opts Options) (plan.Plan, error) {
	out := plan.New("install", "Install LLStack web stack components")
	out.DryRun = opts.DryRun
	out.PlanOnly = opts.PlanOnly

	appendPlan := func(next plan.Plan) {
		out.Warnings = append(out.Warnings, next.Warnings...)
		out.Operations = append(out.Operations, next.Operations...)
	}

	completedSteps := make([]string, 0, 5)

	wrapError := func(step string, err error) error {
		if len(completedSteps) == 0 {
			return err
		}
		return fmt.Errorf("%s failed: %w (previously completed: %s — manual cleanup may be required)",
			step, err, strings.Join(completedSteps, ", "))
	}

	if opts.Backend != "" {
		out.AddOperation(plan.Operation{ID: "select-backend", Kind: "backend.select", Target: opts.Backend})
	}

	// Prepare host environment: directories, SELinux context, FPM socket permissions
	if !opts.DryRun && !opts.PlanOnly {
		sitesRoot := s.cfg.Paths.SitesRootDir
		prepareDirs := []string{
			filepath.Join(s.cfg.Paths.ConfigDir, "sites"),
			s.cfg.Paths.StateDir, s.cfg.Paths.HistoryDir, s.cfg.Paths.BackupsDir,
			s.cfg.Paths.LogDir, sitesRoot,
		}
		for _, dir := range prepareDirs {
			os.MkdirAll(dir, 0o755)
		}
		// Ensure SELinux tools are available and set context for sites root
		s.exec.Run(ctx, system.Command{
			Name: "dnf", Args: []string{"-y", "install", "policycoreutils-python-utils"},
		})
		s.exec.Run(ctx, system.Command{
			Name: "semanage",
			Args: []string{"fcontext", "-a", "-t", "httpd_sys_rw_content_t", sitesRoot + "(/.*)?"},
		})
		s.exec.Run(ctx, system.Command{
			Name: "restorecon", Args: []string{"-Rv", sitesRoot},
		})
		completedSteps = append(completedSteps, "host-prepare")

		// Install and configure backend web server
		if opts.Backend == "apache" {
			s.exec.Run(ctx, system.Command{Name: "dnf", Args: []string{"-y", "install", "httpd"}})
			// Create LLStack managed vhost include
			includeDir := s.cfg.Apache.ManagedVhostsDir
			os.MkdirAll(includeDir, 0o755)
			// Place include file in conf.d/ (not conf.d/llstack/) so Apache auto-loads it
			confDir := filepath.Dir(filepath.Dir(includeDir)) // /etc/httpd/conf.d
			includeLine := fmt.Sprintf("IncludeOptional %s/*.conf\n", includeDir)
			targetFile := filepath.Join(confDir, "00-llstack-managed-sites.conf")
			if _, err := os.Stat(targetFile); os.IsNotExist(err) {
				if writeErr := os.WriteFile(targetFile, []byte(includeLine), 0o644); writeErr != nil {
					s.logger.Info("Apache include file creation failed", "path", targetFile, "error", writeErr)
				}
			}
			os.MkdirAll("/run/httpd", 0o755)
			s.exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"enable", "--now", "httpd"}})
			completedSteps = append(completedSteps, "backend:apache")
		}
		if opts.Backend == "ols" {
			// OLS install via repo (best-effort, rpm --nodeps if needed)
			os.MkdirAll("/usr/local/lsws/conf/vhosts", 0o755)
			os.MkdirAll(s.cfg.OLS.ManagedListenersDir, 0o755)
			os.MkdirAll(filepath.Join(s.cfg.Paths.StateDir, "parity"), 0o755)
			completedSteps = append(completedSteps, "backend:ols")
		}
	}

	phpVersions := uniqueVersions(opts.PHPVersion, opts.PHPVersions)
	for _, version := range phpVersions {
		manager := phpruntime.NewManager(s.cfg, s.logger, s.exec)
		p, err := manager.Install(ctx, phpruntime.InstallOptions{
			Version:      version,
			Profile:      phpruntime.ProfileGeneric,
			DryRun:       opts.DryRun,
			PlanOnly:     opts.PlanOnly,
			IncludeFPM:   true,
			IncludeLSAPI: true,
		})
		if err != nil {
			return plan.Plan{}, wrapError("php:"+version, err)
		}
		appendPlan(p)
		if !opts.DryRun && !opts.PlanOnly {
			completedSteps = append(completedSteps, "php:"+version)
		}
	}

	if opts.DBProvider != "" {
		manager := db.NewManager(s.cfg, s.logger, s.exec)
		p, err := manager.Install(ctx, db.InstallOptions{
			Provider: db.ProviderName(opts.DBProvider),
			TLSMode:  db.TLSMode(opts.DBTLS),
			DryRun:   opts.DryRun,
			PlanOnly: opts.PlanOnly,
		})
		if err != nil {
			return plan.Plan{}, wrapError("db:"+opts.DBProvider, err)
		}
		appendPlan(p)
		if !opts.DryRun && !opts.PlanOnly {
			completedSteps = append(completedSteps, "db:"+opts.DBProvider)
		}
	}

	if opts.WithMemcached {
		manager := cache.NewManager(s.cfg, s.logger, s.exec)
		p, err := manager.Install(ctx, cache.InstallOptions{
			Provider: cache.ProviderMemcached,
			DryRun:   opts.DryRun,
			PlanOnly: opts.PlanOnly,
		})
		if err != nil {
			return plan.Plan{}, wrapError("cache:memcached", err)
		}
		appendPlan(p)
		if !opts.DryRun && !opts.PlanOnly {
			completedSteps = append(completedSteps, "cache:memcached")
		}
	}
	if opts.WithRedis {
		manager := cache.NewManager(s.cfg, s.logger, s.exec)
		p, err := manager.Install(ctx, cache.InstallOptions{
			Provider: cache.ProviderRedis,
			DryRun:   opts.DryRun,
			PlanOnly: opts.PlanOnly,
		})
		if err != nil {
			return plan.Plan{}, wrapError("cache:redis", err)
		}
		appendPlan(p)
		if !opts.DryRun && !opts.PlanOnly {
			completedSteps = append(completedSteps, "cache:redis")
		}
	}

	if opts.Site != "" {
		siteSpec := model.Site{
			Name:    opts.Site,
			Backend: firstNonEmpty(opts.Backend, "apache"),
			Domain: model.DomainBinding{
				ServerName: opts.Site,
			},
			PHP: model.PHPRuntimeBinding{
				Enabled: opts.SiteProfile != site.ProfileStatic && opts.SiteProfile != site.ProfileReverseProxy,
				Version: firstNonEmpty(opts.PHPVersion, "8.3"),
			},
		}
		if err := site.ApplyProfile(&siteSpec, firstNonEmpty(opts.SiteProfile, site.ProfileGeneric), opts.SiteUpstream); err != nil {
			return plan.Plan{}, wrapError("site:"+opts.Site, err)
		}
		manager := site.NewManager(s.cfg, s.logger, s.exec)
		p, err := manager.Create(ctx, site.CreateOptions{
			Site:       siteSpec,
			DryRun:     opts.DryRun,
			PlanOnly:   opts.PlanOnly,
			SkipReload: opts.DryRun || opts.PlanOnly,
		})
		if err != nil {
			return plan.Plan{}, wrapError("site:"+opts.Site, err)
		}
		appendPlan(p)
		if !opts.DryRun && !opts.PlanOnly {
			completedSteps = append(completedSteps, "site:"+opts.Site)
		}
	}

	out.Warnings = uniqueStrings(out.Warnings)
	return out, nil
}

func uniqueVersions(primary string, extra []string) []string {
	versions := make([]string, 0, len(extra)+1)
	if strings.TrimSpace(primary) != "" {
		versions = append(versions, primary)
	}
	versions = append(versions, extra...)
	return uniqueStrings(versions)
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
	slices.Sort(out)
	return out
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
