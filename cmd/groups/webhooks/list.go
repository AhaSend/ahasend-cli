package webhooks

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
)

// NewListCommand creates the list command
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all webhooks",
		Long: `List all webhooks in your AhaSend account with their configuration,
event types, and status information.

The list shows webhook names, URLs, enabled status, and configured event types
to help you manage your webhook endpoints effectively.`,
		Example: `  # List all webhooks
  ahasend webhooks list

  # List webhooks with JSON output
  ahasend webhooks list --output json

  # List webhooks with pagination
  ahasend webhooks list --limit 10

  # List only enabled webhooks
  ahasend webhooks list --enabled`,
		RunE:         runWebhooksList,
		SilenceUsage: true,
	}

	cmd.Flags().Int32("limit", 0, "Maximum number of webhooks to return")
	cmd.Flags().String("cursor", "", "Pagination cursor for next page")
	cmd.Flags().Bool("enabled", false, "Show only enabled webhooks")

	return cmd
}

func runWebhooksList(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Get flags
	limit, _ := cmd.Flags().GetInt32("limit")
	cursor, _ := cmd.Flags().GetString("cursor")
	enabledOnly, _ := cmd.Flags().GetBool("enabled")

	// Handle pagination
	var limitPtr *int32
	var cursorPtr *string

	if limit > 0 {
		limitPtr = &limit
	}
	if cursor != "" {
		cursorPtr = &cursor
	}

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"limit":        limit,
		"cursor":       cursor,
		"enabled_only": enabledOnly,
	}).Debug("Executing webhooks list command")

	// Fetch webhooks
	response, err := client.ListWebhooks(limitPtr, cursorPtr)
	if err != nil {
		return handler.HandleError(err)
	}

	// Filter by enabled status if specified (client-side filtering)
	if enabledOnly && response != nil && response.Data != nil {
		var filteredWebhooks []responses.Webhook
		for _, webhook := range response.Data {
			if webhook.Enabled {
				filteredWebhooks = append(filteredWebhooks, webhook)
			}
		}
		// Update response with filtered data
		response.Data = filteredWebhooks
	}

	// Use the new ResponseHandler to display webhooks list
	emptyMessage := "No webhooks found"
	if enabledOnly {
		emptyMessage = "No enabled webhooks found"
	}

	return handler.HandleWebhookList(response, printer.ListConfig{
		EmptyMessage: emptyMessage,
		FieldOrder:   []string{"id", "name", "url", "enabled", "event_types", "scope", "domains", "created_at", "updated_at"},
	})
}
