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

  # Export raw data to CSV for analysis
  ahasend stats delivery-time \
    --from-time 30d \
    --raw \
    --output csv

  # JSON output for automation
  ahasend stats delivery-time \
    --from-time 7d \
    --output json`,
		RunE: runDeliveryTimeStats,
	}

	// Time range flags
	cmd.Flags().String("from-time", "7d", "Start time (RFC3339 format or relative like '7d', '24h')")
	cmd.Flags().String("to-time", "", "End time (RFC3339 format or relative, defaults to now)")

	// Filtering flags
	cmd.Flags().String("group-by", "day", "Group results by: hour, day, week, month")
	cmd.Flags().String("sender-domain", "", "Filter by sender domain")
	cmd.Flags().StringSlice("recipient-domain", []string{}, "Filter by recipient domains (can be used multiple times)")
	cmd.Flags().String("tags", "", "Filter by message tags (comma-separated)")

	// Analysis flags
	cmd.Flags().Bool("raw", false, "Show raw data without interpretation (useful for CSV/JSON)")
	cmd.Flags().Bool("show-totals", true, "Show summary statistics")

	return cmd
}

func runDeliveryTimeStats(cmd *cobra.Command, args []string) error {
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

	// Parse time parameters
	fromTimeValue, err := output.ParseTimePast(fromTimeStr)
	if err != nil {
		return errors.NewValidationError(fmt.Sprintf("invalid from-time: %v", err), nil)
	}
	fromTime := &fromTimeValue

	var toTime *time.Time
	if toTimeStr != "" {
		parsed, err := output.ParseTimePast(toTimeStr)
		if err != nil {
			return errors.NewValidationError(fmt.Sprintf("invalid to-time: %v", err), nil)
		}
		toTime = &parsed
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
	}).Debug("Fetching delivery time statistics")

	// Build request parameters
	params := requests.GetDeliveryTimeStatisticsParams{
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
	response, err := client.GetDeliveryTimeStatistics(params)
	if err != nil {
		return errors.NewAPIError("failed to get delivery time statistics", err)
	}

	// Use the new ResponseHandler to display delivery time statistics
	return handler.HandleDeliveryTimeStats(response, printer.StatsConfig{
		Title:      "Delivery Time Statistics",
		ShowChart:  false, // Delivery time data is better in tabular format
		FieldOrder: []string{"time_bucket", "avg_delivery_time", "message_count"},
	})
}

// Helper functions shared across stats commands

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
