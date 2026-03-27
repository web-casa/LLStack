package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	phpruntime "github.com/web-casa/llstack/internal/php"
	"github.com/web-casa/llstack/internal/site"
)

func (r *Root) newPHPListCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "php:list",
		Short: "List managed PHP runtimes",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := phpruntime.NewManager(r.Config, r.Logger, r.Exec)
			runtimes, err := manager.List()
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, runtimes)
			}
			if len(runtimes) == 0 {
				_, err := fmt.Fprintln(r.Stdout, "no managed php runtimes\nhint: run `llstack php:install 8.3 --dry-run` to preview the first runtime install")
				return err
			}
			for _, runtime := range runtimes {
				if _, err := fmt.Fprintf(r.Stdout, "%s\t%s\t%s\n", runtime.Version, runtime.Profile, strings.Join(runtime.Extensions, ",")); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newPHPInstallCommand() *cobra.Command {
	var extensions string
	var profile string
	var includeFPM bool
	var includeLSAPI bool
	var dryRun bool
	var planOnly bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "php:install <version>",
		Short: "Install a managed PHP runtime",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := phpruntime.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.Install(cmd.Context(), phpruntime.InstallOptions{
				Version:      args[0],
				Extensions:   splitCSV(extensions),
				Profile:      phpruntime.ProfileName(profile),
				DryRun:       dryRun,
				PlanOnly:     planOnly,
				IncludeFPM:   includeFPM,
				IncludeLSAPI: includeLSAPI,
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

	cmd.Flags().StringVar(&extensions, "extensions", "", "Comma-separated extension names")
	cmd.Flags().StringVar(&profile, "profile", string(phpruntime.ProfileGeneric), "php.ini profile: generic|wp|laravel|api|custom")
	cmd.Flags().BoolVar(&includeFPM, "with-fpm", true, "Install php-fpm packages")
	cmd.Flags().BoolVar(&includeLSAPI, "with-litespeed", true, "Install php-litespeed packages")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying changes")
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Print the plan and exit")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newPHPExtensionsCommand() *cobra.Command {
	var install string
	var dryRun bool
	var planOnly bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "php:extensions <version>",
		Short: "Install or inspect extensions for a managed PHP runtime",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := phpruntime.NewManager(r.Config, r.Logger, r.Exec)
			if install == "" {
				runtimes, err := manager.List()
				if err != nil {
					return err
				}
				for _, runtime := range runtimes {
					if runtime.Version == args[0] {
						if jsonOutput {
							return writeJSON(r.Stdout, runtime.Extensions)
						}
						_, err := fmt.Fprintln(r.Stdout, strings.Join(runtime.Extensions, ","))
						return err
					}
				}
				return fmt.Errorf("runtime %s not found", args[0])
			}

			p, err := manager.ConfigureExtensions(cmd.Context(), phpruntime.ExtensionsOptions{
				Version:    args[0],
				Extensions: splitCSV(install),
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

	cmd.Flags().StringVar(&install, "install", "", "Comma-separated extension names to install")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying changes")
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Print the plan and exit")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newPHPINICommand() *cobra.Command {
	var profile string
	var dryRun bool
	var planOnly bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "php:ini <version>",
		Short: "Apply a managed php.ini profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := phpruntime.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.ApplyProfile(cmd.Context(), phpruntime.ProfileOptions{
				Version:  args[0],
				Profile:  phpruntime.ProfileName(profile),
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

	cmd.Flags().StringVar(&profile, "profile", string(phpruntime.ProfileGeneric), "php.ini profile: generic|wp|laravel|api|custom")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying changes")
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Print the plan and exit")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newSitePHPCommand() *cobra.Command {
	var version string
	var dryRun bool
	var planOnly bool
	var jsonOutput bool
	var skipReload bool

	cmd := &cobra.Command{
		Use:   "site:php <server-name>",
		Short: "Switch the PHP runtime bound to a managed site",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if version == "" {
				return fmt.Errorf("--version is required")
			}
			manager := site.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.UpdatePHPVersion(cmd.Context(), site.UpdatePHPOptions{
				Name:       args[0],
				Version:    version,
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

	cmd.Flags().StringVar(&version, "version", "", "Target PHP version")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying changes")
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Print the plan and exit")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	cmd.Flags().BoolVar(&skipReload, "skip-reload", false, "Skip backend configtest/reload")
	return cmd
}

func (r *Root) newPHPRemoveCommand() *cobra.Command {
	var dryRun bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "php:remove <version>",
		Short:   "Remove a managed PHP runtime",
		Args:    cobra.ExactArgs(1),
		Example: "  llstack php:remove 8.2",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := phpruntime.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.Uninstall(cmd.Context(), phpruntime.UninstallOptions{
				Version: args[0],
				DryRun:  dryRun,
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

func (r *Root) newPHPTuneCommand() *cobra.Command {
	var version string
	var maxChildren, startServers, minSpare, maxSpare int
	var dryRun bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "php:tune",
		Short: "Tune PHP-FPM pool parameters",
		Example: `  llstack php:tune --version 8.3 --max-children 30
  llstack php:tune --version 8.3 --dry-run --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if version == "" {
				return fmt.Errorf("--version is required")
			}
			manager := phpruntime.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.TunePool(cmd.Context(), phpruntime.PoolTuneOptions{
				Version:      version,
				MaxChildren:  maxChildren,
				StartServers: startServers,
				MinSpare:     minSpare,
				MaxSpare:     maxSpare,
				DryRun:       dryRun,
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
	cmd.Flags().StringVar(&version, "version", "", "PHP version to tune")
	cmd.Flags().IntVar(&maxChildren, "max-children", 0, "pm.max_children (0 = default)")
	cmd.Flags().IntVar(&startServers, "start-servers", 0, "pm.start_servers (0 = default)")
	cmd.Flags().IntVar(&minSpare, "min-spare", 0, "pm.min_spare_servers (0 = default)")
	cmd.Flags().IntVar(&maxSpare, "max-spare", 0, "pm.max_spare_servers (0 = default)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}
