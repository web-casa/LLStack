package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/web-casa/llstack/internal/backend/ols"
	"github.com/web-casa/llstack/internal/site"
)

func (r *Root) newSitePHPConfigCommand() *cobra.Command {
	var show bool
	var reset bool
	var dryRun bool
	var jsonOutput bool
	var params []string

	cmd := &cobra.Command{
		Use:   "site:php-config <site>",
		Short: "Manage per-site PHP configuration overrides",
		Long:  "Set, show, or reset per-site PHP parameters (memory_limit, upload_max_filesize, etc.)",
		Args:  cobra.ExactArgs(1),
		Example: `  llstack site:php-config wp.example.com --set memory_limit=512M --set upload_max_filesize=128M
  llstack site:php-config wp.example.com --show
  llstack site:php-config wp.example.com --reset`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := site.NewManager(r.Config, r.Logger, r.Exec)
			siteName := args[0]

			if show {
				content, err := mgr.ShowPHPConfig(siteName)
				if err != nil {
					return err
				}
				fmt.Fprint(r.Stdout, content)
				return nil
			}

			var overrides []site.PHPConfigOverride
			for _, p := range params {
				parts := strings.SplitN(p, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid parameter format %q, expected key=value", p)
				}
				overrides = append(overrides, site.PHPConfigOverride{Key: parts[0], Value: parts[1]})
			}

			plan, err := mgr.UpdatePHPConfig(cmd.Context(), site.PHPConfigOptions{
				Name:      siteName,
				Overrides: overrides,
				Reset:     reset,
				DryRun:    dryRun,
			})
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, plan)
			}
			return writePlanText(r.Stdout, plan)
		},
	}

	cmd.Flags().BoolVar(&show, "show", false, "Show current per-site PHP config")
	cmd.Flags().BoolVar(&reset, "reset", false, "Remove all per-site PHP overrides")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	cmd.Flags().StringArrayVar(&params, "set", nil, "Set a PHP parameter (e.g. --set memory_limit=512M)")
	return cmd
}

