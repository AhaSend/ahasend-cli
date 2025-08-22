package routes

import (
	"fmt"
	"net/url"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
)

// NewUpdateCommand creates the update command
func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <route-id>",
		Short: "Update an existing inbound email route",
		Long: `Update an existing inbound email route's configuration including
name, webhook URL, recipient filtering, processing options, and status.

This command allows you to modify any aspect of a route's configuration:
- Route name and webhook URL
- Recipient filtering patterns
- Processing options (attachments, headers, grouping, reply stripping)
- Route status (enabled/disabled)

You can update multiple properties in a single command by combining flags.
Only the specified flags will be updated; unspecified properties remain unchanged.`,
		Example: `  # Update route name
  ahasend routes update abcd1234-5678-90ef-abcd-1234567890ab --name "New Route Name"

  # Update webhook URL
  ahasend routes update abcd1234-5678-90ef-abcd-1234567890ab --url "https://api.example.com/new-webhook"

  # Enable a route
  ahasend routes update abcd1234-5678-90ef-abcd-1234567890ab --enabled

  # Disable a route
  ahasend routes update abcd1234-5678-90ef-abcd-1234567890ab --disabled

  # Update recipient filter
  ahasend routes update abcd1234-5678-90ef-abcd-1234567890ab --recipient "support@*"

  # Clear recipient filter (accept all emails)
  ahasend routes update abcd1234-5678-90ef-abcd-1234567890ab --clear-recipient

  # Enable processing options
  ahasend routes update abcd1234-5678-90ef-abcd-1234567890ab \
    --include-attachments \
    --include-headers \
    --group-by-message-id

  # Disable specific processing options
  ahasend routes update abcd1234-5678-90ef-abcd-1234567890ab \
    --no-include-attachments \
    --no-strip-replies

  # Multiple updates at once
  ahasend routes update abcd1234-5678-90ef-abcd-1234567890ab \
    --name "Updated Route" \
    --url "https://api.example.com/updated" \
    --enabled \
    --include-headers`,
		Args:         cobra.ExactArgs(1),
		RunE:         runRoutesUpdate,
		SilenceUsage: true,
	}

	// Basic properties
	cmd.Flags().String("name", "", "Update route name")
	cmd.Flags().String("url", "", "Update webhook URL")
	cmd.Flags().String("recipient", "", "Update recipient filter pattern")
	cmd.Flags().Bool("clear-recipient", false, "Clear recipient filter (accept all emails)")

	// Status flags
	cmd.Flags().Bool("enabled", false, "Enable the route")
	cmd.Flags().Bool("disabled", false, "Disable the route")

	// Processing option flags - positive
	cmd.Flags().Bool("include-attachments", false, "Enable including attachments in webhook payload")
	cmd.Flags().Bool("include-headers", false, "Enable including headers in webhook payload")
	cmd.Flags().Bool("group-by-message-id", false, "Enable grouping related emails by message ID")
	cmd.Flags().Bool("strip-replies", false, "Enable stripping reply content from emails")

	// Processing option flags - negative
	cmd.Flags().Bool("no-include-attachments", false, "Disable including attachments in webhook payload")
	cmd.Flags().Bool("no-include-headers", false, "Disable including headers in webhook payload")
	cmd.Flags().Bool("no-group-by-message-id", false, "Disable grouping related emails by message ID")
	cmd.Flags().Bool("no-strip-replies", false, "Disable stripping reply content from emails")

	// Mutual exclusion for status
	cmd.MarkFlagsMutuallyExclusive("enabled", "disabled")

	// Mutual exclusion for processing options
	cmd.MarkFlagsMutuallyExclusive("include-attachments", "no-include-attachments")
	cmd.MarkFlagsMutuallyExclusive("include-headers", "no-include-headers")
	cmd.MarkFlagsMutuallyExclusive("group-by-message-id", "no-group-by-message-id")
	cmd.MarkFlagsMutuallyExclusive("strip-replies", "no-strip-replies")

	// Mutual exclusion for recipient filter
	cmd.MarkFlagsMutuallyExclusive("recipient", "clear-recipient")

	return cmd
}

func runRoutesUpdate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	routeID := args[0]

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"route_id": routeID,
	}).Debug("Executing routes update command")

	// Parse and validate update configuration
	config, err := parseUpdateFlags(cmd)
	if err != nil {
		return handler.HandleError(err)
	}

	// Validate that at least one field is being updated
	if !config.HasUpdates() {
		return handler.HandleError(fmt.Errorf("no updates specified. Provide at least one flag to update the route"))
	}

	// Update the route
	route, err := updateRoute(client, routeID, config)
	if err != nil {
		return handler.HandleError(err)
	}

	// Use the new ResponseHandler to display updated route
	return handler.HandleUpdateRoute(route, printer.UpdateConfig{
		SuccessMessage: fmt.Sprintf("Route %s updated successfully", routeID),
		ItemName:       "route",
		FieldOrder:     []string{"id", "name", "url", "enabled", "recipient", "attachments", "headers", "group_by_message_id", "strip_replies", "updated_at"},
	})
}

