package webhooks

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewDeleteCommand creates the delete command
func NewDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <webhook-id>",
		Short: "Delete a webhook",
		Long: `Delete a webhook permanently.

This action cannot be undone. Once deleted, the webhook will no longer
receive event notifications and cannot be restored.

Delete asks for confirmation unless --force is set.`,
		Example: `  # Delete webhook with confirmation prompt
  ahasend webhooks delete abcd1234-5678-90ef-abcd-1234567890ab

  # Delete webhook without confirmation
  ahasend webhooks delete abcd1234-5678-90ef-abcd-1234567890ab --force

  # Delete webhook with JSON output
  ahasend webhooks delete abcd1234-5678-90ef-abcd-1234567890ab --force --output json`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runWebhooksDelete,
		SilenceUsage: true,
	}

	prompt.AddForceFlag(cmd)

	return cmd
}

func runWebhooksDelete(cmd *cobra.Command, args []string) error {
	webhookID := args[0]
	webhookUUID, err := validation.ParseUUIDField("webhook ID", webhookID)
	if err != nil {
		return err
	}
	if err := prompt.Confirm(cmd, fmt.Sprintf("Are you sure you want to delete webhook %s?", webhookID)); err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"webhook_id": webhookID,
	}).Debug("Executing webhooks delete command")

	response, err := client.DeleteWebhook(webhookUUID)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleSuccessResponse(response, printer.ResultConfig{
		SuccessMessage: fmt.Sprintf("Webhook '%s' deleted successfully", webhookID),
	})
}
