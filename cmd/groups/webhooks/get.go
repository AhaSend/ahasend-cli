package webhooks

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the get command
func NewGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <webhook-id>",
		Short: "Get detailed information about a specific webhook",
		Long: `Get detailed information about a specific webhook including its
configuration, event types, status, and metadata.

This command shows comprehensive webhook details including:
- Basic information (name, URL, status)
- Configured event types and descriptions
- Optional settings (scope, domain restrictions)
- Timestamps (created, last updated)
- Complete webhook configuration

The webhook ID can be found using the 'ahasend webhooks list' command.`,
		Example: `  # Get webhook details
  ahasend webhooks get abcd1234-5678-90ef-abcd-1234567890ab

  # Get webhook details in JSON format
  ahasend webhooks get abcd1234-5678-90ef-abcd-1234567890ab --output json

  # Get webhook configuration for backup/restore
  ahasend webhooks get abcd1234-5678-90ef-abcd-1234567890ab --output json > webhook-backup.json`,
		Args:         cobra.ExactArgs(1),
		RunE:         runWebhooksGet,
		SilenceUsage: true,
	}

	return cmd
}

func runWebhooksGet(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	webhookID := args[0]

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"webhook_id": webhookID,
	}).Debug("Executing webhooks get command")

	// Get the webhook
	webhook, err := getWebhook(client, webhookID)
	if err != nil {
		return err
	}

	// Use the new ResponseHandler to display single webhook
	return handler.HandleSingleWebhook(webhook, printer.SingleConfig{
		SuccessMessage: fmt.Sprintf("Retrieved webhook: %s", webhook.Name),
		FieldOrder:     []string{"id", "name", "url", "enabled", "event_types", "scope", "domains", "created_at", "updated_at"},
	})
}

func getWebhook(client client.AhaSendClient, webhookID string) (*responses.Webhook, error) {
	webhook, err := client.GetWebhook(webhookID)
	if err != nil {
		return nil, err
	}
	return webhook, nil
}
