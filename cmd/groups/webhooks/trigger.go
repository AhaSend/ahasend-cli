package webhooks

import (
	"fmt"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
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

Note: This is a development-only feature and may not be available in
production environments.`,
		Example: `  # Trigger a single event
  ahasend webhooks trigger abcd1234-5678-90ef-abcd-1234567890ab \
    --events "on_delivered"

  # Trigger multiple events
  ahasend webhooks trigger abcd1234-5678-90ef-abcd-1234567890ab \
    --events "on_delivered,on_opened,on_clicked"

  # Trigger all available events
  ahasend webhooks trigger abcd1234-5678-90ef-abcd-1234567890ab \
    --all-events`,
		Args:         cobra.ExactArgs(1),
		RunE:         runWebhooksTrigger,
		SilenceUsage: true,
	}

	// Event selection flags
	cmd.Flags().StringSlice("events", []string{}, "Event types to trigger")
	cmd.Flags().Bool("all-events", false, "Trigger all available event types")

	return cmd
}

func runWebhooksTrigger(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	webhookID := args[0]

	// Get flags
	events, _ := cmd.Flags().GetStringSlice("events")
	allEvents, _ := cmd.Flags().GetBool("all-events")

	// Validate conflicting flags
	if len(events) > 0 && allEvents {
		return fmt.Errorf("cannot specify both --events and --all-events")
	}

	// Check if any events are specified
	if len(events) == 0 && !allEvents {
		return fmt.Errorf("no events specified. Use --events or --all-events")
	}

	// Set events to trigger
	var eventsToTrigger []string
	if allEvents {
		eventsToTrigger = getAllValidTriggerEvents()
	} else {
		// Validate provided events
		validated, err := validateTriggerEvents(events)
		if err != nil {
			return err
		}
		eventsToTrigger = validated
	}

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"webhook_id": webhookID,
		"events":     eventsToTrigger,
		"all_events": allEvents,
	}).Debug("Executing webhooks trigger command")

	// Trigger the webhook
	err = client.TriggerWebhook(webhookID, eventsToTrigger)
	if err != nil {
		return err
	}

	// Show success message
	eventsList := strings.Join(eventsToTrigger, ", ")
	successMsg := fmt.Sprintf("Successfully triggered webhook events: %s", eventsList)

	return handler.HandleTriggerWebhook(webhookID, eventsToTrigger, printer.TriggerConfig{
		SuccessMessage: successMsg,
	})
}

func getAllValidTriggerEvents() []string {
	return []string{
		"on_reception",
		"on_delivered",
		"on_transient_error",
		"on_failed",
		"on_bounced",
		"on_suppressed",
		"on_suppression_created",
		"on_dns_error",
		"on_opened",
		"on_clicked",
	}
}

func validateTriggerEvents(events []string) ([]string, error) {
	validEvents := map[string]bool{
		"on_reception":           true,
		"on_delivered":           true,
		"on_transient_error":     true,
		"on_failed":              true,
		"on_bounced":             true,
		"on_suppressed":          true,
		"on_suppression_created": true,
		"on_dns_error":           true,
		"on_opened":              true,
		"on_clicked":             true,
	}

	var validatedEvents []string
	var invalidEvents []string

	for _, event := range events {
		event = strings.TrimSpace(event)
		if validEvents[event] {
			validatedEvents = append(validatedEvents, event)
		} else {
			invalidEvents = append(invalidEvents, event)
		}
	}

	if len(invalidEvents) > 0 {
		validKeys := getAllValidTriggerEvents()
		return nil, fmt.Errorf("invalid event types: %s\n\nValid event types are:\n%s",
			strings.Join(invalidEvents, ", "),
			strings.Join(validKeys, "\n"))
	}

	return validatedEvents, nil
}
