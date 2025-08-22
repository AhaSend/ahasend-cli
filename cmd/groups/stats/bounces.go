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

// NewBouncesCommand creates the stats bounces command
func NewBouncesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bounces",
		Short: "View email bounce statistics and analysis",
		Long: `View detailed email bounce statistics with classification and trend analysis.

Bounce statistics show how many messages bounced, categorized by specific bounce reasons
to help identify and troubleshoot delivery issues faster.

The command provides bounce trends over time, top bounce domains, and detailed
classification analysis to help improve email deliverability.

Bounce Classifications:
- AuthenticationFailed: Message rejected due to DMARC or authentication issues
- BadDomain: The recipient domain doesn't exist
- DNSFailure: The domain's MX record is invalid
- InactiveMailbox: The mailbox provider has deactivated the email address
- InvalidRecipient: The email address doesn't exist
- PolicyRelated: Blocked due to recipient server policies (spam/blocklists)
- ProtocolErrors: SMTP communication issues with recipient mail server
- QuotaIssues: The recipient's mailbox is full
- RoutingErrors: The recipient mail server couldn't route the email
- TransientFailure: The recipient server temporarily rejected the message
- Uncategorized: Other bounce types not specifically categorized`,
		Example: `  # View bounce trends (default view)
  ahasend stats bounces --from-time 7d

  # View classification summary breakdown
  ahasend stats bounces --classification --from-time 7d

  # Export raw data to CSV (ideal for further analysis)
  ahasend stats bounces --raw --from-time 30d --output csv

  # View trends with hourly grouping
  ahasend stats bounces --trends --from-time 24h --group-by hour

  # Filter classification view by domain
  ahasend stats bounces --classification \
    --sender-domain example.com \
    --from-time 7d`,
		RunE: runBounceStats,
	}

	// Time range flags (inherit from deliverability pattern)
	cmd.Flags().String("from-time", "7d", "Start time (RFC3339 format or relative like '7d', '24h')")
	cmd.Flags().String("to-time", "", "End time (RFC3339 format or relative, defaults to now)")

	// Filtering flags
	cmd.Flags().String("group-by", "day", "Group results by: hour, day, week, month")
	cmd.Flags().String("sender-domain", "", "Filter by sender domain")
	cmd.Flags().StringSlice("recipient-domain", []string{}, "Filter by recipient domains (can be used multiple times)")
	cmd.Flags().String("tags", "", "Filter by message tags (comma-separated)")

	// View mode flags (mutually exclusive)
	cmd.Flags().Bool("raw", false, "Show raw data without interpretation (useful for CSV/JSON)")
	cmd.Flags().Bool("classification", false, "Show classification summary breakdown")
	cmd.Flags().Bool("trends", false, "Show time-period focused trends (default)")

	// Additional analysis flags
	cmd.Flags().Bool("show-domains", false, "Show top bouncing recipient domains")
	cmd.Flags().Bool("show-totals", true, "Show summary totals")

	return cmd
}

func runBounceStats(cmd *cobra.Command, args []string) error {
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

	// Note: View mode flags (raw, classification, trends) are handled by the ResponseHandler
	// which provides consistent output across all formats

	// Parse time parameters (reuse logic from deliverability)
	var fromTime *time.Time
	if fromTimeStr != "" {
		parsedTime, err := output.ParseTimePast(fromTimeStr)
		if err == nil {
			fromTime = &parsedTime
		}
		if err != nil {
			return handler.HandleError(errors.NewValidationError(fmt.Sprintf("invalid from-time: %v", err), nil))
		}
	} else {
		f := time.Now().Add(-30 * 24 * time.Hour)
		fromTime = &f
	}

	var toTime *time.Time
	if toTimeStr != "" {
		parsed, err := output.ParseTimePast(toTimeStr)
		if err != nil {
			return handler.HandleError(errors.NewValidationError(fmt.Sprintf("invalid to-time: %v", err), nil))
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
	}).Debug("Fetching bounce statistics")

	// Build request parameters
	params := requests.GetBounceStatisticsParams{
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
	response, err := client.GetBounceStatistics(params)
	if err != nil {
		return handler.HandleError(errors.NewAPIError("failed to get bounce statistics", err))
	}

	// Use the new ResponseHandler to display bounce statistics
	return handler.HandleBounceStats(response, printer.StatsConfig{
		Title:      "Bounce Statistics",
		ShowChart:  false, // Complex bounce data doesn't work well with simple charts
		FieldOrder: []string{"time_bucket", "classification", "count", "percentage", "description"},
	})

}
