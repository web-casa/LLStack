package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/rollback"
	"github.com/web-casa/llstack/internal/site"
)

func (r *Root) newSiteCreateCommand() *cobra.Command {
	var backend string
	var profile string
	var upstream string
	var aliases []string
	var docroot string
	var phpVersion string
	var phpSocket string
	var tlsEnabled bool
	var certFile string
	var keyFile string
	var dryRun bool
	var planOnly bool
	var jsonOutput bool
	var nonInteractive bool
	var skipReload bool

	cmd := &cobra.Command{
		Use:   "site:create [server-name]",
		Short: "Create a managed site",
		Long: strings.TrimSpace(`
Create a managed site from LLStack's canonical site model and render it to the
selected backend. Interactive prompts are used unless --non-interactive is set.
`),
		Example: strings.TrimSpace(`
  llstack site:create example.com --backend apache --profile wordpress --dry-run
  llstack site:create static.example.com --profile static --non-interactive
  llstack site:create proxy.example.com --profile reverse-proxy --upstream http://127.0.0.1:8080
`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			siteSpec, err := r.collectSiteInput(cmd.InOrStdin(), siteInputOptions{
				Backend:         backend,
				Profile:         profile,
				Upstream:        upstream,
				ServerName:      firstArg(args),
				Aliases:         aliases,
				Docroot:         docroot,
				PHPVersion:      phpVersion,
				PHPSocket:       phpSocket,
				TLSEnabled:      tlsEnabled,
				CertificateFile: certFile,
				CertificateKey:  keyFile,
				NonInteractive:  nonInteractive,
			})
			if err != nil {
				return err
			}

			manager := site.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.Create(cmd.Context(), site.CreateOptions{
				Site:       siteSpec,
				DryRun:     dryRun,
				PlanOnly:   planOnly,
				SkipReload: skipReload,
			})
			if err != nil {
				return err
			}

			if jsonOutput {
				return writeJSON(r.Stdout, p)
			}
			return writePlanText(r.Stdout, p)
		},
	}

	cmd.Flags().StringVar(&backend, "backend", "apache", "Backend to use: apache|ols|lsws")
	cmd.Flags().StringVar(&profile, "profile", site.ProfileGeneric, "Deploy profile: generic|wordpress|laravel|static|reverse-proxy")
	cmd.Flags().StringVar(&upstream, "upstream", "", "Reverse proxy upstream URL")
	cmd.Flags().StringSliceVar(&aliases, "alias", nil, "Additional server aliases")
	cmd.Flags().StringVar(&docroot, "docroot", "", "Document root")
	cmd.Flags().StringVar(&phpVersion, "php-version", "", "Site PHP version label")
	cmd.Flags().StringVar(&phpSocket, "php-socket", "", "php-fpm unix socket")
	cmd.Flags().BoolVar(&tlsEnabled, "tls", false, "Enable TLS")
	cmd.Flags().StringVar(&certFile, "cert-file", "", "TLS certificate file")
	cmd.Flags().StringVar(&keyFile, "key-file", "", "TLS certificate key")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying changes")
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Print the plan and exit")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Disable prompts and require explicit flags")
	cmd.Flags().BoolVar(&skipReload, "skip-reload", false, "Skip apache configtest/reload")
	return cmd
}

