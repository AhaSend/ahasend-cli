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
	return &cobra.Command{
		Use:   "get <campaign-id>",
		Short: "Get a campaign",
		Long: `Get one campaign with its lists and content. For a draft it also shows whether
the campaign is ready to send and, when it is not, each issue in the way.`,
		Example: `  # Get a campaign
  ahasend campaigns get 123e4567-e89b-12d3-a456-426614174000`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runCampaignsGet,
		SilenceUsage: true,
	}
}

func runCampaignsGet(cmd *cobra.Command, args []string) error {
	return showCampaign(cmd, args[0], "get", func(c client.AhaSendClient, id uuid.UUID) (*responses.Campaign, error) {
		return c.GetCampaign(id)
	}, printer.ResponseHandler.HandleSingleCampaign, printer.SingleConfig{
		SuccessMessage: "Campaign retrieved successfully",
		EmptyMessage:   "Campaign not found",
	})
}
