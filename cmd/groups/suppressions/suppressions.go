package suppressions

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/spf13/cobra"
)

// NewCommand creates the suppressions command group
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suppressions",
		Short: "Manage email suppressions",
		Long: `Manage email suppressions (bounces, complaints, unsubscribes).

Suppressions are email addresses that should not receive emails from your account.
They can be created automatically (bounces, complaints) or manually (unsubscribes).
Suppressions can be domain-specific or global across all domains.

Use suppressions to:
- View all suppressed email addresses
- Check if a specific email is suppressed
- Create new suppressions for email addresses
- Delete existing suppressions
- Bulk manage suppressions with CSV export/import

Examples:
  # List all suppressions
  ahasend suppressions list

  # Check if an email is suppressed
  ahasend suppressions check user@example.com

  # Create a new suppression
  ahasend suppressions create user@example.com --reason unsubscribe

  # Delete a suppression
  ahasend suppressions delete user@example.com`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Call root command's PersistentPreRunE to initialize printer
			// We need to traverse up to find the root command
			root := cmd.Root()
			if root != nil && root != cmd && root.PersistentPreRunE != nil {
				if err := root.PersistentPreRunE(cmd, args); err != nil {
					return err
				}
			}
			return auth.RequireAuth(cmd)
		},
	}

	// Add subcommands
	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewCheckCommand())
	cmd.AddCommand(NewCreateCommand())
	cmd.AddCommand(NewDeleteCommand())
	cmd.AddCommand(NewWipeCommand())

	return cmd
}