func (r *Root) newSiteListCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "site:list",
		Short: "List managed sites",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := site.NewManager(r.Config, r.Logger, r.Exec)
			sites, err := manager.List()
			if err != nil {
				return err
			}

			if jsonOutput {
				return writeJSON(r.Stdout, sites)
			}
			if len(sites) == 0 {
				_, err := fmt.Fprintln(r.Stdout, "no managed sites\nhint: run `llstack site:create <server-name> --dry-run` or use `llstack install --site <server-name> --dry-run`")
				return err
			}
			for _, site := range sites {
				licenseMode := "-"
				if site.Capabilities != nil && site.Capabilities.LicenseMode != "" {
					licenseMode = site.Capabilities.LicenseMode
				}
				if _, err := fmt.Fprintf(r.Stdout, "%s\t%s\t%s\t%s\t%s\t%s\n", site.Site.Name, site.Site.Backend, site.Site.State, site.Site.Domain.ServerName, site.Site.DocumentRoot, licenseMode); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newSiteShowCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "site:show <server-name>",
		Short: "Show a managed site manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := site.NewManager(r.Config, r.Logger, r.Exec)
			manifest, err := manager.Show(args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, manifest)
			}
			_, err = fmt.Fprintf(r.Stdout, "name: %s\nbackend: %s\nstate: %s\nprofile: %s\ndocroot: %s\nserver_name: %s\ntls: %t\nphp: %s\n", manifest.Site.Name, manifest.Site.Backend, manifest.Site.State, manifest.Site.Profile, manifest.Site.DocumentRoot, manifest.Site.Domain.ServerName, manifest.Site.TLS.Enabled, manifest.Site.PHP.Version)
			return err
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newSiteUpdateCommand() *cobra.Command {
	var docroot string
	var aliases []string
	var indexFiles []string
	var upstream string
	var dryRun bool
	var planOnly bool
	var jsonOutput bool
	var skipReload bool

	cmd := &cobra.Command{
		Use:   "site:update <server-name>",
		Short: "Update editable settings for a managed site",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := site.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.UpdateSettings(cmd.Context(), site.UpdateSettingsOptions{
				Name:         args[0],
				DocumentRoot: docroot,
				Aliases:      aliases,
				IndexFiles:   indexFiles,
				Upstream:     upstream,
				DryRun:       dryRun,
				PlanOnly:     planOnly,
				SkipReload:   skipReload,
			})
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, p)
			}
			return writePlanText(r.Stdout, p)
		},
	}

	cmd.Flags().StringVar(&docroot, "docroot", "", "New document root")
	cmd.Flags().StringSliceVar(&aliases, "alias", nil, "Replacement alias list")
	cmd.Flags().StringSliceVar(&indexFiles, "index", nil, "Replacement index file list")
	cmd.Flags().StringVar(&upstream, "upstream", "", "New reverse proxy upstream")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying changes")
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Print the plan and exit")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	cmd.Flags().BoolVar(&skipReload, "skip-reload", false, "Skip backend configtest/reload")
	return cmd
}

func (r *Root) newSiteDeleteCommand() *cobra.Command {
	var dryRun bool
	var planOnly bool
	var jsonOutput bool
	var purgeRoot bool
	var skipReload bool

	cmd := &cobra.Command{
		Use:   "site:delete <server-name>",
		Short: "Delete a managed site",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := site.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.Delete(cmd.Context(), site.DeleteOptions{
				Name:       args[0],
				DryRun:     dryRun,
				PlanOnly:   planOnly,
				PurgeRoot:  purgeRoot,
				SkipReload: skipReload,
			})
			if err != nil {
				return err
			}

			if jsonOutput {
				return writeJSON(r.Stdout, p)
			}
			return writePlanText(r.Stdout, p)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying changes")
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Print the plan and exit")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	cmd.Flags().BoolVar(&purgeRoot, "purge-root", false, "Remove the document root")
	cmd.Flags().BoolVar(&skipReload, "skip-reload", false, "Skip apache configtest/reload")
	return cmd
}

func (r *Root) newSiteReloadCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "site:reload <server-name>",
		Short: "Run configtest and reload for a managed site backend",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := site.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.Reload(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, p)
			}
			return writePlanText(r.Stdout, p)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newSiteRestartCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "site:restart <server-name>",
		Short: "Run configtest and restart a managed site backend",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := site.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.Restart(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, p)
			}
			return writePlanText(r.Stdout, p)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newSiteStartCommand() *cobra.Command {
	var jsonOutput bool
	var dryRun bool
	var planOnly bool
	var skipReload bool

	cmd := &cobra.Command{
		Use:   "site:start <server-name>",
		Short: "Enable a managed site and reload the backend",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := site.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.SetState(cmd.Context(), site.StateChangeOptions{
				Name:       args[0],
				State:      "enabled",
				DryRun:     dryRun,
				PlanOnly:   planOnly,
				SkipReload: skipReload,
			})
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, p)
			}
			return writePlanText(r.Stdout, p)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying changes")
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Print the plan and exit")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	cmd.Flags().BoolVar(&skipReload, "skip-reload", false, "Skip backend configtest/reload")
	return cmd
}

