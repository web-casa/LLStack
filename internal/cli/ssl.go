package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/web-casa/llstack/internal/site"
	sslprovider "github.com/web-casa/llstack/internal/ssl"
)

func (r *Root) newSSLStatusCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "ssl:status",
		Short: "Show TLS certificate status for all sites",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := site.NewManager(r.Config, r.Logger, r.Exec)
			manifests, err := mgr.List()
			if err != nil {
				return err
			}
			var sites []sslprovider.SiteInfo
			for _, m := range manifests {
				if !m.Site.TLS.Enabled {
					continue
				}
				sites = append(sites, sslprovider.SiteInfo{
					Name:     m.Site.Name,
					CertFile: m.Site.TLS.CertificateFile,
					Domain:   m.Site.Domain.ServerName,
					Docroot:  m.Site.DocumentRoot,
				})
			}
			lm := sslprovider.NewLifecycleManager(r.Config, r.Exec)
			statuses := lm.Status(sites)
			if jsonOutput {
				return writeJSON(r.Stdout, statuses)
			}
			for _, s := range statuses {
				fmt.Fprintf(r.Stdout, "%-30s  %-10s  days_left=%d  issuer=%s\n", s.Site, s.Status, s.DaysLeft, s.Issuer)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newSSLRenewCommand() *cobra.Command {
	var email string
	var all bool
	var dryRun bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "ssl:renew [site]",
		Short: "Renew TLS certificate for a site (or --all expiring)",
		RunE: func(cmd *cobra.Command, args []string) error {
			lm := sslprovider.NewLifecycleManager(r.Config, r.Exec)
			mgr := site.NewManager(r.Config, r.Logger, r.Exec)
			manifests, _ := mgr.List()

			var sites []sslprovider.SiteInfo
			for _, m := range manifests {
				if !m.Site.TLS.Enabled {
					continue
				}
				sites = append(sites, sslprovider.SiteInfo{
					Name:     m.Site.Name,
					CertFile: m.Site.TLS.CertificateFile,
					Domain:   m.Site.Domain.ServerName,
					Docroot:  m.Site.DocumentRoot,
				})
			}

			if all {
				plans, err := lm.RenewExpiring(cmd.Context(), sites, sslprovider.RenewAllOptions{
					Email:         email,
					ThresholdDays: sslprovider.ExpiryThresholdDays,
					DryRun:        dryRun,
				})
				if err != nil {
					return err
				}
				if jsonOutput {
					return writeJSON(r.Stdout, plans)
				}
				fmt.Fprintf(r.Stdout, "renewed %d certificates\n", len(plans))
				return nil
			}

			if len(args) == 0 {
				return fmt.Errorf("specify a site name or use --all")
			}
			var target sslprovider.SiteInfo
			for _, s := range sites {
				if s.Name == args[0] {
					target = s
					break
				}
			}
			if target.Name == "" {
				return fmt.Errorf("site %q not found or TLS not enabled", args[0])
			}
			p, err := lm.Renew(cmd.Context(), target, email, dryRun)
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, p)
			}
			return writePlanText(r.Stdout, p)
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "ACME account email")
	cmd.Flags().BoolVar(&all, "all", false, "Renew all expiring certificates")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newSSLAutoRenewCommand() *cobra.Command {
	var disable bool

	cmd := &cobra.Command{
		Use:   "ssl:auto-renew",
		Short: "Enable or disable automatic SSL certificate renewal",
		RunE: func(cmd *cobra.Command, args []string) error {
			lm := sslprovider.NewLifecycleManager(r.Config, r.Exec)
			if disable {
				if err := lm.DisableAutoRenew(cmd.Context()); err != nil {
					return err
				}
				fmt.Fprintln(r.Stdout, "SSL auto-renewal disabled")
				return nil
			}
			if err := lm.EnableAutoRenew(cmd.Context()); err != nil {
				return err
			}
			fmt.Fprintln(r.Stdout, "SSL auto-renewal enabled (daily systemd timer)")
			return nil
		},
	}
	cmd.Flags().BoolVar(&disable, "disable", false, "Disable auto-renewal")
	return cmd
}
