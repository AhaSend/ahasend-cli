package messages

import (
	"fmt"
	"strings"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/output"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	ahasend "github.com/AhaSend/ahasend-go"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// NewListCommand creates the list command
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List messages",
		Long: `List messages with filtering and pagination support.

List messages with optional filtering by sender, recipient, subject, status, message ID, and date range.

Status filtering supports single or multiple flags:
  - Single: --status delivered
  - Multiple: --status delivered --status bounced --status failed
  - Valid statuses: received, delivered, deferred, bounced, failed, suppressed, sandbox delivered, sandbox deferred, sandbox failed, sandbox bounced, sandbox suppressed

Date/time values should be in RFC3339 format (e.g., 2024-01-15T10:30:00Z).
For relative times, you can use:
  - "1h" for 1 hour ago
  - "24h" for 24 hours ago
  - "7d" for 7 days ago
  - "30d" for 30 days ago`,
		Example: `  # List all messages in account
  ahasend messages list

  # List all messages from a specific sender
  ahasend messages list --sender noreply@example.com

  # List messages to a specific recipient
  ahasend messages list --recipient user@example.com

  # List messages with subject filter
  ahasend messages list --subject "Welcome"

  # List messages by status
  ahasend messages list --status delivered

  # List messages with multiple statuses
  ahasend messages list --status delivered --status bounced

  # List messages from the last 24 hours
  ahasend messages list --from-time 24h

  # List messages between specific dates
  ahasend messages list --from-time 2024-01-01T00:00:00Z --to-time 2024-01-31T23:59:59Z

  # List with multiple filters
  ahasend messages list --sender noreply@example.com --recipient user@example.com --subject "Welcome" --status delivered --status deferred

  # List with pagination (limit results)
  ahasend messages list --limit 10

  # Export to JSON
  ahasend messages list --output json`,
		RunE:         runMessagesList,
		SilenceUsage: true,
	}

	// Filter parameters
	cmd.Flags().String("sender", "", "Sender email address (must be from your domain)")
	cmd.Flags().String("recipient", "", "Filter by recipient email address")
	cmd.Flags().String("subject", "", "Filter by subject text (partial match)")
	cmd.Flags().String("message-id", "", "Filter by message ID header")
	cmd.Flags().StringSlice("status", []string{}, "Filter by message status (can be used multiple times)")
	cmd.Flags().String("from-time", "", "Filter messages created after this time (RFC3339 or relative like '24h', '7d')")
	cmd.Flags().String("to-time", "", "Filter messages created before this time (RFC3339 or relative)")

	// Pagination parameters
	cmd.Flags().Int("limit", 100, "Maximum number of messages to return (1-100)")
	cmd.Flags().String("cursor", "", "Pagination cursor for next page")

	// Display options
	cmd.Flags().Bool("show-details", false, "Show detailed message information")

	return cmd
}

func runMessagesList(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	// Get authenticated client
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Parse the account ID from client
	accountID, err := uuid.Parse(client.GetAccountID())
	if err != nil {
		return errors.NewConfigError("invalid account ID", err)
	}

	// Get flags
	sender, _ := cmd.Flags().GetString("sender")
	recipient, _ := cmd.Flags().GetString("recipient")
	subject, _ := cmd.Flags().GetString("subject")
	messageID, _ := cmd.Flags().GetString("message-id")
	statuses, _ := cmd.Flags().GetStringSlice("status")
	fromTimeStr, _ := cmd.Flags().GetString("from-time")
	toTimeStr, _ := cmd.Flags().GetString("to-time")
	limit, _ := cmd.Flags().GetInt("limit")
	cursor, _ := cmd.Flags().GetString("cursor")
	showDetails, _ := cmd.Flags().GetBool("show-details")

	// Validate limit
	if limit < 1 || limit > 100 {
		return errors.NewValidationError("limit must be between 1 and 100", nil)
	}

	// Validate and normalize status if provided
	var normalizedStatus string
	if len(statuses) > 0 {
		// Map of user-friendly input to API format
		statusMap := map[string]string{
			"received":           "Received",
			"delivered":          "Delivered",
			"deferred":           "Deferred",
			"bounced":            "Bounced",
			"failed":             "Failed",
			"suppressed":         "Suppressed",
			"sandbox delivered":  "Sandbox Delivered",
			"sandbox deferred":   "Sandbox Deferred",
			"sandbox failed":     "Sandbox Failed",
			"sandbox bounced":    "Sandbox Bounced",
			"sandbox suppressed": "Sandbox Suppressed",
		}

		var normalizedList []string
		for _, s := range statuses {
			s = strings.TrimSpace(strings.ToLower(s))
			if apiStatus, valid := statusMap[s]; valid {
				normalizedList = append(normalizedList, apiStatus)
			} else {
				validInputs := make([]string, 0, len(statusMap))
				for k := range statusMap {
					validInputs = append(validInputs, k)
				}
				return errors.NewValidationError(fmt.Sprintf("invalid status '%s'. Valid statuses: %s", s, strings.Join(validInputs, ", ")), nil)
			}
		}
		normalizedStatus = strings.Join(normalizedList, ",")
	}

	// Parse time filters
	var fromTime, toTime *time.Time
	if fromTimeStr != "" {
		t, err := output.ParseTimePast(fromTimeStr)
		if err != nil {
			return err
		}
		fromTime = &t
	}

	if toTimeStr != "" {
		t, err := output.ParseTimePast(toTimeStr)
		if err != nil {
			return err
		}
		toTime = &t
	}

	// Log the operation
	logger.Get().WithFields(map[string]interface{}{
		"account_id": accountID.String(),
		"sender":     sender,
		"recipient":  recipient,
		"subject":    subject,
		"message_id": messageID,
		"statuses":   statuses,
		"from_time":  fromTime,
		"to_time":    toTime,
		"limit":      limit,
		"cursor":     cursor,
	}).Debug("Listing messages")

	// Build parameters for the client wrapper
	params := requests.GetMessagesParams{
		Status:          ahasend.String(normalizedStatus),
		Sender:          ahasend.String(sender),
		Recipient:       ahasend.String(recipient),
		Subject:         ahasend.String(subject),
		MessageIDHeader: ahasend.String(messageID),
		FromTime:        fromTime,
		ToTime:          toTime,
		Limit:           ahasend.Int32(int32(limit)),
		Cursor:          ahasend.String(cursor),
	}

	// Execute the request through our client wrapper (includes retry logic and logging)
	response, err := client.GetMessages(params)
	if err != nil {
		return err
	}

	// Use the new ResponseHandler to display message list
	fieldOrder := []string{"api_id", "sender", "recipient", "subject", "status", "created", "delivered", "opens", "clicks"}
	if showDetails {
		fieldOrder = append(fieldOrder, "message_id", "direction", "domain_id", "attempts", "tags", "bounce_class", "retain_until")
	}

	return handler.HandleMessageList(response, printer.ListConfig{
		SuccessMessage: "Messages retrieved successfully",
		EmptyMessage:   "No messages found matching criteria",
		ShowPagination: true,
		FieldOrder:     fieldOrder,
	})
}
