package templates

import (
	"github.com/AhaSend/ahasend-cli/cmd/groups/templates/draft"
	"github.com/AhaSend/ahasend-cli/cmd/groups/templates/versions"
	"github.com/spf13/cobra"
)

// NewCommand creates the templates command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "Manage transactional templates",
		Long: `Manage transactional templates, the stored designs that
"messages send --template-id" sends.

Changes go to the template's draft. Sends use the published copy, so publish
the draft with --publish or "templates publish" before sending. Every publish
is kept as a version that can be restored.`,
	}

	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewGetCommand())
	cmd.AddCommand(NewCreateCommand())
	cmd.AddCommand(NewUpdateCommand())
	cmd.AddCommand(NewDeleteCommand())
	cmd.AddCommand(NewPublishCommand())
	cmd.AddCommand(draft.NewCommand())
	cmd.AddCommand(versions.NewCommand())

	return cmd
}
