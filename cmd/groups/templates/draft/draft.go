package draft

import "github.com/spf13/cobra"

// NewCommand creates the templates draft command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "draft",
		Short: "Discard a template's draft",
		Long: `Discard a template's draft: the changes saved in the dashboard or through
"templates update" that are not published yet. "templates get" shows the
draft while the template has one.`,
	}

	cmd.AddCommand(NewDiscardCommand())

	return cmd
}
