package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/web-casa/llstack/internal/core/plan"
)

func (r *Root) newInitCommand() *cobra.Command {
	var jsonOutput bool
	var dryRun bool
	var planOnly bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Prepare LLStack state and configuration directories",
		RunE: func(cmd *cobra.Command, args []string) error {
			p := plan.New("init", "Prepare LLStack directories and local state")
			p.DryRun = dryRun
			p.PlanOnly = planOnly
			p.Warnings = []string{"Phase 1 does not apply changes yet; this command previews the intended bootstrap actions."}
			p.AddOperation(plan.Operation{
				ID:     "create-config-dir",
				Kind:   "mkdir",
				Target: r.Config.Paths.ConfigDir,
			})
			p.AddOperation(plan.Operation{
				ID:     "create-state-dir",
				Kind:   "mkdir",
				Target: r.Config.Paths.StateDir,
			})
			p.AddOperation(plan.Operation{
				ID:     "create-history-dir",
				Kind:   "mkdir",
				Target: r.Config.Paths.HistoryDir,
			})
			p.AddOperation(plan.Operation{
				ID:     "create-backups-dir",
				Kind:   "mkdir",
				Target: r.Config.Paths.BackupsDir,
			})
			p.AddOperation(plan.Operation{
				ID:     "create-log-dir",
				Kind:   "mkdir",
				Target: r.Config.Paths.LogDir,
			})

			if jsonOutput {
				return writeJSON(r.Stdout, p)
			}

			if dryRun || planOnly {
				return writePlanText(r.Stdout, p)
			}

			_, err := fmt.Fprintf(r.Stdout, "Phase 1 skeleton: init apply is not implemented yet. Use --plan-only, --dry-run, or --json to inspect the bootstrap plan.\n")
			return err
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying changes")
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Print the plan and exit")
	return cmd
}