type RouteUpdateConfig struct {
	Name               *string
	URL                *string
	Recipient          *string
	ClearRecipient     bool
	Enabled            *bool
	IncludeAttachments *bool
	IncludeHeaders     *bool
	GroupByMessageID   *bool
	StripReplies       *bool
}

func (c RouteUpdateConfig) HasUpdates() bool {
	return c.Name != nil ||
		c.URL != nil ||
		c.Recipient != nil ||
		c.ClearRecipient ||
		c.Enabled != nil ||
		c.IncludeAttachments != nil ||
		c.IncludeHeaders != nil ||
		c.GroupByMessageID != nil ||
		c.StripReplies != nil
}

func parseUpdateFlags(cmd *cobra.Command) (RouteUpdateConfig, error) {
	var config RouteUpdateConfig

	// Basic properties
	if cmd.Flags().Changed("name") {
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return config, fmt.Errorf("route name cannot be empty")
		}
		config.Name = &name
	}

	if cmd.Flags().Changed("url") {
		webhookURL, _ := cmd.Flags().GetString("url")
		if webhookURL == "" {
			return config, fmt.Errorf("webhook URL cannot be empty")
		}

		// Validate URL format
		if err := validateURL(webhookURL); err != nil {
			return config, err
		}
		config.URL = &webhookURL
	}

	if cmd.Flags().Changed("recipient") {
		recipient, _ := cmd.Flags().GetString("recipient")
		if recipient == "" {
			return config, fmt.Errorf("recipient filter cannot be empty. Use --clear-recipient to remove filtering")
		}
		config.Recipient = &recipient
	}

	if cmd.Flags().Changed("clear-recipient") {
		clearRecipient, _ := cmd.Flags().GetBool("clear-recipient")
		config.ClearRecipient = clearRecipient
	}

	// Status
	if cmd.Flags().Changed("enabled") {
		enabled := true
		config.Enabled = &enabled
	} else if cmd.Flags().Changed("disabled") {
		enabled := false
		config.Enabled = &enabled
	}

	// Processing options - positive flags
	if cmd.Flags().Changed("include-attachments") {
		includeAttachments := true
		config.IncludeAttachments = &includeAttachments
	} else if cmd.Flags().Changed("no-include-attachments") {
		includeAttachments := false
		config.IncludeAttachments = &includeAttachments
	}

	if cmd.Flags().Changed("include-headers") {
		includeHeaders := true
		config.IncludeHeaders = &includeHeaders
	} else if cmd.Flags().Changed("no-include-headers") {
		includeHeaders := false
		config.IncludeHeaders = &includeHeaders
	}

	if cmd.Flags().Changed("group-by-message-id") {
		groupByMessageID := true
		config.GroupByMessageID = &groupByMessageID
	} else if cmd.Flags().Changed("no-group-by-message-id") {
		groupByMessageID := false
		config.GroupByMessageID = &groupByMessageID
	}

	if cmd.Flags().Changed("strip-replies") {
		stripReplies := true
		config.StripReplies = &stripReplies
	} else if cmd.Flags().Changed("no-strip-replies") {
		stripReplies := false
		config.StripReplies = &stripReplies
	}

	return config, nil
}

func validateURL(webhookURL string) error {
	parsedURL, err := url.Parse(webhookURL)
	if err != nil {
		return fmt.Errorf("invalid webhook URL format: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("webhook URL must use http or https scheme")
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("webhook URL must include a valid host")
	}

	// Security warning for HTTP URLs
	if parsedURL.Scheme == "http" {
		fmt.Println("⚠️  Warning: Using HTTP URL for webhook. Consider using HTTPS for production.")
	}

	return nil
}

func updateRoute(client client.AhaSendClient, routeID string, config RouteUpdateConfig) (*responses.Route, error) {
	// Build update request
	req := requests.UpdateRouteRequest{}

	// Set fields that need updating
	if config.Name != nil {
		req.Name = config.Name
	}

	if config.URL != nil {
		req.URL = config.URL
	}

	if config.Recipient != nil {
		req.Recipient = config.Recipient
	} else if config.ClearRecipient {
		// To clear recipient filter, set it to empty string
		emptyRecipient := ""
		req.Recipient = &emptyRecipient
	}

	if config.Enabled != nil {
		req.Enabled = config.Enabled
	}

	if config.IncludeAttachments != nil {
		req.Attachments = config.IncludeAttachments
	}

	if config.IncludeHeaders != nil {
		req.Headers = config.IncludeHeaders
	}

	if config.GroupByMessageID != nil {
		req.GroupByMessageId = config.GroupByMessageID
	}

	if config.StripReplies != nil {
		req.StripReplies = config.StripReplies
	}

	// Update the route
	route, err := client.UpdateRoute(routeID, req)
	if err != nil {
		return nil, err
	}

	return route, nil
}
