package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/web-casa/llstack/internal/site"
	"github.com/web-casa/llstack/internal/system"
	"github.com/web-casa/llstack/internal/tuning"
)

func (r *Root) newTuneCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "tune",
		Short: "Show hardware-aware tuning recommendations",
		Long:  "Detect server hardware and calculate optimal parameters for PHP, Apache, OLS, database, and cache.",
		Example: `  llstack tune
  llstack tune --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			hw := system.DetectHardware()
			mgr := site.NewManager(r.Config, r.Logger, r.Exec)
			manifests, _ := mgr.List()
			siteCount := len(manifests)
			if siteCount < 1 {
				siteCount = 1
			}

			profile := tuning.Calculate(hw, siteCount)

			if jsonOutput {
				return writeJSON(r.Stdout, profile)
			}

			fmt.Fprintf(r.Stdout, "Hardware: %d CPU cores, %d MB RAM (%.1f GB)\n", hw.CPUCores, hw.MemoryMB, hw.MemoryGB)
			fmt.Fprintf(r.Stdout, "Sites:    %d managed\n\n", siteCount)

			fmt.Fprintln(r.Stdout, "PHP-FPM (per-site pool):")
			fmt.Fprintf(r.Stdout, "  pm.max_children    = %d\n", profile.PHPMaxChildrenSite)
			fmt.Fprintf(r.Stdout, "  pm.start_servers   = %d\n", profile.PHPStartServers)
			fmt.Fprintf(r.Stdout, "  pm.min_spare       = %d\n", profile.PHPMinSpare)
			fmt.Fprintf(r.Stdout, "  pm.max_spare       = %d\n\n", profile.PHPMaxSpare)

			fmt.Fprintln(r.Stdout, "Apache:")
			fmt.Fprintf(r.Stdout, "  MaxRequestWorkers  = %d\n\n", profile.ApacheMaxRequestWorkers)

			fmt.Fprintln(r.Stdout, "OLS/LSWS:")
			fmt.Fprintf(r.Stdout, "  maxConns           = %d\n", profile.OLSMaxConns)
			fmt.Fprintf(r.Stdout, "  PHP_LSAPI_CHILDREN = %d\n\n", profile.OLSLSAPIChildren)

			fmt.Fprintln(r.Stdout, "Database:")
			fmt.Fprintf(r.Stdout, "  buffer_pool_size   = %d MB\n", profile.DBBufferPoolMB)
			fmt.Fprintf(r.Stdout, "  max_connections    = %d\n", profile.DBMaxConnections)
			fmt.Fprintf(r.Stdout, "  pg_shared_buffers  = %d MB\n\n", profile.PGSharedBuffersMB)

			fmt.Fprintln(r.Stdout, "Cache:")
			fmt.Fprintf(r.Stdout, "  redis_maxmemory    = %d MB\n", profile.RedisMaxMemoryMB)
			fmt.Fprintf(r.Stdout, "  memcached_cache    = %d MB\n", profile.MemcachedCacheMB)

			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}
