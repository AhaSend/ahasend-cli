package campaigns

import (
	"github.com/AhaSend/ahasend-cli/cmd/groups/campaigns/templates"
	"github.com/spf13/cobra"
)

// NewCommand creates the campaigns command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "campaigns",
		Short: "Manage marketing campaigns",
		Long: `Draft, test, send, schedule and report on marketing campaigns, which send to
contact lists.

A campaign is a draft until it is sent; only drafts can be changed or deleted.
The draft lists the issues that keep it from being sent. Campaign templates,
the designs a campaign can start from, are managed through
"campaigns templates".`,
	}

	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewGetCommand())
	cmd.AddCommand(NewCreateCommand())
	cmd.AddCommand(NewUpdateCommand())
	cmd.AddCommand(NewDeleteCommand())
	cmd.AddCommand(NewDuplicateCommand())
	cmd.AddCommand(NewSendCommand())
	cmd.AddCommand(NewUnscheduleCommand())
	cmd.AddCommand(NewTestCommand())
	cmd.AddCommand(NewStatsCommand())
	cmd.AddCommand(NewRecipientsCommand())
	cmd.AddCommand(templates.NewCommand())

	return cmd
}
