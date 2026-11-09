package campaigns

import (
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the campaigns get command.
func NewGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <campaign-id>",
		Short: "Get a campaign",
		Long: `Get one campaign with its audience and content. For a draft it also shows
whether the campaign is ready to send and, when it is not, each issue in the
way.

--markup writes only the AhaML of a Simple Editor design to stdout. Save it,
edit it and write it back with "campaigns update --markup-file". It fails
when the design has no AhaML, and cannot be used with --output.`,
		Example: `  # Get a campaign
  ahasend campaigns get 123e4567-e89b-12d3-a456-426614174000

  # Save the design as AhaML to edit it
  ahasend campaigns get 123e4567-e89b-12d3-a456-426614174000 --markup > design.ahaml`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runCampaignsGet,
		SilenceUsage: true,
	}
	cmdutil.AddMarkupFlag(cmd)
	return cmd
}

func runCampaignsGet(cmd *cobra.Command, args []string) error {
	markup, err := cmdutil.ReadMarkupFlag(cmd)
	if err != nil {
		return err
	}
	show := printer.ResponseHandler.HandleSingleCampaign
	if markup {
		show = func(_ printer.ResponseHandler, campaign *responses.Campaign, _ printer.SingleConfig) error {
			var content *responses.TemplateContent
			if campaign != nil {
				content = campaign.Content
			}
			return cmdutil.WriteMarkup(cmd, "campaign", args[0], content)
		}
	}
	return showCampaign(cmd, args[0], "get", func(c client.AhaSendClient, id uuid.UUID) (*responses.Campaign, error) {
		return c.GetCampaign(id)
	}, show, printer.SingleConfig{
		SuccessMessage: "Campaign retrieved successfully",
		EmptyMessage:   "Campaign not found",
	})
}
