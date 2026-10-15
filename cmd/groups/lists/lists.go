package lists

import (
	"github.com/AhaSend/ahasend-cli/cmd/groups/lists/members"
	"github.com/spf13/cobra"
)

// maxDescriptionLength is the most characters the API accepts in a list's
// description.
const maxDescriptionLength = 1000

// NewCommand creates the lists command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lists",
		Short: "Manage contact lists and their members",
		Long: `Manage contact lists, the static lists of contacts that campaigns target,
and the contacts that belong to them.

A list's contact count is the number of members a campaign to the list would
reach.`,
	}

	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewGetCommand())
	cmd.AddCommand(NewCreateCommand())
	cmd.AddCommand(NewUpdateCommand())
	cmd.AddCommand(NewDeleteCommand())
	cmd.AddCommand(members.NewCommand())

	return cmd
}