func (r *Root) newSiteBackupCommand() *cobra.Command {
	var outputDir, dbDumpCmd string
	var includeDB, dryRun, jsonOutput bool

	cmd := &cobra.Command{
		Use:   "site:backup <site>",
		Short: "Backup a site (files + config + optional DB dump)",
		Args:  cobra.ExactArgs(1),
		Example: `  llstack site:backup wp.example.com
  llstack site:backup wp.example.com --include-db --db-dump-cmd "mysqldump wordpress_db"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := site.NewManager(r.Config, r.Logger, r.Exec)
			p, result, err := mgr.Backup(cmd.Context(), site.BackupOptions{
				Name:      args[0],
				OutputDir: outputDir,
				IncludeDB: includeDB,
				DBDumpCmd: dbDumpCmd,
				DryRun:    dryRun,
			})
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, result)
			}
			if dryRun {
				return writePlanText(r.Stdout, p)
			}
			fmt.Fprintf(r.Stdout, "Backup created: %s (%d bytes)\n", result.BackupPath, result.Size)
			return nil
		},
	}
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory")
	cmd.Flags().BoolVar(&includeDB, "include-db", false, "Include database dump")
	cmd.Flags().StringVar(&dbDumpCmd, "db-dump-cmd", "", "DB dump command (e.g. 'mysqldump dbname -u user -ppass')")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	return cmd
}

func (r *Root) newSiteRestoreCommand() *cobra.Command {
	var dryRun, jsonOutput bool

	cmd := &cobra.Command{
		Use:     "site:restore <site> <backup-file>",
		Short:   "Restore a site from a backup archive",
		Args:    cobra.ExactArgs(2),
		Example: "  llstack site:restore wp.example.com /var/lib/llstack/backups/sites/wp.example.com/wp.example.com-20260327.tar.gz",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := site.NewManager(r.Config, r.Logger, r.Exec)
			p, err := mgr.Restore(cmd.Context(), site.RestoreOptions{
				Name:       args[0],
				BackupPath: args[1],
				DryRun:     dryRun,
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
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	return cmd
}

func (r *Root) newSitePHPSwitchCommand() *cobra.Command {
	var version string
	var dryRun, skipReload, jsonOutput bool

	cmd := &cobra.Command{
		Use:     "site:php-switch <site>",
		Short:   "Switch PHP version for a site",
		Args:    cobra.ExactArgs(1),
		Example: "  llstack site:php-switch wp.example.com --version 8.4",
		RunE: func(cmd *cobra.Command, args []string) error {
			if version == "" {
				return fmt.Errorf("--version is required")
			}
			mgr := site.NewManager(r.Config, r.Logger, r.Exec)
			p, err := mgr.SwitchPHP(cmd.Context(), site.PHPSwitchOptions{
				Name:       args[0],
				NewVersion: version,
				DryRun:     dryRun,
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
	cmd.Flags().StringVar(&version, "version", "", "Target PHP version")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview")
	cmd.Flags().BoolVar(&skipReload, "skip-reload", false, "Skip reload")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	return cmd
}

func (r *Root) newSiteStatsCommand() *cobra.Command {
	var topN int
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "site:stats <site>",
		Short: "Show access log statistics for a site",
		Args:  cobra.ExactArgs(1),
		Example: `  llstack site:stats wp.example.com
  llstack site:stats wp.example.com --top 20 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := site.NewManager(r.Config, r.Logger, r.Exec)
			manifest, err := mgr.Show(args[0])
			if err != nil {
				return fmt.Errorf("site %q not found", args[0])
			}
			logPath := manifest.Site.Logs.AccessLog
			if logPath == "" {
				return fmt.Errorf("no access log configured for %q", args[0])
			}
			stats, err := site.AnalyzeAccessLog(manifest.Site.Name, logPath, topN)
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, stats)
			}
			fmt.Fprint(r.Stdout, site.StatsText(stats))
			return nil
		},
	}
	cmd.Flags().IntVar(&topN, "top", 10, "Number of top entries to show")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newSiteBatchCreateCommand() *cobra.Command {
	var dryRun bool
	var skipReload bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "site:batch-create <config-file>",
		Short: "Create multiple sites from a YAML/JSON config file",
		Args:  cobra.ExactArgs(1),
		Example: `  llstack site:batch-create sites.yaml --dry-run
  llstack site:batch-create sites.json --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := site.LoadBatchConfig(args[0])
			if err != nil {
				return err
			}
			mgr := site.NewManager(r.Config, r.Logger, r.Exec)
			plans, err := mgr.CreateBatch(cmd.Context(), cfg, dryRun, skipReload)
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, plans)
			}
			for _, p := range plans {
				writePlanText(r.Stdout, p)
				fmt.Fprintln(r.Stdout, "")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying")
	cmd.Flags().BoolVar(&skipReload, "skip-reload", false, "Skip backend reload")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newSiteHtaccessCheckCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "site:htaccess-check <site>",
		Short: "Check .htaccess compatibility for OLS sites",
		Args:  cobra.ExactArgs(1),
		Example: `  llstack site:htaccess-check wp.example.com
  llstack site:htaccess-check wp.example.com --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := site.NewManager(r.Config, r.Logger, r.Exec)
			manifest, err := mgr.Show(args[0])
			if err != nil {
				return fmt.Errorf("site %q not found", args[0])
			}
			if manifest.Site.Backend != "ols" {
				fmt.Fprintln(r.Stdout, "htaccess-check is only relevant for OLS backend sites")
				return nil
			}
			result, err := ols.CheckHtaccess(manifest.Site.Name, manifest.Site.DocumentRoot)
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, result)
			}
			if len(result.Translated) == 0 && len(result.Warnings) == 0 {
				fmt.Fprintln(r.Stdout, "no .htaccess compatibility issues found")
				return nil
			}
			for _, d := range result.Translated {
				fmt.Fprintf(r.Stdout, "  line %d: CONVERTIBLE  %s\n    → %s\n", d.Line, d.Directive, d.Suggestion)
			}
			for _, d := range result.Warnings {
				fmt.Fprintf(r.Stdout, "  line %d: UNSUPPORTED  %s\n    → %s\n", d.Line, d.Directive, d.Suggestion)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newSiteHtaccessCompileCommand() *cobra.Command {
	var apply bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "site:htaccess-compile <site>",
		Short: "Convert incompatible .htaccess directives for OLS",
		Long:  "Converts php_value/php_flag to .user.ini and comments out unsupported directives. Use --apply to modify files.",
		Args:  cobra.ExactArgs(1),
		Example: `  llstack site:htaccess-compile wp.example.com
  llstack site:htaccess-compile wp.example.com --apply`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := site.NewManager(r.Config, r.Logger, r.Exec)
			manifest, err := mgr.Show(args[0])
			if err != nil {
				return fmt.Errorf("site %q not found", args[0])
			}
			if manifest.Site.Backend != "ols" {
				fmt.Fprintln(r.Stdout, "htaccess-compile is only relevant for OLS backend sites")
				return nil
			}
			result, err := ols.CompileHtaccess(manifest.Site.Name, manifest.Site.DocumentRoot, apply)
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, result)
			}
			if len(result.Translated) == 0 {
				fmt.Fprintln(r.Stdout, "no convertible directives found")
				return nil
			}
			action := "would convert"
			if apply {
				action = "converted"
			}
			fmt.Fprintf(r.Stdout, "%s %d directives\n", action, len(result.Translated))
			for _, d := range result.Translated {
				fmt.Fprintf(r.Stdout, "  line %d: %s → %s\n", d.Line, d.Directive, d.Target)
			}
			if !apply {
				fmt.Fprintln(r.Stdout, "\nUse --apply to modify files.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually modify .htaccess and generate .user.ini")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}
