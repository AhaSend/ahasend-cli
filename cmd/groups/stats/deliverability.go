package stats

import (
	"fmt"
	"strings"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/output"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewDeliverabilityCommand creates the stats deliverability command
func NewDeliverabilityCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deliverability",
		Short: "View email deliverability statistics",
		Long: `View comprehensive email deliverability statistics including sent, delivered,
bounced, and rejected message counts.

Statistics can be filtered by time range, domain, tags and grouped by different periods.
The command provides ASCII charts for visual representation and supports CSV export.

Time ranges can be specified using RFC3339 format or relative formats:
- RFC3339: "2024-01-15T00:00:00Z"
- Relative: "1h", "24h", "7d", "30d" (from now)

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

  # Export to CSV with visual chart
  ahasend stats deliverability --from-time 30d --output csv --chart

  # View recipient domain breakdown
  ahasend stats deliverability \
    --from-time 7d \
    --recipient-domain gmail.com \
    --recipient-domain googlemail.com`,
		RunE: runDeliverabilityStats,
	}

	// Time range flags
	cmd.Flags().String("from-time", "7d", "Start time (RFC3339 format or relative like '7d', '24h')")
	cmd.Flags().String("to-time", "", "End time (RFC3339 format or relative, defaults to now)")

	// Filtering flags
	cmd.Flags().String("group-by", "day", "Group results by: hour, day, week, month")
	cmd.Flags().String("sender-domain", "", "Filter by sender domain")
	cmd.Flags().StringSlice("recipient-domain", []string{}, "Filter by recipient domains (can be used multiple times)")
	cmd.Flags().String("tags", "", "Filter by message tags (comma-separated)")

	// Display flags
	cmd.Flags().Bool("chart", false, "Show ASCII chart visualization")
	cmd.Flags().Bool("raw", false, "Show raw data without interpretation (useful for CSV/JSON)")
	cmd.Flags().Bool("show-totals", true, "Show summary totals")

	return cmd
}

func runDeliverabilityStats(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Get flag values
	fromTimeStr, _ := cmd.Flags().GetString("from-time")
	toTimeStr, _ := cmd.Flags().GetString("to-time")
	groupBy, _ := cmd.Flags().GetString("group-by")
	senderDomain, _ := cmd.Flags().GetString("sender-domain")
	recipientDomains, _ := cmd.Flags().GetStringSlice("recipient-domain")
	tags, _ := cmd.Flags().GetString("tags")
	showChart, _ := cmd.Flags().GetBool("chart")

	// Parse time parameters
	var fromTime *time.Time
	if fromTimeStr != "" {
		parsedTime, err := output.ParseTimePast(fromTimeStr)
		if err == nil {
			fromTime = &parsedTime
		}
		if err != nil {
			return errors.NewValidationError(fmt.Sprintf("invalid from-time: %v", err), nil)
		}
	} else {
		f := time.Now().Add(-30 * 24 * time.Hour)
		fromTime = &f
	}

	var toTime *time.Time
	if toTimeStr != "" {
		parsed, err := output.ParseTimePast(toTimeStr)
		if err != nil {
			return errors.NewValidationError(fmt.Sprintf("invalid to-time: %v", err), nil)
		}
		toTime = &parsed
	} else {
		n := time.Now()
		toTime = &n
	}

	// Validate group-by parameter
	validGroupBy := []string{"hour", "day", "week", "month"}
	if !contains(validGroupBy, groupBy) {
		return handler.HandleError(errors.NewValidationError(fmt.Sprintf("invalid group-by '%s', must be one of: %s",
			groupBy, strings.Join(validGroupBy, ", ")), nil))
	}

	logger.Get().WithFields(map[string]interface{}{
		"from_time":         fromTime,
		"to_time":           toTime,
		"group_by":          groupBy,
		"sender_domain":     senderDomain,
		"recipient_domains": recipientDomains,
	}).Debug("Fetching deliverability statistics")

	// Build request parameters
	params := requests.GetDeliverabilityStatisticsParams{
		FromTime: fromTime,
		ToTime:   toTime,
		GroupBy:  &groupBy,
	}

	if senderDomain != "" {
		params.SenderDomain = &senderDomain
	}
	if len(recipientDomains) > 0 {
		recipientDomainsStr := strings.Join(recipientDomains, ",")
		params.RecipientDomains = &recipientDomainsStr
	}
	if tags != "" {
		params.Tags = &tags
	}

	// Fetch statistics
	response, err := client.GetDeliverabilityStatistics(params)
	if err != nil {
		return errors.NewAPIError("failed to get deliverability statistics", err)
	}

	// Use the new ResponseHandler to display deliverability statistics
	return handler.HandleDeliverabilityStats(response, printer.StatsConfig{
		Title:      "Deliverability Statistics",
		ShowChart:  showChart,
		FieldOrder: []string{"time_bucket", "sent", "delivered", "bounced", "rejected", "delivery_rate"},
	})
}
