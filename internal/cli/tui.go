package cli

import (
	"github.com/spf13/cobra"

	"github.com/web-casa/llstack/internal/tui"
)

func (r *Root) newTUICommand() *cobra.Command {
	return &cobra.Command{
		Use:     "tui",
		Short:   "Launch the interactive TUI",
		Long:    "Launch the interactive terminal interface for install, site, runtime, doctor, and history workflows.",
		Example: "  llstack tui",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run(r.Version, r.Config, r.Logger, r.Exec)
		},
	}
}
