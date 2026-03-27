package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	installsvc "github.com/web-casa/llstack/internal/install"
	"github.com/web-casa/llstack/internal/site"
	"github.com/web-casa/llstack/internal/system"
)

type installOptions struct {
	Backend        string
	PHPVersion     string
	PHPVersions    []string
	DB             string
	DBTLS          string
	WithMemcached  bool
	WithRedis      bool
	Site           string
	Email          string
	NonInteractive bool
	DryRun         bool
	PlanOnly       bool
	JSON           bool
	SiteProfile    string
}

func (r *Root) newInstallCommand() *cobra.Command {
	opts := installOptions{}
	var phpVersions string
	var configPath string

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Plan or apply LLStack installation",
		Long: strings.TrimSpace(`
Build or apply an installation plan for LLStack. Input can come from CLI flags,
or from --config YAML/JSON and then selectively overridden by CLI flags.
`),
		Example: strings.TrimSpace(`
  llstack install --backend apache --php_version 8.3 --db mariadb --site example.com --dry-run
  llstack install --config examples/install/basic.yaml --plan-only
  llstack install --config /etc/llstack/install.yaml --backend apache
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no backend/config specified and not non-interactive, launch interactive wizard
			if !cmd.Flags().Changed("backend") && configPath == "" && !opts.NonInteractive && !opts.DryRun {
				return r.runInteractiveInstall(cmd)
			}

			profile := installsvc.Profile{}
			if strings.TrimSpace(configPath) != "" {
				loaded, err := installsvc.LoadProfile(configPath)
				if err != nil {
					return err
				}
				profile = loaded
			}
			if cmd.Flags().Changed("backend") {
				profile.Backend = opts.Backend
			}
			if cmd.Flags().Changed("php_version") {
				profile.PHPVersion = opts.PHPVersion
			}
			if cmd.Flags().Changed("php_versions") {
				profile.PHPVersions = splitCSV(phpVersions)
			}
			if cmd.Flags().Changed("db") {
				profile.DB = opts.DB
			}
			if cmd.Flags().Changed("db_tls") {
				profile.DBTLS = opts.DBTLS
			}
			if cmd.Flags().Changed("with_memcached") {
				profile.WithMemcached = opts.WithMemcached
			}
			if cmd.Flags().Changed("with_redis") {
				profile.WithRedis = opts.WithRedis
			}
			if cmd.Flags().Changed("site") {
				profile.Site = opts.Site
			}
			if cmd.Flags().Changed("email") {
				profile.Email = opts.Email
			}
			if cmd.Flags().Changed("site_profile") {
				profile.SiteProfile = opts.SiteProfile
			}
			if cmd.Flags().Changed("non-interactive") {
				profile.NonInteractive = opts.NonInteractive
			}
			if cmd.Flags().Changed("dry-run") {
				profile.DryRun = opts.DryRun
			}
			if cmd.Flags().Changed("plan-only") {
				profile.PlanOnly = opts.PlanOnly
			}

			serviceOpts := profile.ToOptions()
			if serviceOpts.SiteProfile == "" && serviceOpts.Site != "" {
				serviceOpts.SiteProfile = site.ProfileGeneric
			}
			service := installsvc.NewService(r.Config, r.Logger, r.Exec)
			p, err := service.Execute(cmd.Context(), serviceOpts)
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(r.Stdout, p)
			}
			return writePlanText(r.Stdout, p)
		},
	}

	cmd.Flags().StringVar(&opts.Backend, "backend", "", "Backend to use: apache|ols|lsws")
	cmd.Flags().StringVar(&opts.PHPVersion, "php_version", "", "Primary PHP version")
	cmd.Flags().StringVar(&phpVersions, "php_versions", "", "Comma-separated PHP versions")
	cmd.Flags().StringVar(&opts.DB, "db", "", "Database provider: mariadb|mysql|postgresql|percona")
	cmd.Flags().StringVar(&opts.DBTLS, "db_tls", "disabled", "Database TLS policy: enabled|disabled|required")
	cmd.Flags().BoolVar(&opts.WithMemcached, "with_memcached", false, "Include Memcached")
	cmd.Flags().BoolVar(&opts.WithRedis, "with_redis", false, "Include Redis")
	cmd.Flags().StringVar(&opts.Site, "site", "", "Create the first site")
	cmd.Flags().StringVar(&opts.Email, "email", "", "Primary administrator email")
	cmd.Flags().StringVar(&opts.SiteProfile, "site_profile", "", "Site profile: generic|wordpress|laravel|static|reverse-proxy")
	cmd.Flags().StringVar(&configPath, "config", "", "Load install options from a YAML or JSON file")
	cmd.Flags().BoolVar(&opts.NonInteractive, "non-interactive", false, "Disable interactive prompts")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Preview without applying changes")
	cmd.Flags().BoolVar(&opts.PlanOnly, "plan-only", false, "Print the plan and exit")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) runInteractiveInstall(cmd *cobra.Command) error {
	result, err := installsvc.RunInteractive(r.Stdin, r.Stdout)
	if err != nil {
		return err
	}
	if !result.Confirmed {
		fmt.Fprintln(r.Stdout, "Installation cancelled.")
		return nil
	}

	fmt.Fprintln(r.Stdout, "")
	fmt.Fprintln(r.Stdout, "Installing...")

	// Build service options from interactive result
	serviceOpts := installsvc.Options{
		Backend:       result.Backend,
		PHPVersion:    result.PHPVersion,
		PHPVersions:   result.ExtraPHP,
		DBProvider:    result.DBProvider,
		WithMemcached: result.WithMemcached,
		WithRedis:     result.WithRedis || result.WithValkey,
	}

	service := installsvc.NewService(r.Config, r.Logger, r.Exec)
	p, err := service.Execute(cmd.Context(), serviceOpts)
	if err != nil {
		return fmt.Errorf("install failed: %w", err)
	}

	fmt.Fprintf(r.Stdout, "  [✓] Core installation complete (%d operations)\n", len(p.Operations))

	// Setup welcome page
	serverIP := installsvc.DetectServerIP(cmd.Context(), r.Exec)
	if err := installsvc.SetupWelcomePage(cmd.Context(), r.Exec, installsvc.WelcomePageConfig{
		SitesRoot:  r.Config.Paths.SitesRootDir,
		Backend:    result.Backend,
		PHPVersion: result.PHPVersion,
		ServerIP:   serverIP,
	}); err != nil {
		fmt.Fprintf(r.Stdout, "  [!] Welcome page setup skipped: %s\n", err)
	} else {
		fmt.Fprintln(r.Stdout, "  [✓] Welcome page installed")
	}

	// Install fail2ban in monitor mode
	if result.EnableFail2ban {
		r.Exec.Run(cmd.Context(), system.Command{Name: "dnf", Args: []string{"-y", "install", "fail2ban"}})
		// Monitor-only config: no banning by default
		monitorJail := "[DEFAULT]\nbanaction = \nbanaction_allports = \n\n[sshd]\nenabled = true\naction = %(action_)s\n"
		os.MkdirAll("/etc/fail2ban/jail.d", 0o755)
		os.WriteFile("/etc/fail2ban/jail.d/llstack-monitor.conf", []byte(monitorJail), 0o644)
		r.Exec.Run(cmd.Context(), system.Command{Name: "systemctl", Args: []string{"enable", "--now", "fail2ban"}})
		fmt.Fprintln(r.Stdout, "  [✓] fail2ban installed (monitor mode, no blocking)")
	}

	// Print final summary
	fmt.Fprintln(r.Stdout, "")
	fmt.Fprintln(r.Stdout, "═══════════════════════════════════════")
	fmt.Fprintln(r.Stdout, "  Installation complete!")
	fmt.Fprintln(r.Stdout, "")
	if serverIP != "" {
		fmt.Fprintf(r.Stdout, "  Web: http://%s\n", serverIP)
	}
	fmt.Fprintln(r.Stdout, "")

	// Show DB password
	if result.DBProvider != "" && result.DBRootPass != "" {
		fmt.Fprintf(r.Stdout, "  Database: %s\n", result.DBProvider)
		fmt.Fprintf(r.Stdout, "  Root password: %s\n", result.DBRootPass)
		if result.DBPassAuto {
			fmt.Fprintln(r.Stdout, "  (auto-generated, save this password!)")
		}
		fmt.Fprintln(r.Stdout, "")
	}

	fmt.Fprintln(r.Stdout, "  Next steps:")
	fmt.Fprintln(r.Stdout, "    llstack site:create example.com    Add a site")
	fmt.Fprintln(r.Stdout, "    llstack tui                        Open TUI interface")
	fmt.Fprintln(r.Stdout, "    llstack doctor                     Run diagnostics")
	fmt.Fprintln(r.Stdout, "    llstack --help                     Show all commands")
	fmt.Fprintln(r.Stdout, "")
	fmt.Fprintln(r.Stdout, "  ⚠ Remember to remove the welcome page after verification:")
	fmt.Fprintln(r.Stdout, "    llstack welcome:remove")
	fmt.Fprintln(r.Stdout, "═══════════════════════════════════════")

	return nil
}

func (r *Root) newWelcomeRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "welcome:remove",
		Short: "Remove the default welcome page, PHP probe, and Adminer",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := installsvc.RemoveWelcomePage(cmd.Context(), r.Exec, r.Config.Paths.SitesRootDir, "apache"); err != nil {
				return err
			}
			fmt.Fprintln(r.Stdout, "Welcome page, PHP probe, and Adminer removed.")
			fmt.Fprintln(r.Stdout, "Default vhost configuration cleaned up.")
			return nil
		},
	}
}

func splitCSV(input string) []string {
	if input == "" {
		return nil
	}

	raw := strings.Split(input, ",")
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		values = append(values, trimmed)
	}

	return values
}
