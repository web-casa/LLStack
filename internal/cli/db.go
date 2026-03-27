package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/web-casa/llstack/internal/db"
)

func (r *Root) newDBInstallCommand() *cobra.Command {
	var version string
	var tlsMode string
	var dryRun bool
	var planOnly bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "db:install <provider>",
		Short: "Install a managed database provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := db.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.Install(cmd.Context(), db.InstallOptions{
				Provider: db.ProviderName(args[0]),
				Version:  version,
				TLSMode:  db.TLSMode(tlsMode),
				DryRun:   dryRun,
				PlanOnly: planOnly,
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

	cmd.Flags().StringVar(&version, "version", "", "Provider version override")
	cmd.Flags().StringVar(&tlsMode, "tls", string(db.TLSDisabled), "TLS mode: enabled|disabled|required")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying changes")
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Print the plan and exit")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newDBInitCommand() *cobra.Command {
	var provider string
	var adminUser string
	var adminPassword string
	var tlsMode string
	var dryRun bool
	var planOnly bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "db:init",
		Short: "Initialize a managed database provider",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := db.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.Init(cmd.Context(), db.InitOptions{
				Provider:      db.ProviderName(provider),
				AdminUser:     adminUser,
				AdminPassword: adminPassword,
				TLSMode:       db.TLSMode(tlsMode),
				DryRun:        dryRun,
				PlanOnly:      planOnly,
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

	cmd.Flags().StringVar(&provider, "provider", "", "Provider name: mariadb|mysql|postgresql|percona")
	cmd.Flags().StringVar(&adminUser, "admin-user", "llstack_admin", "Managed admin user to create")
	cmd.Flags().StringVar(&adminPassword, "admin-password", "", "Managed admin password")
	cmd.Flags().StringVar(&tlsMode, "tls", string(db.TLSDisabled), "TLS mode: enabled|disabled|required")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying changes")
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Print the plan and exit")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newDBCreateCommand() *cobra.Command {
	var provider string
	var owner string
	var encoding string
	var dryRun bool
	var planOnly bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "db:create <database>",
		Short: "Create a managed database",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := db.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.CreateDatabase(cmd.Context(), db.CreateDatabaseOptions{
				Provider: db.ProviderName(provider),
				Name:     args[0],
				Owner:    owner,
				Encoding: encoding,
				DryRun:   dryRun,
				PlanOnly: planOnly,
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

	cmd.Flags().StringVar(&provider, "provider", "", "Provider name: mariadb|mysql|postgresql|percona")
	cmd.Flags().StringVar(&owner, "owner", "", "Owner role/user")
	cmd.Flags().StringVar(&encoding, "encoding", "UTF8", "Database encoding")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying changes")
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Print the plan and exit")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newDBUserCreateCommand() *cobra.Command {
	var provider string
	var password string
	var database string
	var privileges string
	var tlsMode string
	var dryRun bool
	var planOnly bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "db:user:create <user>",
		Short: "Create a managed database user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := db.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.CreateUser(cmd.Context(), db.CreateUserOptions{
				Provider:   db.ProviderName(provider),
				Name:       args[0],
				Password:   password,
				Database:   database,
				Privileges: privileges,
				TLSMode:    db.TLSMode(tlsMode),
				DryRun:     dryRun,
				PlanOnly:   planOnly,
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

	cmd.Flags().StringVar(&provider, "provider", "", "Provider name: mariadb|mysql|postgresql|percona")
	cmd.Flags().StringVar(&password, "password", "", "User password")
	cmd.Flags().StringVar(&database, "database", "", "Database to grant against")
	cmd.Flags().StringVar(&privileges, "privileges", "ALL PRIVILEGES", "Privileges to grant")
	cmd.Flags().StringVar(&tlsMode, "tls", "", "TLS mode override: enabled|disabled|required")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying changes")
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Print the plan and exit")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newDBListCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "db:list",
		Short: "List managed database providers",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := db.NewManager(r.Config, r.Logger, r.Exec)
			manifests, err := manager.List()
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, manifests)
			}
			if len(manifests) == 0 {
				_, err := fmt.Fprintln(r.Stdout, "no managed database providers\nhint: run `llstack db:install mariadb --dry-run` to preview the first provider install")
				return err
			}
			for _, manifest := range manifests {
				if _, err := fmt.Fprintf(r.Stdout, "%s\t%s\t%s\t%s\n", manifest.Provider, manifest.ServiceName, manifest.Status, manifest.TLS.Mode); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func dbProviderDetails(r *Root, provider string, version string, tlsMode string) (map[string]string, []string) {
	spec, err := db.ResolveProvider(r.Config, db.ProviderName(provider), version)
	if err != nil {
		return nil, nil
	}
	details := map[string]string{
		"service":  spec.ServiceName,
		"packages": strings.Join(spec.Packages, " "),
	}
	if tlsMode != "" {
		details["tls_mode"] = tlsMode
	}
	return details, spec.Warnings
}

func (r *Root) newDBRemoveCommand() *cobra.Command {
	var dryRun bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "db:remove <provider>",
		Short:   "Remove a managed database provider",
		Args:    cobra.ExactArgs(1),
		Example: "  llstack db:remove mariadb",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := db.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.Uninstall(cmd.Context(), db.UninstallOptions{
				Provider: db.ProviderName(args[0]),
				DryRun:   dryRun,
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
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newDBBackupCommand() *cobra.Command {
	var outputDir string
	var dryRun bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "db:backup <provider>",
		Short:   "Backup a managed database provider",
		Args:    cobra.ExactArgs(1),
		Example: "  llstack db:backup mariadb\n  llstack db:backup postgresql --output-dir /backups",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := db.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.Backup(cmd.Context(), db.BackupOptions{
				Provider:  db.ProviderName(args[0]),
				OutputDir: outputDir,
				DryRun:    dryRun,
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
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory for backup file")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newDBTuneCommand() *cobra.Command {
	var force, dryRun, jsonOutput bool

	cmd := &cobra.Command{
		Use:     "db:tune <provider>",
		Short:   "Apply hardware-aware database parameter tuning",
		Args:    cobra.ExactArgs(1),
		Example: "  llstack db:tune mariadb\n  llstack db:tune postgresql --dry-run --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := db.NewManager(r.Config, r.Logger, r.Exec)
			p, result, err := manager.Tune(cmd.Context(), db.TuneOptions{
				Provider: db.ProviderName(args[0]),
				DryRun:   dryRun,
				Force:    force,
			})
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, result)
			}
			fmt.Fprintf(r.Stdout, "Provider: %s\nConfig: %s\n\nParameters:\n", result.Provider, result.ConfigPath)
			for k, v := range result.Parameters {
				fmt.Fprintf(r.Stdout, "  %-35s = %s\n", k, v)
			}
			if dryRun {
				fmt.Fprintln(r.Stdout, "\n(dry-run, no files written)")
			}
			_ = p
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing tuning config")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}
