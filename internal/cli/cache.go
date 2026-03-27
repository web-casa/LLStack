package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/web-casa/llstack/internal/cache"
)

func (r *Root) newCacheInstallCommand() *cobra.Command {
	var dryRun bool
	var planOnly bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "cache:install <provider>",
		Short: "Install a managed cache provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := cache.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.Install(cmd.Context(), cache.InstallOptions{
				Provider: cache.ProviderName(args[0]),
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

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying changes")
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Print the plan and exit")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newCacheStatusCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "cache:status",
		Short: "List managed cache providers",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := cache.NewManager(r.Config, r.Logger, r.Exec)
			manifests, err := manager.Status()
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, manifests)
			}
			if len(manifests) == 0 {
				_, err := fmt.Fprintln(r.Stdout, "no managed cache providers\nhint: run `llstack cache:install memcached --dry-run` to preview the first cache install")
				return err
			}
			for _, manifest := range manifests {
				if _, err := fmt.Fprintf(r.Stdout, "%s\t%s\t%s\t%dMB\t%s\n", manifest.Provider, manifest.ServiceName, manifest.Status, manifest.MaxMemoryMB, manifest.ConfigPath); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newCacheConfigureCommand() *cobra.Command {
	var bind string
	var port int
	var maxMemoryMB int
	var dryRun bool
	var planOnly bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "cache:configure <provider>",
		Short: "Configure a managed cache provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := cache.NewManager(r.Config, r.Logger, r.Exec)
			p, err := manager.Configure(cmd.Context(), cache.ConfigureOptions{
				Provider:    cache.ProviderName(args[0]),
				Bind:        bind,
				Port:        port,
				MaxMemoryMB: maxMemoryMB,
				DryRun:      dryRun,
				PlanOnly:    planOnly,
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

	cmd.Flags().StringVar(&bind, "bind", "127.0.0.1", "Bind address")
	cmd.Flags().IntVar(&port, "port", 0, "Listen port")
	cmd.Flags().IntVar(&maxMemoryMB, "max-memory", 256, "Max memory in MB")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying changes")
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Print the plan and exit")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}
