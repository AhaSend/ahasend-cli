package templates

import "github.com/spf13/cobra"

// NewCommand creates the campaigns templates command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "Manage campaign templates",
		Long: `Manage campaign templates, the designs a campaign can copy with
"campaigns create --campaign-template-id". A campaign template has no draft,
subject or sender: changes apply at once, and campaigns already made from it
keep their own copies.`,
	}

	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewGetCommand())
	cmd.AddCommand(NewCreateCommand())
	cmd.AddCommand(NewUpdateCommand())
	cmd.AddCommand(NewDeleteCommand())

	return cmd
}
