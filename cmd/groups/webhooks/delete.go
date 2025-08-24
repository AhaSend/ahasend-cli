package webhooks

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/output"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
)

// NewDeleteCommand creates the delete command
func NewDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <webhook-id>",
		Short: "Delete a webhook",
		Long: `Delete an existing webhook endpoint permanently.

This action cannot be undone. Once deleted, the webhook will no longer
receive event notifications and cannot be restored.

By default, you will be prompted to confirm the deletion. Use the --force
flag to skip the confirmation prompt for automated scripts.

The webhook ID can be found using the 'ahasend webhooks list' command.`,
		Example: `  # Delete webhook with confirmation prompt
  ahasend webhooks delete abcd1234-5678-90ef-abcd-1234567890ab

  # Delete webhook without confirmation
  ahasend webhooks delete abcd1234-5678-90ef-abcd-1234567890ab --force

  # Delete webhook with JSON output
  ahasend webhooks delete abcd1234-5678-90ef-abcd-1234567890ab --output json`,
		Args:         cobra.ExactArgs(1),
		RunE:         runWebhooksDelete,
		SilenceUsage: true,
	}

	cmd.Flags().Bool("force", false, "Skip confirmation prompt")

	return cmd
}

func runWebhooksDelete(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	webhookID := args[0]
	force, _ := cmd.Flags().GetBool("force")

	// Get webhook details for confirmation (unless force is used)
	var webhook *responses.Webhook
	if !force {
		webhook, err = getWebhookForConfirmation(client, webhookID)
		if err != nil {
			return err
		}

		// Show webhook details and ask for confirmation
		if !confirmDeletion(webhook) {
			return handler.HandleSimpleSuccess("Webhook deletion cancelled")
		}
	}

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"webhook_id": webhookID,
		"force":      force,
	}).Debug("Executing webhooks delete command")

	// Delete the webhook
	err = deleteWebhook(client, webhookID)
	if err != nil {
		return err
	}

	// Use the new ResponseHandler to display deletion success
	successMsg := "Successfully deleted webhook"
	if webhook != nil {
		successMsg = fmt.Sprintf("Successfully deleted webhook: %s", webhook.Name)
	}

	return handler.HandleDeleteWebhook(true, printer.DeleteConfig{
		SuccessMessage: successMsg,
		ItemName:       "webhook",
	})
}

func getWebhookForConfirmation(client client.AhaSendClient, webhookID string) (*responses.Webhook, error) {
	webhook, err := client.GetWebhook(webhookID)
	if err != nil {
		return nil, err
	}
	return webhook, nil
}

func confirmDeletion(webhook *responses.Webhook) bool {
	fmt.Printf("You are about to delete the following webhook:\n\n")
	fmt.Printf("  Name:         %s\n", webhook.Name)
	fmt.Printf("  ID:           %s\n", webhook.ID)
	fmt.Printf("  URL:          %s\n", webhook.URL)
	fmt.Printf("  Status:       %s\n", getWebhookStatus(webhook.Enabled))

	events := getConfiguredEvents(webhook)
	if len(events) > 0 {
		fmt.Printf("  Event Types:  %s\n", strings.Join(events, ", "))
	}

	fmt.Printf("  Created:      %s\n", output.FormatTimeLocalValue(webhook.CreatedAt))
	fmt.Printf("\n")

	fmt.Printf("⚠️  This action cannot be undone!")
	fmt.Printf("\nDo you want to delete this webhook? (y/N): ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

func deleteWebhook(client client.AhaSendClient, webhookID string) error {
	err := client.DeleteWebhook(webhookID)
	if err != nil {
		return err
	}
	return nil
}

// Helper functions for webhook status and events
func getWebhookStatus(enabled bool) string {
	if enabled {
		return "Enabled"
	}
	return "Disabled"
}

func getConfiguredEvents(webhook *responses.Webhook) []string {
	var events []string
	if webhook.OnReception {
		events = append(events, "reception")
	}
	if webhook.OnDelivered {
		events = append(events, "delivered")
	}
	if webhook.OnTransientError {
		events = append(events, "transient_error")
	}
	if webhook.OnFailed {
		events = append(events, "failed")
	}
	if webhook.OnBounced {
		events = append(events, "bounced")
	}
	if webhook.OnSuppressed {
		events = append(events, "suppressed")
	}
	if webhook.OnOpened {
		events = append(events, "opened")
	}
	if webhook.OnClicked {
		events = append(events, "clicked")
	}
	if webhook.OnSuppressionCreated {
		events = append(events, "suppression_created")
	}
	if webhook.OnDNSError {
		events = append(events, "dns_error")
	}
	return events
}
