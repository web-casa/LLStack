package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	appdeploy "github.com/web-casa/llstack/internal/appdeploy"
)

func (r *Root) newAppInstallCommand() *cobra.Command {
	var siteName, backend, phpVersion, dbProvider string
	var dryRun, jsonOutput bool

	cmd := &cobra.Command{
		Use:   "app:install <app-name>",
		Short: "Deploy a PHP application to a site",
		Long:  "Supported apps: " + appList(),
		Args:  cobra.ExactArgs(1),
		Example: `  llstack app:install wordpress --site wp.example.com
  llstack app:install nextcloud --site cloud.example.com --backend apache
  llstack app:install laravel --site app.example.com`,
		RunE: func(cmd *cobra.Command, args []string) error {
			appName := args[0]
			spec, ok := appdeploy.FindApp(appName)
			if !ok {
				return fmt.Errorf("unknown app %q; available: %s", appName, appList())
			}
			if siteName == "" {
				return fmt.Errorf("--site is required")
			}
			if backend == "" {
				backend = "apache"
			}
			if phpVersion == "" {
				phpVersion = "8.3"
			}
			docroot := fmt.Sprintf("/data/www/%s", siteName)
			_ = spec

			result, err := appdeploy.Deploy(cmd.Context(), r.Exec, appdeploy.DeployOptions{
				AppName:    appName,
				SiteName:   siteName,
				Docroot:    docroot,
				Backend:    backend,
				PHPVersion: phpVersion,
				DBProvider: dbProvider,
				DryRun:     dryRun,
			})
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, result)
			}
			fmt.Fprintf(r.Stdout, "App: %s\nSite: %s\nStatus: %s\n", result.App, result.Site, result.Status)
			if result.DBName != "" {
				fmt.Fprintf(r.Stdout, "Database: %s\nDB User: %s\nDB Pass: %s\n", result.DBName, result.DBUser, result.DBPass)
			}
			if result.Message != "" {
				fmt.Fprintf(r.Stdout, "Message: %s\n", result.Message)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&siteName, "site", "", "Target site domain")
	cmd.Flags().StringVar(&backend, "backend", "apache", "Web backend")
	cmd.Flags().StringVar(&phpVersion, "php-version", "8.3", "PHP version")
	cmd.Flags().StringVar(&dbProvider, "db", "", "Database provider for auto-setup")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without deploying")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newAppListCommand() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "app:list",
		Short: "List available applications",
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog := appdeploy.Catalog()
			if jsonOutput {
				return writeJSON(r.Stdout, catalog)
			}
			fmt.Fprintf(r.Stdout, "%-15s %-30s %s\n", "NAME", "DISPLAY", "DESCRIPTION")
			fmt.Fprintln(r.Stdout, strings.Repeat("-", 70))
			for _, a := range catalog {
				fmt.Fprintf(r.Stdout, "%-15s %-30s %s\n", a.Name, a.DisplayName, a.Description)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func appList() string {
	names := make([]string, 0)
	for _, a := range appdeploy.Catalog() {
		names = append(names, a.Name)
	}
	return strings.Join(names, ", ")
}
