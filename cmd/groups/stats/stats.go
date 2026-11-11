package stats

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the stats command group
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "View email statistics and reporting",
		Long: `View email statistics and reports including email counts of any period,
deliverability, bounce analysis, and delivery time performance metrics.

Statistics can be filtered by time range and sender domain, and grouped by
periods (hour, day, week, month). Deliverability, bounces and delivery-time
can also be filtered by recipient domain and tag; emails covers the whole
history of the account and can break the counts down by domain. Data can be
exported to CSV format for further analysis.

Common workflow:
  1. View email counts: ahasend stats emails
  2. View deliverability stats: ahasend stats deliverability
  3. Check bounce statistics: ahasend stats bounces
  4. Monitor delivery times: ahasend stats delivery-time
  5. Export to CSV: ahasend stats deliverability --output csv > stats.csv`,
	}

	// Add subcommands
	cmd.AddCommand(NewEmailsCommand())
	cmd.AddCommand(NewDeliverabilityCommand())
	cmd.AddCommand(NewBouncesCommand())
	cmd.AddCommand(NewDeliveryTimeCommand())

	return cmd
}
