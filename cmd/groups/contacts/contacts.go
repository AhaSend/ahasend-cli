package contacts

import "github.com/spf13/cobra"

// NewCommand creates the contacts command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contacts",
		Short: "Manage account contacts",
		Long: `Manage account-global contacts used for transactional and marketing email.

Contacts created through this command group are not added to a list. Contact
attribute keys must already be configured in the AhaSend dashboard.`,
	}

	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewGetCommand())
	cmd.AddCommand(NewCreateCommand())
	cmd.AddCommand(NewUpdateCommand())
	cmd.AddCommand(NewDeleteCommand())
	cmd.AddCommand(NewBatchCommand())
	cmd.AddCommand(NewMembershipsCommand())

	return cmd
}
