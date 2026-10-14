package webhooks

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the get command
func NewGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <webhook-id>",
		Short: "Get a webhook",
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
		Args:         cmdutil.ExactArgs(1),
		RunE:         runWebhooksGet,
		SilenceUsage: true,
	}

	return cmd
}

func runWebhooksGet(cmd *cobra.Command, args []string) error {
	webhookID := args[0]
	webhookUUID, err := validation.ParseUUIDField("webhook ID", webhookID)
	if err != nil {
		return err
	}

	handler := printer.GetResponseHandlerFromCommand(cmd)
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"webhook_id": webhookID,
	}).Debug("Executing webhooks get command")

	// Get the webhook
	webhook, err := client.GetWebhook(webhookUUID)
	if err != nil {
		return err
	}

	// Use the new ResponseHandler to display single webhook
	return handler.HandleSingleWebhook(webhook, printer.SingleConfig{
		SuccessMessage: "Webhook retrieved successfully",
	})
}
