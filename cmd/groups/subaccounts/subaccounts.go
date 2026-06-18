package subaccounts

import (
	"github.com/AhaSend/ahasend-cli/cmd/groups/subaccounts/apikeys"
	"github.com/spf13/cobra"
)

// NewCommand creates the subaccounts command group
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subaccounts",
		Short: "Manage your AhaSend sub-accounts",
		Long: `Manage sub-accounts under your AhaSend parent account, including listing
sub-accounts, inspecting an individual sub-account, and reviewing usage
allocation across the parent and its sub-accounts.

Common workflow:
  1. List sub-accounts: ahasend subaccounts list
  2. Inspect one: ahasend subaccounts get <sub-account-id>
  3. Review usage: ahasend subaccounts usage`,
	}

	// Add read-only subcommands
	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewGetCommand())
	cmd.AddCommand(NewUsageCommand())

	// Add mutating subcommands
	cmd.AddCommand(NewCreateCommand())
	cmd.AddCommand(NewUpdateCommand())

	// Add state-changing lifecycle subcommands
	cmd.AddCommand(NewDeleteCommand())
	cmd.AddCommand(NewSuspendCommand())
	cmd.AddCommand(NewUnsuspendCommand())

	// Add the nested sub-account API-key subgroup
	cmd.AddCommand(apikeys.NewCommand())

	return cmd
}
