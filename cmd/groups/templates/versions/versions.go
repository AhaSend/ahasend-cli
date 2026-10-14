package versions

import "github.com/spf13/cobra"

// NewCommand creates the templates versions command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "versions",
		Short: "Manage a template's published versions",
		Long: `List, inspect and restore a template's published versions. Every publish keeps
a version, and a template keeps its 50 most recent versions.`,
	}

	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewGetCommand())
	cmd.AddCommand(NewRestoreCommand())

	return cmd
}