func (r *Root) newSiteStopCommand() *cobra.Command {
	var jsonOutput bool
	var dryRun bool
	var planOnly bool
	var skipReload bool

	cmd := &cobra.Command{
		Use:   "site:stop <server-name>",
		Short: "Disable a managed site and reload the backend",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := site.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.SetState(cmd.Context(), site.StateChangeOptions{
				Name:       args[0],
				State:      "disabled",
				DryRun:     dryRun,
				PlanOnly:   planOnly,
				SkipReload: skipReload,
			})
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, p)
			}
			return writePlanText(r.Stdout, p)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying changes")
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Print the plan and exit")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	cmd.Flags().BoolVar(&skipReload, "skip-reload", false, "Skip backend configtest/reload")
	return cmd
}

func (r *Root) newSiteSSLCommand() *cobra.Command {
	var certFile string
	var keyFile string
	var letsEncrypt bool
	var email string
	var dryRun bool
	var planOnly bool
	var jsonOutput bool
	var skipReload bool

	cmd := &cobra.Command{
		Use:   "site:ssl <server-name>",
		Short: "Configure TLS for a managed site",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := "custom"
			if letsEncrypt {
				mode = "letsencrypt"
			}
			manager := site.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.UpdateTLS(cmd.Context(), site.UpdateTLSOptions{
				Name:            args[0],
				Mode:            mode,
				CertificateFile: certFile,
				CertificateKey:  keyFile,
				Email:           email,
				DryRun:          dryRun,
				PlanOnly:        planOnly,
				SkipReload:      skipReload,
			})
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, p)
			}
			return writePlanText(r.Stdout, p)
		},
	}

	cmd.Flags().StringVar(&certFile, "cert-file", "", "TLS certificate file for custom mode")
	cmd.Flags().StringVar(&keyFile, "key-file", "", "TLS certificate key for custom mode")
	cmd.Flags().BoolVar(&letsEncrypt, "letsencrypt", false, "Request and configure a Let's Encrypt certificate")
	cmd.Flags().StringVar(&email, "email", "", "ACME account email")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying changes")
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Print the plan and exit")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	cmd.Flags().BoolVar(&skipReload, "skip-reload", false, "Skip backend configtest/reload")
	return cmd
}

func (r *Root) newSiteLogsCommand() *cobra.Command {
	var kind string
	var lines int

	cmd := &cobra.Command{
		Use:   "site:logs <server-name>",
		Short: "Read recent access or error log lines for a managed site",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := site.NewManager(r.Config, r.Logger, r.Exec)
			content, err := manager.ReadLogs(site.LogReadOptions{
				Name:  args[0],
				Kind:  kind,
				Lines: lines,
			})
			if err != nil {
				return err
			}
			for _, line := range content {
				if _, err := fmt.Fprintln(r.Stdout, line); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&kind, "kind", "access", "Log kind: access|error")
	cmd.Flags().IntVar(&lines, "lines", 20, "Number of trailing lines to print")
	return cmd
}

func (r *Root) newSiteDiffCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "site:diff <server-name>",
		Short: "Preview drift between managed assets and current canonical rendering",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := site.NewManager(r.Config, r.Logger, r.Exec)
			report, err := manager.Diff(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, report)
			}
			if len(report.Entries) == 0 {
				_, err := fmt.Fprintln(r.Stdout, "no drift detected")
				return err
			}
			for _, entry := range report.Entries {
				if _, err := fmt.Fprintf(r.Stdout, "%s\t%s\n%s\n\n", entry.Status, entry.Path, entry.Preview); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newRollbackCommand() *cobra.Command {
	var dryRun bool
	var planOnly bool
	var jsonOutput bool
	var skipReload bool

	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Rollback the latest managed change",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := site.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.RollbackLast(cmd.Context(), site.RollbackOptions{
				DryRun:     dryRun,
				PlanOnly:   planOnly,
				SkipReload: skipReload,
			})
			if err != nil {
				return err
			}

			if jsonOutput {
				return writeJSON(r.Stdout, p)
			}
			return writePlanText(r.Stdout, p)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying changes")
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Print the plan and exit")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	cmd.Flags().BoolVar(&skipReload, "skip-reload", false, "Skip apache configtest/reload")
	cmd.Flags().Bool("last", true, "Rollback the latest operation")
	return cmd
}

