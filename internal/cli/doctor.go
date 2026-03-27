package cli

import (
	"fmt"
	"strings"

	"github.com/web-casa/llstack/internal/doctor"
	"github.com/spf13/cobra"
)

func (r *Root) newDoctorCommand() *cobra.Command {
	var jsonOutput bool
	var bundle bool
	var bundlePath string

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run baseline environment diagnostics",
		Example: strings.TrimSpace(`
  llstack doctor
  llstack doctor --bundle --bundle-path /tmp/llstack-doctor.tar.gz
  llstack doctor --json
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			service := doctor.NewService(r.Config, r.Logger, r.Exec)
			payload, err := service.Run(cmd.Context())
			if err != nil {
				return err
			}
			var result any = payload
			if bundle {
				bundleResult, err := service.Bundle(cmd.Context(), bundlePath)
				if err != nil {
					return err
				}
				result = map[string]any{
					"report": payload,
					"bundle": bundleResult,
				}
				if !jsonOutput {
					if _, err := fmt.Fprintf(r.Stdout, "doctor status: %s\n", payload.Status); err != nil {
						return err
					}
					for _, check := range payload.Checks {
						if _, err := fmt.Fprintf(r.Stdout, "- %s: %s (%s)\n", check.Name, check.Status, check.Summary); err != nil {
							return err
						}
					}
					_, err = fmt.Fprintf(r.Stdout, "bundle: %s (%d entries)\n", bundleResult.Path, bundleResult.Entries)
					return err
				}
			}

			if jsonOutput {
				return writeJSON(r.Stdout, result)
			}

			if _, err := fmt.Fprintf(r.Stdout, "doctor status: %s\n", payload.Status); err != nil {
				return err
			}

			for _, check := range payload.Checks {
				if _, err := fmt.Fprintf(r.Stdout, "- %s: %s (%s)\n", check.Name, check.Status, check.Summary); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	cmd.Flags().BoolVar(&bundle, "bundle", false, "Generate a diagnostics bundle alongside the report")
	cmd.Flags().StringVar(&bundlePath, "bundle-path", "", "Write the diagnostics bundle to the given .tar.gz path")
	return cmd
}
