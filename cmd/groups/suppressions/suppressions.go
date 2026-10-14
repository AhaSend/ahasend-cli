package suppressions

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the suppressions command group
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suppressions",
		Short: "Manage email suppressions",
		Long: `Manage email suppressions (bounces, complaints, unsubscribes).

Suppressions are email addresses that should not receive emails from your
account. Each one applies to one domain: there is no suppression for every
domain, and 'suppressions create' without --domain adds one for each domain
the account has. Bounces and spam complaints add suppressions automatically,
and unsubscribes come from the recipients themselves. Unsubscribes and
complaints are protected: 'suppressions wipe' keeps them.`,
	}

	// Add subcommands
	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewCheckCommand())
	cmd.AddCommand(NewCreateCommand())
	cmd.AddCommand(NewDeleteCommand())
	cmd.AddCommand(NewWipeCommand())

	return cmd
}
