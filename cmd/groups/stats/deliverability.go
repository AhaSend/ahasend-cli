package stats

import (
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
)

// NewDeliverabilityCommand creates the stats deliverability command
func NewDeliverabilityCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deliverability",
		Short: "View email deliverability statistics",
		Long: `View email deliverability statistics. Each period shows how many messages
were received, delivered, deferred, bounced, failed, suppressed, opened and
clicked, and the delivery rate (delivered out of received) and open rate
(opened out of delivered) the CLI works out from them.

Statistics can be filtered by time range, domain and tag, and grouped by
different periods.

Time ranges can be specified using RFC3339 format or relative formats:
- RFC3339: "2024-01-15T00:00:00Z"
- Relative: "1h", "24h", "7d", "30d" (ago, such as 7d for 7 days ago)

Grouping options:
- hour: Group by hour
- day: Group by day (default)
- week: Group by week
- month: Group by month`,
		Example: `  # View deliverability for last 7 days
  ahasend stats deliverability --from-time 7d

  # View statistics for specific date range
  ahasend stats deliverability \
    --from-time "2024-01-15T00:00:00Z" \
    --to-time "2024-01-16T00:00:00Z"

  # Group by hour and filter by domain
  ahasend stats deliverability \
    --from-time 24h \
    --group-by hour \
    --sender-domain example.com

  # Export to CSV
  ahasend stats deliverability --from-time 30d --output csv

  # View recipient domain breakdown
  ahasend stats deliverability \
    --from-time 7d \
    --recipient-domain gmail.com \
    --recipient-domain googlemail.com`,
		Args:         cmdutil.NoArgs,
		RunE:         runDeliverabilityStats,
		SilenceUsage: true,
	}

	addFilterFlags(cmd)
	addNoEffectFlag(cmd, "chart")
	addNoEffectFlag(cmd, "raw")
	addNoEffectFlag(cmd, "show-totals")

	return cmd
}

func runDeliverabilityStats(cmd *cobra.Command, args []string) error {
	return runStats(cmd, "Deliverability Statistics", func(c client.AhaSendClient, filters statsFilters) (*responses.DeliverabilityStatisticsResponse, error) {
		return c.GetDeliverabilityStatistics(requests.GetDeliverabilityStatisticsParams(filters))
	}, printer.ResponseHandler.HandleDeliverabilityStats)
}
