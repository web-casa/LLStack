package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/web-casa/llstack/internal/php"
	"github.com/web-casa/llstack/internal/rollback"
	"github.com/web-casa/llstack/internal/site"
)

type serviceStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type statusPayload struct {
	Backends        []string        `json:"backends"`
	DefaultSite     string          `json:"default_site_root"`
	ManagedSites    int             `json:"managed_sites"`
	EnabledSites    int             `json:"enabled_sites"`
	DisabledSites   int             `json:"disabled_sites"`
	PHPRuntimes     int             `json:"php_runtimes"`
	PendingRollback bool            `json:"pending_rollback"`
	ServiceStatus   []serviceStatus `json:"service_status"`
}

func (r *Root) newStatusCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current LLStack runtime status",
		RunE: func(cmd *cobra.Command, args []string) error {
			siteMgr := site.NewManager(r.Config, r.Logger, r.Exec)
			sites, err := siteMgr.List()
			if err != nil {
				return err
			}
			phpRuntimes, err := php.NewManager(r.Config, r.Logger, r.Exec).List()
			if err != nil {
				return err
			}
			backendSet := map[string]struct{}{}
			enabledSites := 0
			disabledSites := 0
			for _, manifest := range sites {
				backendSet[manifest.Site.Backend] = struct{}{}
				if manifest.Site.State == "disabled" {
					disabledSites++
				} else {
					enabledSites++
				}
			}
			backends := make([]string, 0, len(backendSet))
			for backend := range backendSet {
				backends = append(backends, backend)
			}
			if len(backends) == 0 {
				backends = append(backends, "unconfigured")
			}
			pendingRollback := false
			if _, err := rollback.LoadLatestPending(r.Config.Paths.HistoryDir); err == nil {
				pendingRollback = true
			}
			payload := statusPayload{
				Backends:        backends,
				DefaultSite:     r.Config.Paths.SitesRootDir,
				ManagedSites:    len(sites),
				EnabledSites:    enabledSites,
				DisabledSites:   disabledSites,
				PHPRuntimes:     len(phpRuntimes),
				PendingRollback: pendingRollback,
				ServiceStatus: []serviceStatus{
					{Name: "web", Status: summarizeStatus(len(sites) > 0)},
					{Name: "php", Status: summarizeStatus(len(phpRuntimes) > 0)},
					{Name: "database", Status: "managed"},
					{Name: "cache", Status: "managed"},
				},
			}

			if jsonOutput {
				return writeJSON(r.Stdout, payload)
			}

			if _, err := fmt.Fprintf(r.Stdout, "backends: %s\ndefault sites root: %s\nmanaged sites: %d (enabled=%d disabled=%d)\nphp runtimes: %d\npending rollback: %t\n", strings.Join(payload.Backends, ","), payload.DefaultSite, payload.ManagedSites, payload.EnabledSites, payload.DisabledSites, payload.PHPRuntimes, payload.PendingRollback); err != nil {
				return err
			}

			for _, svc := range payload.ServiceStatus {
				if _, err := fmt.Fprintf(r.Stdout, "- %s: %s\n", svc.Name, svc.Status); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func summarizeStatus(available bool) string {
	if available {
		return "managed"
	}
	return "not_configured"
}
