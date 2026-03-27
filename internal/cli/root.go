package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/web-casa/llstack/internal/buildinfo"
	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/core/plan"
	"github.com/web-casa/llstack/internal/logging"
	"github.com/web-casa/llstack/internal/system"
)

// Dependencies contains the application services used by commands.
type Dependencies struct {
	Version string
	Build   buildinfo.Info
	Config  config.RuntimeConfig
	Logger  logging.Logger
	Exec    system.Executor
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

// Root owns the CLI dependency graph.
type Root struct {
	Dependencies
}

const (
	commandGroupCore     = "core"
	commandGroupSite     = "site"
	commandGroupRuntime  = "runtime"
	commandGroupOps      = "ops"
	commandGroupSecurity = "security"
	commandGroupMeta     = "meta"
)

// NewRoot creates the root command container.
func NewRoot(deps Dependencies) *Root {
	if deps.Build.Version == "" && deps.Version != "" {
		deps.Build.Version = deps.Version
	}
	deps.Build = buildinfo.Normalize(deps.Build)
	deps.Version = deps.Build.Version
	return &Root{Dependencies: deps}
}

// Command builds the Cobra command tree.
func (r *Root) Command(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "llstack",
		Short: "Manage EL9/EL10 web stack install, sites, runtimes, and repair workflows.",
		Long: strings.TrimSpace(`
LLStack is a CLI and TUI-first web stack installer and site lifecycle manager for
EL9/EL10 systems.

Use dry-run and plan-only modes to preview changes before applying them. Most
write operations support machine-readable JSON output for automation.
`),
		Example: strings.TrimSpace(`
  llstack install --config examples/install/basic.yaml --dry-run
  llstack site:create example.com --backend apache --profile wordpress --dry-run
  llstack doctor --bundle --bundle-path /tmp/llstack-doctor.tar.gz
  llstack tui
`),
	}

	cmd.SetContext(ctx)
	cmd.SetIn(r.Stdin)
	cmd.SetOut(r.Stdout)
	cmd.SetErr(r.Stderr)
	cmd.SilenceUsage = true
	cmd.SuggestionsMinimumDistance = 2
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		return fmt.Errorf("%w\nrun '%s --help' for usage", err, c.CommandPath())
	})
	cmd.AddGroup(
		&cobra.Group{ID: commandGroupCore, Title: "Getting Started"},
		&cobra.Group{ID: commandGroupSite, Title: "Site Lifecycle"},
		&cobra.Group{ID: commandGroupRuntime, Title: "PHP, Database, and Cache"},
		&cobra.Group{ID: commandGroupOps, Title: "Diagnostics and Recovery"},
		&cobra.Group{ID: commandGroupSecurity, Title: "Security and Firewall"},
		&cobra.Group{ID: commandGroupMeta, Title: "Interfaces and Metadata"},
	)
	cmd.AddCommand(
		withGroup(commandGroupMeta, r.newVersionCommand()),
		withGroup(commandGroupCore, r.newStatusCommand()),
		withGroup(commandGroupOps, r.newDoctorCommand()),
		withGroup(commandGroupOps, r.newRepairCommand()),
		withGroup(commandGroupCore, r.newInitCommand()),
		withGroup(commandGroupCore, r.newInstallCommand()),
		withGroup(commandGroupRuntime, r.newDBListCommand()),
		withGroup(commandGroupRuntime, r.newDBInstallCommand()),
		withGroup(commandGroupRuntime, r.newDBInitCommand()),
		withGroup(commandGroupRuntime, r.newDBCreateCommand()),
		withGroup(commandGroupRuntime, r.newDBUserCreateCommand()),
		withGroup(commandGroupRuntime, r.newDBRemoveCommand()),
		withGroup(commandGroupRuntime, r.newDBBackupCommand()),
		withGroup(commandGroupRuntime, r.newDBTuneCommand()),
		withGroup(commandGroupRuntime, r.newCacheInstallCommand()),
		withGroup(commandGroupRuntime, r.newCacheStatusCommand()),
		withGroup(commandGroupRuntime, r.newCacheConfigureCommand()),
		withGroup(commandGroupRuntime, r.newPHPListCommand()),
		withGroup(commandGroupRuntime, r.newPHPInstallCommand()),
		withGroup(commandGroupRuntime, r.newPHPExtensionsCommand()),
		withGroup(commandGroupRuntime, r.newPHPINICommand()),
		withGroup(commandGroupRuntime, r.newPHPRemoveCommand()),
		withGroup(commandGroupRuntime, r.newPHPTuneCommand()),
		withGroup(commandGroupSite, r.newSiteCreateCommand()),
		withGroup(commandGroupSite, r.newSiteUpdateCommand()),
		withGroup(commandGroupSite, r.newSiteShowCommand()),
		withGroup(commandGroupSite, r.newSitePHPCommand()),
		withGroup(commandGroupSite, r.newSiteListCommand()),
		withGroup(commandGroupSite, r.newSiteStartCommand()),
		withGroup(commandGroupSite, r.newSiteStopCommand()),
		withGroup(commandGroupSite, r.newSiteReloadCommand()),
		withGroup(commandGroupSite, r.newSiteRestartCommand()),
		withGroup(commandGroupSite, r.newSiteSSLCommand()),
		withGroup(commandGroupSite, r.newSiteLogsCommand()),
		withGroup(commandGroupSite, r.newSiteDiffCommand()),
		withGroup(commandGroupSite, r.newSiteDeleteCommand()),
		withGroup(commandGroupOps, r.newRollbackCommand()),
		withGroup(commandGroupOps, r.newRollbackListCommand()),
		withGroup(commandGroupOps, r.newRollbackShowCommand()),
		withGroup(commandGroupSite, r.newSiteBackupCommand()),
		withGroup(commandGroupSite, r.newSiteRestoreCommand()),
		withGroup(commandGroupSite, r.newSitePHPSwitchCommand()),
		withGroup(commandGroupSite, r.newSiteStatsCommand()),
		withGroup(commandGroupSite, r.newSiteBatchCreateCommand()),
		withGroup(commandGroupSite, r.newSitePHPConfigCommand()),
		withGroup(commandGroupSite, r.newSiteHtaccessCheckCommand()),
		withGroup(commandGroupSite, r.newSiteHtaccessCompileCommand()),
		withGroup(commandGroupOps, r.newSSLStatusCommand()),
		withGroup(commandGroupOps, r.newSSLRenewCommand()),
		withGroup(commandGroupOps, r.newSSLAutoRenewCommand()),
		withGroup(commandGroupOps, r.newCronAddCommand()),
		withGroup(commandGroupOps, r.newCronListCommand()),
		withGroup(commandGroupOps, r.newCronRemoveCommand()),
		withGroup(commandGroupOps, r.newTuneCommand()),
		withGroup(commandGroupSecurity, r.newFail2banEnableCommand()),
		withGroup(commandGroupSecurity, r.newFail2banStatusCommand()),
		withGroup(commandGroupSecurity, r.newBlockIPCommand()),
		withGroup(commandGroupSecurity, r.newUnblockIPCommand()),
		withGroup(commandGroupSecurity, r.newBlockListCommand()),
		withGroup(commandGroupSecurity, r.newRateLimitCommand()),
		withGroup(commandGroupSecurity, r.newFirewallStatusCommand()),
		withGroup(commandGroupSecurity, r.newFirewallOpenCommand()),
		withGroup(commandGroupSecurity, r.newFirewallCloseCommand()),
		withGroup(commandGroupCore, r.newAppInstallCommand()),
		withGroup(commandGroupCore, r.newAppListCommand()),
		withGroup(commandGroupOps, r.newSFTPCreateCommand()),
		withGroup(commandGroupOps, r.newSFTPListCommand()),
		withGroup(commandGroupOps, r.newSFTPRemoveCommand()),
		withGroup(commandGroupMeta, r.newTUICommand()),
		withGroup(commandGroupCore, r.newWelcomeRemoveCommand()),
	)

	return cmd
}

func withGroup(groupID string, cmd *cobra.Command) *cobra.Command {
	cmd.GroupID = groupID
	return cmd
}

func writeJSON(w io.Writer, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(w, string(encoded))
	return err
}

func writePlanText(w io.Writer, p plan.Plan) error {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("%s\n", p.Summary))
	for _, warning := range p.Warnings {
		builder.WriteString(fmt.Sprintf("warning: %s\n", warning))
	}
	for _, op := range p.Operations {
		builder.WriteString(fmt.Sprintf("- %s %s", op.Kind, op.Target))
		if len(op.Details) > 0 {
			details := make([]string, 0, len(op.Details))
			keys := make([]string, 0, len(op.Details))
			for key := range op.Details {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				details = append(details, fmt.Sprintf("%s=%s", key, op.Details[key]))
			}
			builder.WriteString(fmt.Sprintf(" (%s)", strings.Join(details, ", ")))
		}
		builder.WriteString("\n")
	}

	_, err := fmt.Fprint(w, builder.String())
	return err
}
