package draft

import "github.com/spf13/cobra"

// NewCommand creates the templates draft command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "draft",
		Short: "Inspect or discard a template's draft",
		Long: `Inspect or discard a template's draft: the changes saved in the dashboard or
through "templates update" that are not published yet.`,
	}

	cmd.AddCommand(NewGetCommand())
	cmd.AddCommand(NewDiscardCommand())

	return cmd
}
