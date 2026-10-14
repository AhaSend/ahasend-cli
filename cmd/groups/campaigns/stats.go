package campaigns

import (
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// NewStatsCommand creates the campaigns stats command.
func NewStatsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stats <campaign-id>",
		Short: "Show a campaign's delivery counts and rates",
		Long: `Show a campaign's live counters, its rates and its stored report, with the
recipients counted by status and by bounce classification. The report is
recounted from time to time; when it is missing or old the request asks for a
recount and shows what is stored, so check again later for new numbers. A rate
that cannot be computed yet shows "-" in table output, is left out of plain
output and is an empty cell in CSV output.`,
		Example: `  # Show a campaign's report
  ahasend campaigns stats 123e4567-e89b-12d3-a456-426614174000`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runCampaignsStats,
		SilenceUsage: true,
	}
}

func runCampaignsStats(cmd *cobra.Command, args []string) error {
	return showCampaign(cmd, args[0], "stats", func(c client.AhaSendClient, id uuid.UUID) (*responses.CampaignStats, error) {
		return c.GetCampaignStats(id)
	}, printer.ResponseHandler.HandleCampaignStats, printer.SingleConfig{
		SuccessMessage: "Campaign stats retrieved successfully",
		EmptyMessage:   "No campaign stats found",
	})
}