func (r *Root) newRollbackListCommand() *cobra.Command {
	var jsonOutput bool
	var limit int

	cmd := &cobra.Command{
		Use:   "rollback:list",
		Short: "List managed rollback history entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := rollback.List(r.Config.Paths.HistoryDir, limit)
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, entries)
			}
			if len(entries) == 0 {
				_, err := fmt.Fprintln(r.Stdout, "no rollback history")
				return err
			}
			latestPending, _ := rollback.LoadLatestPending(r.Config.Paths.HistoryDir)
			for _, entry := range entries {
				state := "rolled-back"
				if !entry.RolledBack {
					state = "pending"
				}
				if latestPending.Path != "" && entry.Path == latestPending.Path {
					state += " latest"
				}
				if _, err := fmt.Fprintf(r.Stdout, "%s\t%s\t%s\t%s\t%s\t%s\n", entry.Timestamp.Format("2006-01-02T15:04:05Z07:00"), entry.ID, entry.Action, entry.Resource, entry.Backend, state); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of history entries to print")
	return cmd
}

func (r *Root) newRollbackShowCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "rollback:show <history-id|filename>",
		Short: "Show a managed rollback history entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, err := rollback.Get(r.Config.Paths.HistoryDir, args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, entry)
			}

			latestPending, _ := rollback.LoadLatestPending(r.Config.Paths.HistoryDir)
			latest := entry.Path != "" && latestPending.Path == entry.Path
			backend := entry.Backend
			if strings.TrimSpace(backend) == "" {
				backend = "(none)"
			}
			if _, err := fmt.Fprintf(
				r.Stdout,
				"id: %s\naction: %s\nresource: %s\nbackend: %s\ntimestamp: %s\nrolled_back: %t\nlatest_pending: %t\npath: %s\nchanges: %d\n",
				entry.ID,
				entry.Action,
				entry.Resource,
				backend,
				entry.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
				entry.RolledBack,
				latest,
				filepath.Base(entry.Path),
				len(entry.Changes),
			); err != nil {
				return err
			}
			for _, change := range entry.Changes {
				if _, err := fmt.Fprintf(r.Stdout, "- %s %s\n", change.Kind, change.Path); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

type siteInputOptions struct {
	Backend         string
	Profile         string
	Upstream        string
	ServerName      string
	Aliases         []string
	Docroot         string
	PHPVersion      string
	PHPSocket       string
	TLSEnabled      bool
	CertificateFile string
	CertificateKey  string
	NonInteractive  bool
}

func (r *Root) collectSiteInput(in io.Reader, opts siteInputOptions) (model.Site, error) {
	spec := model.Site{
		Name:         opts.ServerName,
		Backend:      opts.Backend,
		Profile:      opts.Profile,
		DocumentRoot: opts.Docroot,
		Domain: model.DomainBinding{
			ServerName: opts.ServerName,
			Aliases:    opts.Aliases,
		},
		TLS: model.TLSConfig{
			Enabled:         opts.TLSEnabled,
			Mode:            "custom",
			CertificateFile: opts.CertificateFile,
			CertificateKey:  opts.CertificateKey,
		},
		PHP: model.PHPRuntimeBinding{
			Enabled: opts.PHPVersion != "" || opts.PHPSocket != "",
			Version: opts.PHPVersion,
			Socket:  opts.PHPSocket,
		},
	}

	if opts.NonInteractive {
		if spec.Domain.ServerName == "" {
			return model.Site{}, errors.New("server name is required in non-interactive mode")
		}
		if spec.Backend == "" {
			spec.Backend = "apache"
		}
		if err := site.ApplyProfile(&spec, opts.Profile, opts.Upstream); err != nil {
			return model.Site{}, err
		}
		return spec, nil
	}

	reader := bufio.NewReader(in)
	if spec.Profile == "" {
		value, err := promptDefault(reader, r.Stdout, "Profile", site.ProfileGeneric)
		if err != nil {
			return model.Site{}, err
		}
		spec.Profile = value
	}
	if spec.Backend == "" {
		value, err := promptDefault(reader, r.Stdout, "Backend", "apache")
		if err != nil {
			return model.Site{}, err
		}
		spec.Backend = value
	}
	if spec.Domain.ServerName == "" {
		value, err := prompt(reader, r.Stdout, "Server name")
		if err != nil {
			return model.Site{}, err
		}
		spec.Domain.ServerName = value
		spec.Name = value
	}
	if spec.DocumentRoot == "" {
		value, err := promptDefault(reader, r.Stdout, "Document root", fmt.Sprintf("%s/%s", r.Config.Paths.SitesRootDir, spec.Name))
		if err != nil {
			return model.Site{}, err
		}
		spec.DocumentRoot = value
	}
	if len(spec.Domain.Aliases) == 0 {
		value, err := promptDefault(reader, r.Stdout, "Aliases (comma separated)", "")
		if err != nil {
			return model.Site{}, err
		}
		spec.Domain.Aliases = splitCSV(value)
	}
	if !spec.PHP.Enabled {
		value, err := promptDefault(reader, r.Stdout, "PHP version (leave blank to disable)", "")
		if err != nil {
			return model.Site{}, err
		}
		if value != "" {
			spec.PHP.Enabled = true
			spec.PHP.Version = value
			socket, socketErr := promptDefault(reader, r.Stdout, "php-fpm socket", "/run/php-fpm/www.sock")
			if socketErr != nil {
				return model.Site{}, socketErr
			}
			spec.PHP.Socket = socket
		}
	}
	if !spec.TLS.Enabled {
		value, err := promptDefault(reader, r.Stdout, "Enable TLS? (y/N)", "n")
		if err != nil {
			return model.Site{}, err
		}
		spec.TLS.Enabled = strings.EqualFold(value, "y")
		if spec.TLS.Enabled {
			cert, certErr := prompt(reader, r.Stdout, "Certificate file")
			if certErr != nil {
				return model.Site{}, certErr
			}
			key, keyErr := prompt(reader, r.Stdout, "Certificate key")
			if keyErr != nil {
				return model.Site{}, keyErr
			}
			spec.TLS.CertificateFile = cert
			spec.TLS.CertificateKey = key
		}
	}

	if err := site.ApplyProfile(&spec, spec.Profile, opts.Upstream); err != nil {
		return model.Site{}, err
	}

	return spec, nil
}

func prompt(reader *bufio.Reader, out io.Writer, label string) (string, error) {
	if _, err := fmt.Fprintf(out, "%s: ", label); err != nil {
		return "", err
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func promptDefault(reader *bufio.Reader, out io.Writer, label, fallback string) (string, error) {
	if _, err := fmt.Fprintf(out, "%s [%s]: ", label, fallback); err != nil {
		return "", err
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return fallback, nil
	}
	return line, nil
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}
