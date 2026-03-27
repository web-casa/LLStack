package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/web-casa/llstack/internal/security"
)

func (r *Root) newFail2banEnableCommand() *cobra.Command {
	var dryRun bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "security:fail2ban",
		Short: "Enable fail2ban with LLStack default jails",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := security.NewManager(r.Exec)
			p, err := mgr.Fail2banEnable(cmd.Context(), dryRun)
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

func (r *Root) newFail2banStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "security:fail2ban-status",
		Short: "Show fail2ban status",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := security.NewManager(r.Exec)
			status, err := mgr.Fail2banStatus(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprintln(r.Stdout, status)
			return nil
		},
	}
	return cmd
}

func (r *Root) newBlockIPCommand() *cobra.Command {
	var dryRun bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "security:block <ip>",
		Short:   "Block an IP address via firewalld",
		Args:    cobra.ExactArgs(1),
		Example: "  llstack security:block 1.2.3.4",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := security.NewManager(r.Exec)
			p, err := mgr.BlockIP(cmd.Context(), args[0], dryRun)
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

func (r *Root) newUnblockIPCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "security:unblock <ip>",
		Short: "Unblock an IP address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := security.NewManager(r.Exec)
			if err := mgr.UnblockIP(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(r.Stdout, "unblocked %s\n", args[0])
			return nil
		},
	}
	return cmd
}

func (r *Root) newBlockListCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "security:blocklist",
		Short: "List blocked IP addresses",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := security.NewManager(r.Exec)
			ips, err := mgr.BlockList(cmd.Context())
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, ips)
			}
			if len(ips) == 0 {
				fmt.Fprintln(r.Stdout, "no blocked IPs")
				return nil
			}
			for _, ip := range ips {
				fmt.Fprintln(r.Stdout, ip)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newRateLimitCommand() *cobra.Command {
	var backend string
	var maxRate int
	var burst int
	var dryRun bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "security:ratelimit",
		Short:   "Enable rate limiting for the web backend",
		Example: "  llstack security:ratelimit --backend apache --max-rate 10 --burst 50",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := security.NewManager(r.Exec)
			p, err := mgr.EnableRateLimit(cmd.Context(), backend, security.RateLimitConfig{
				MaxRequestsPerSecond: maxRate,
				BurstSize:            burst,
			}, dryRun)
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, p)
			}
			return writePlanText(r.Stdout, p)
		},
	}
	cmd.Flags().StringVar(&backend, "backend", "apache", "Web backend (apache, ols, lsws)")
	cmd.Flags().IntVar(&maxRate, "max-rate", 10, "Max requests per second")
	cmd.Flags().IntVar(&burst, "burst", 50, "Burst size")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newFirewallStatusCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "firewall:status",
		Short: "Show firewalld status and open ports",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := security.NewManager(r.Exec)
			info, err := mgr.FirewallStatus(cmd.Context())
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, info)
			}
			for k, v := range info {
				fmt.Fprintf(r.Stdout, "%-10s %s\n", k+":", v)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newFirewallOpenCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "firewall:open <port>",
		Short:   "Open a firewalld port",
		Args:    cobra.ExactArgs(1),
		Example: "  llstack firewall:open 8080",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := security.NewManager(r.Exec)
			if err := mgr.FirewallOpenPort(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(r.Stdout, "port %s opened\n", args[0])
			return nil
		},
	}
	return cmd
}

func (r *Root) newFirewallCloseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "firewall:close <port>",
		Short:   "Close a firewalld port",
		Args:    cobra.ExactArgs(1),
		Example: "  llstack firewall:close 8080",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := security.NewManager(r.Exec)
			if err := mgr.FirewallClosePort(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(r.Stdout, "port %s closed\n", args[0])
			return nil
		},
	}
	return cmd
}
