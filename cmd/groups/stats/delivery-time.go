package stats

import (
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
)

// NewDeliveryTimeCommand creates the stats delivery-time command
func NewDeliveryTimeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delivery-time",
		Short: "View email delivery time performance metrics",
		Long: `View detailed email delivery time statistics and performance metrics.

Delivery time statistics show how long it takes for emails to be delivered
after they are sent, helping identify performance bottlenecks and optimize
sending strategies.

Performance metrics include:
- Average delivery time per time period
- Performance by recipient domain (Gmail, Outlook, etc.)

This data helps optimize send times, identify slow-delivering domains,
and improve overall email delivery performance.`,
		Example: `  # View delivery times for last 7 days
  ahasend stats delivery-time --from-time 7d

  # View hourly performance metrics
  ahasend stats delivery-time --from-time 24h --group-by hour

  # Performance by recipient domain
  ahasend stats delivery-time \
    --from-time 7d \
    --recipient-domain gmail.com \
    --recipient-domain outlook.com

  # Export to CSV for analysis
  ahasend stats delivery-time --from-time 30d --output csv

  # JSON output for automation
  ahasend stats delivery-time \
    --from-time 7d \
    --output json`,
		Args:         cmdutil.NoArgs,
		RunE:         runDeliveryTimeStats,
		SilenceUsage: true,
	}

	addFilterFlags(cmd)
	addNoEffectFlag(cmd, "raw")
	addNoEffectFlag(cmd, "show-totals")

	return cmd
}

func runDeliveryTimeStats(cmd *cobra.Command, args []string) error {
	return runStats(cmd, "Delivery Time Statistics", func(c client.AhaSendClient, filters statsFilters) (*responses.DeliveryTimeStatisticsResponse, error) {
		return c.GetDeliveryTimeStatistics(requests.GetDeliveryTimeStatisticsParams(filters))
	}, printer.ResponseHandler.HandleDeliveryTimeStats)
}
