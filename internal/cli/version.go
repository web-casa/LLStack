package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type versionPayload struct {
	Version      string `json:"version"`
	Commit       string `json:"commit"`
	BuildDate    string `json:"build_date"`
	TargetOS     string `json:"target_os"`
	TargetArch   string `json:"target_arch"`
	GoVersion    string `json:"go_version"`
	DefaultSites string `json:"default_sites_root"`
}

func (r *Root) newVersionCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print LLStack version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := versionPayload{
				Version:      r.Version,
				Commit:       r.Build.Commit,
				BuildDate:    r.Build.BuildDate,
				TargetOS:     r.Build.TargetOS,
				TargetArch:   r.Build.TargetArch,
				GoVersion:    r.Build.GoVersion,
				DefaultSites: r.Config.Paths.SitesRootDir,
			}

			if jsonOutput {
				return writeJSON(r.Stdout, payload)
			}

			_, err := fmt.Fprintf(
				r.Stdout,
				"llstack %s\ncommit: %s\nbuild date: %s\ntarget: %s/%s\ngo: %s\nsites root: %s\n",
				payload.Version,
				payload.Commit,
				payload.BuildDate,
				payload.TargetOS,
				payload.TargetArch,
				payload.GoVersion,
				payload.DefaultSites,
			)
			return err
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}
