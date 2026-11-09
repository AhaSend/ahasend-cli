package segments

import "github.com/spf13/cobra"

// maxDescriptionLength is the most characters the API accepts in a
// segment's description.
const maxDescriptionLength = 1000

// NewCommand creates the segments command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "segments",
		Short: "Manage segments of contacts and preview their rules",
		Long: `Manage segments: saved rules over the account's contacts, whose members
the API finds and keeps up to date. A campaign sends to segments, and leaves
the members of segments out, as it does with lists.

A segment is building while its members are found, ready when they follow
its rules, and failed when they could not be found. A campaign to a segment
cannot be sent until the segment is ready; "segments get" shows its state.`,
	}

	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewGetCommand())
	cmd.AddCommand(NewCreateCommand())
	cmd.AddCommand(NewUpdateCommand())
	cmd.AddCommand(NewDeleteCommand())
	cmd.AddCommand(NewPreviewCommand())
	cmd.AddCommand(NewMembersCommand())

	return cmd
}
