package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/web-casa/llstack/internal/sftp"
)

func (r *Root) newSFTPCreateCommand() *cobra.Command {
	var username, password, sshKey string
	var autoPass, jsonOutput bool

	cmd := &cobra.Command{
		Use:   "sftp:create <site>",
		Short: "Create an SFTP account for a site",
		Args:  cobra.ExactArgs(1),
		Example: `  llstack sftp:create wp.example.com
  llstack sftp:create wp.example.com --username myuser --password secret
  llstack sftp:create wp.example.com --auto-password --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := sftp.NewManager("/etc/llstack/sftp", r.Exec)
			account, pass, err := mgr.Create(cmd.Context(), sftp.CreateOptions{
				Site:     args[0],
				Username: username,
				Password: password,
				AutoPass: autoPass || password == "",
				SSHKey:   sshKey,
			})
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, map[string]string{
					"username": account.Username,
					"password": pass,
					"site":     account.Site,
					"home":     account.HomeDir,
				})
			}
			fmt.Fprintf(r.Stdout, "SFTP account created:\n")
			fmt.Fprintf(r.Stdout, "  Username: %s\n", account.Username)
			fmt.Fprintf(r.Stdout, "  Password: %s\n", pass)
			fmt.Fprintf(r.Stdout, "  Home:     %s\n", account.HomeDir)
			fmt.Fprintf(r.Stdout, "\n  Connect: sftp %s@<server-ip>\n", account.Username)
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "Custom username")
	cmd.Flags().StringVar(&password, "password", "", "Set password")
	cmd.Flags().BoolVar(&autoPass, "auto-password", false, "Auto-generate password")
	cmd.Flags().StringVar(&sshKey, "ssh-key", "", "SSH public key")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newSFTPListCommand() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "sftp:list",
		Short: "List managed SFTP accounts",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := sftp.NewManager("/etc/llstack/sftp", r.Exec)
			accounts, err := mgr.List()
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(r.Stdout, accounts)
			}
			if len(accounts) == 0 {
				fmt.Fprintln(r.Stdout, "no SFTP accounts")
				return nil
			}
			for _, a := range accounts {
				fmt.Fprintf(r.Stdout, "%-20s  site=%-25s  home=%s\n", a.Username, a.Site, a.HomeDir)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}

func (r *Root) newSFTPRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sftp:remove <username>",
		Short: "Remove an SFTP account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := sftp.NewManager("/etc/llstack/sftp", r.Exec)
			if err := mgr.Remove(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(r.Stdout, "SFTP account %s removed\n", args[0])
			return nil
		},
	}
	return cmd
}
