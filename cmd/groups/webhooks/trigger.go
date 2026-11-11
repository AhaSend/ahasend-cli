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

--all-events triggers every event except the contact events: the 14 events
any account can use. --all-contact-events triggers the 8 contact events, and
the two can be used together.

` + contactEventsHelp + `

A disabled webhook gets no events: the API answers that they are queued, and
then drops them.`,
		Example: `  # Trigger a single event
  ahasend webhooks trigger abcd1234-5678-90ef-abcd-1234567890ab \
    --events message.delivered

  # Trigger multiple events
  ahasend webhooks trigger abcd1234-5678-90ef-abcd-1234567890ab \
    --events message.delivered --events message.opened --events message.clicked

  # Trigger every event except the contact events
  ahasend webhooks trigger abcd1234-5678-90ef-abcd-1234567890ab \
    --all-events

  # Trigger the contact events
  ahasend webhooks trigger abcd1234-5678-90ef-abcd-1234567890ab \
    --all-contact-events`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runWebhooksTrigger,
		SilenceUsage: true,
	}

	addEventFlags(cmd, "Event type to trigger, such as message.delivered or delivered (can be used multiple times)")

	return cmd
}

func runWebhooksTrigger(cmd *cobra.Command, args []string) error {
	webhookID := args[0]
	webhookUUID, err := validation.ParseUUIDField("webhook ID", webhookID)
	if err != nil {
		return err
	}

	events, eventsGiven, err := readEventFlags(cmd, "")
	if err != nil {
		return err
	}
	if !eventsGiven {
		return errors.NewValidationError("at least one of --events, --all-events or --all-contact-events must be provided", nil)
	}
	// The trigger endpoint takes the event types of the payloads
	eventsToTrigger := webhooks.EventTypes(events)

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"webhook_id": webhookID,
		"events":     eventsToTrigger,
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
