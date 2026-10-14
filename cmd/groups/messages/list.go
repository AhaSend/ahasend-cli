package messages

import (
	"fmt"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewListCommand creates the list command
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List messages",
		Long: `List messages, with cursor pagination. Filter by sender, recipient,
subject, status, message ID, tag and date range.

--sender matches the sender address exactly. --recipient and --subject match
every message whose recipient or subject contains the value, case-sensitively:
--recipient user@example.com also matches olduser@example.com.

--status can be used multiple times: --status delivered --status bounced.
Valid statuses: received, delivered, opened, clicked, deferred, bounced,
failed, suppressed, sandbox delivered, sandbox deferred, sandbox failed,
sandbox bounced, sandbox suppressed. Delivered also matches opened and
clicked messages, and opened also matches clicked ones.

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

  # List messages with specific tags
  ahasend messages list --tag welcome --tag onboarding

  # List with multiple filters
  ahasend messages list --sender noreply@example.com --recipient user@example.com --subject "Welcome" --status delivered --status deferred

  # List with pagination (limit results)
  ahasend messages list --limit 10

  # Export to JSON
  ahasend messages list --output json`,
		Args:         cmdutil.NoArgs,
		RunE:         runMessagesList,
		SilenceUsage: true,
	}

	// Filter parameters
	cmd.Flags().String("sender", "", "Filter by sender email address (must be from your domain)")
	cmd.Flags().String("recipient", "", "Filter by recipient: addresses that contain the value, such as @example.com")
	cmd.Flags().String("subject", "", "Filter by subject text (matches subjects that contain it)")
	cmd.Flags().String("message-id", "", "Filter by message ID header")
	cmdutil.AddRepeatableOneOfFlag(cmd, "status", "Filter by message status: %s (can be used multiple times)", messageStatuses)
	cmdutil.AddRepeatableFlag(cmd, "tag", "Filter by tag, at most 20, none with a comma (can be used multiple times)")
	cmdutil.AddFlagAlias(cmd, "tags", "tag")
	cmd.Flags().String("from-time", "", "Filter messages created after this time (RFC3339 or relative like '24h', '7d')")
	cmd.Flags().String("to-time", "", "Filter messages created before this time (RFC3339 or relative)")

	// Pagination parameters
	cmdutil.AddPaginationFlags(cmd)
	cmdutil.AddLegacyCursorAlias(cmd)

	// Display options
	cmd.Flags().Bool("show-details", false, "Show detailed message information")

	return cmd
}

// messageStatuses are the values of --status, as users type them. The API
// spells each with every word capitalized, such as "Sandbox Delivered".
var messageStatuses = []string{
	"received", "delivered", "opened", "clicked", "deferred", "bounced", "failed", "suppressed",
	"sandbox delivered", "sandbox deferred", "sandbox failed", "sandbox bounced", "sandbox suppressed",
}

// maxTagFilters is the most tags the API filters messages by.
const maxTagFilters = 20

// readMessageStatuses returns the --status values, in the API's spelling, as
// the comma-separated filter the API takes, or nil when there are none.
func readMessageStatuses(cmd *cobra.Command, statuses []string) (*string, error) {
	apiStatuses := make([]string, len(statuses))
	for i, status := range statuses {
		value, err := cmdutil.MatchOneOf(cmd, "status", strings.TrimSpace(status))
		if err != nil {
			return nil, err
		}
		words := strings.Fields(value)
		for j, word := range words {
			words[j] = strings.ToUpper(word[:1]) + word[1:]
		}
		apiStatuses[i] = strings.Join(words, " ")
	}
	return cmdutil.JoinFilter(apiStatuses), nil
}

func runMessagesList(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	statuses, err := cmdutil.ReadRepeatable(cmd, "status", "")
	if err != nil {
		return err
	}
	tags, err := cmdutil.ReadFilterTags(cmd)
	if err != nil {
		return err
	}
	if len(tags) > maxTagFilters {
		return errors.NewValidationError(fmt.Sprintf("--%s takes at most %d tags, got %d", cmdutil.TypedName(cmd, "tag"), maxTagFilters, len(tags)), nil)
	}
	// The API matches the sender exactly, and a recipient that contains the
	// value, so only the sender must be an address.
	if sender := cmdutil.NonEmptyString(cmd, "sender"); sender != nil {
		if err := validation.ValidateEmail("--sender", *sender); err != nil {
			return err
		}
	}
	showDetails, _ := cmd.Flags().GetBool("show-details")

	pagination, err := cmdutil.ReadPaginationParams(cmd)
	if err != nil {
		return err
	}

	normalizedStatus, err := readMessageStatuses(cmd, statuses)
	if err != nil {
		return err
	}

	fromTime, toTime, err := cmdutil.ReadPastTimeRange(cmd)
	if err != nil {
		return err
	}

	// Get authenticated client
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Log the operation
	logger.Get().WithFields(map[string]interface{}{
		"account_id": client.GetAccountID(),
		"statuses":   statuses,
		"tags":       tags,
		"from_time":  fromTime,
		"to_time":    toTime,
	}).Debug("Listing messages")

	// Build parameters for the client wrapper
	params := requests.GetMessagesParams{
		Status:           normalizedStatus,
		Tags:             tags,
		Sender:           cmdutil.NonEmptyString(cmd, "sender"),
		Recipient:        cmdutil.NonEmptyString(cmd, "recipient"),
		Subject:          cmdutil.NonEmptyString(cmd, "subject"),
		MessageIDHeader:  cmdutil.NonEmptyString(cmd, "message-id"),
		FromTime:         fromTime,
		ToTime:           toTime,
		PaginationParams: pagination,
	}

	// Execute the request through our client wrapper (includes retry logic and logging)
	response, err := client.GetMessages(params)
	if err != nil {
		return err
	}

	return handler.HandleMessageList(response, printer.ListConfig{
		SuccessMessage: "Messages retrieved successfully",
		EmptyMessage:   "No messages found",
		Detailed:       showDetails,
	})
}
