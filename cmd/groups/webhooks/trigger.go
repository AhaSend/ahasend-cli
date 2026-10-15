package webhooks

import (
	"fmt"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-cli/internal/webhooks"
	"github.com/spf13/cobra"
)

// NewTriggerCommand creates the trigger command
func NewTriggerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trigger <webhook-id>",
		Short: "Trigger webhook events for testing",
		Long: `Trigger webhook events for development and testing purposes.

This command allows you to manually trigger webhook events to test your
webhook endpoints without waiting for actual events to occur. This is
particularly useful during development and integration testing.

The webhook ID can be found using the 'ahasend webhooks list' command.

A disabled webhook gets no events: the API answers that they are queued, and
then drops them.`,
		Example: `  # Trigger a single event
  ahasend webhooks trigger abcd1234-5678-90ef-abcd-1234567890ab \
    --events message.delivered

  # Trigger multiple events
  ahasend webhooks trigger abcd1234-5678-90ef-abcd-1234567890ab \
    --events message.delivered --events message.opened --events message.clicked

  # Trigger all available events
  ahasend webhooks trigger abcd1234-5678-90ef-abcd-1234567890ab \
    --all-events`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runWebhooksTrigger,
		SilenceUsage: true,
	}

	// Event selection flags
	cmdutil.AddRepeatableFlag(cmd, "events", "Event type to trigger, such as message.delivered or delivered (can be used multiple times)")
	cmdutil.CompleteValues(cmd, "events", webhooks.EventValues())
	cmd.Flags().Bool("all-events", false, "Select every event type")

	return cmd
}

func runWebhooksTrigger(cmd *cobra.Command, args []string) error {
	webhookID := args[0]
	webhookUUID, err := validation.ParseUUIDField("webhook ID", webhookID)
	if err != nil {
		return err
	}

	events, err := cmdutil.ReadRepeatable(cmd, "events", "")
	if err != nil {
		return err
	}
	if err := cmdutil.RejectTogether(cmd, "events", "all-events"); err != nil {
		return err
	}
	allEvents, _ := cmd.Flags().GetBool("all-events")
	if events == nil && !allEvents {
		return errors.NewValidationError("at least one of --events or --all-events must be provided", nil)
	}

	// Set events to trigger
	// The trigger endpoint takes the event types of the payloads
	parsedEvents, err := webhooks.ParseEvents(events)
	if err != nil {
		return err
	}
	if allEvents {
		parsedEvents = webhooks.Events
	}
	eventsToTrigger := webhooks.EventTypes(parsedEvents)

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"webhook_id": webhookID,
		"events":     eventsToTrigger,
		"all_events": allEvents,
	}).Debug("Executing webhooks trigger command")

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Trigger the webhook
	response, err := client.TriggerWebhook(webhookUUID, eventsToTrigger)
	if err != nil {
		return err
	}

	// Show success message
	eventsList := strings.Join(eventsToTrigger, ", ")
	successMsg := fmt.Sprintf("Webhook events triggered successfully: %s", eventsList)

	return printer.GetResponseHandlerFromCommand(cmd).HandleSuccessResponse(response, printer.ResultConfig{
		SuccessMessage: successMsg,
	})
}
