package members

import "github.com/spf13/cobra"

// NewCommand creates the lists members command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "members",
		Short: "Manage the contacts on a list",
		Long: `Manage the contacts that belong to a contact list. A member is one contact's
membership of one list, with its own subscription status. Contacts must exist
before they can be added to a list.`,
	}

	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewAddCommand())
	cmd.AddCommand(NewSetCommand())
	cmd.AddCommand(NewRemoveCommand())

	return cmd
}
