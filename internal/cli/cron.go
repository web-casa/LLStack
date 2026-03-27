package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/web-casa/llstack/internal/cron"
	"github.com/web-casa/llstack/internal/system"
)

func (r *Root) newCronAddCommand() *cobra.Command {
	var schedule, command, preset, user string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "cron:add <site>",
		Short: "Add a cron job for a site",
		Long:  "Add a custom cron job or use --preset (wp-cron, laravel-scheduler)",
		Args:  cobra.ExactArgs(1),
		Example: `  llstack cron:add wp.example.com --schedule "*/5 * * * *" --command "cd /data/www/wp.example.com && php wp-cron.php"
  llstack cron:add wp.example.com --preset wp-cron
  llstack cron:add laravel.example.com --preset laravel-scheduler`,
		RunE: func(cmd *cobra.Command, args []string) error {
			siteName := args[0]
			if user == "" {
				user = system.SiteUsername(siteName)
			}
			mgr := cron.NewManager("/etc/cron.d", "/etc/llstack/cron")

			var job cron.Job
			var err error
			if preset != "" {
				// Derive docroot from site name using default convention
				docroot := fmt.Sprintf("/data/www/%s", siteName)
				job, err = mgr.AddPreset(siteName, user, docroot, preset)
			} else {
				if schedule == "" || command == "" {
					return fmt.Errorf("--schedule and --command are required (or use --preset)")
				}
				job, err = mgr.Add(siteName, user, schedule, command, "")
			}
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, job)
			}
			fmt.Fprintf(r.Stdout, "cron job added: id=%s schedule=%q\n", job.ID, job.Schedule)
			return nil
		},
	}
	cmd.Flags().StringVar(&schedule, "schedule", "", "Cron schedule (e.g. '*/5 * * * *')")
	cmd.Flags().StringVar(&command, "command", "", "Command to execute")
	cmd.Flags().StringVar(&preset, "preset", "", "Use preset: wp-cron, laravel-scheduler")
	cmd.Flags().StringVar(&user, "user", "", "Linux user to run as (default: site user)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newCronListCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "cron:list [site]",
		Short: "List cron jobs for a site (or all sites)",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := cron.NewManager("/etc/cron.d", "/etc/llstack/cron")
			site := ""
			if len(args) > 0 {
				site = args[0]
			}
			jobs, err := mgr.List(site)
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, jobs)
			}
			if len(jobs) == 0 {
				fmt.Fprintln(r.Stdout, "no cron jobs found")
				return nil
			}
			for _, job := range jobs {
				fmt.Fprintf(r.Stdout, "%-10s  %-25s  %s  %s\n", job.ID, job.Site, job.Schedule, job.Command)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newCronRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cron:remove <site> <job-id>",
		Short: "Remove a cron job",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := cron.NewManager("/etc/cron.d", "/etc/llstack/cron")
			if err := mgr.Remove(args[0], args[1]); err != nil {
				return err
			}
			fmt.Fprintf(r.Stdout, "cron job %s removed from %s\n", args[1], args[0])
			return nil
		},
	}
	return cmd
}
