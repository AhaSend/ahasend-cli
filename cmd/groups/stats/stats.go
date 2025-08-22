package stats

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the stats command group
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "View email statistics and reporting",
		Long: `View comprehensive email statistics and reports including deliverability,
bounce analysis, and delivery time performance metrics.

Statistics can be filtered by time range, domain, and grouped by various periods
(hour, day, week, month). Data can be exported to CSV format for further analysis.

Common workflow:
  1. View deliverability stats: ahasend stats deliverability
  2. Check bounce statistics: ahasend stats bounces
  3. Monitor delivery times: ahasend stats delivery-time
  4. Export to CSV: ahasend stats deliverability --output csv > stats.csv`,
	}

	// Add subcommands
	cmd.AddCommand(NewDeliverabilityCommand())
	cmd.AddCommand(NewBouncesCommand())
	cmd.AddCommand(NewDeliveryTimeCommand())

	return cmd
}
