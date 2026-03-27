package cli

import (
	"github.com/spf13/cobra"

	"github.com/web-casa/llstack/internal/doctor"
)

func (r *Root) newRepairCommand() *cobra.Command {
	var dryRun bool
	var planOnly bool
	var jsonOutput bool
	var skipReload bool

	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Repair missing LLStack-managed directories and site assets",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := doctor.NewService(r.Config, r.Logger, r.Exec).Repair(cmd.Context(), doctor.RepairOptions{
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
	cmd.Flags().BoolVar(&skipReload, "skip-reload", false, "Skip backend configtest/reload during site reconciliation")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}
