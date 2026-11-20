package webhooks

import (
	"fmt"
	"slices"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/listen"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-cli/internal/webhooks"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// NewListenCommand creates the listen command
func NewListenCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "listen",
		Short: "Listen for webhook events in real-time",
		Long: `Listen for webhook events in real-time over a WebSocket connection.

The command receives webhook events over the connection and can:
- Forward events to a local endpoint for development
- Filter events by type
- Display full or slim output

The command generates a webhook secret for signing forwarded events using
the standard-webhooks specification, and forwards events one at a time in
the order they arrive, with their data exactly as received.

The API key needs the webhooks:write:all scope. Without --webhook-id, the API
creates a temporary webhook, "DevMode webhook for <API key label>", which
webhooks list does not show, and the command deletes it when it ends,
Ctrl+C included. Once deleted, it cannot be listened on again by its ID, and
the events that waited for it are gone. To listen again after the command
ends and get the events that waited for a listener, listen on a webhook of
your own with --webhook-id.

Deleting the temporary webhook needs the webhooks:delete:all scope too.
Without it the webhook stays, and a warning on stderr names it and the
command that deletes it, 'ahasend webhooks delete <webhook-id>', to run with
a key that has the scope. The delete does not change the exit code. The
webhook also stays when the command cannot delete it: when the command is
killed with SIGKILL, when its terminal is closed on Linux or macOS, on
Windows when the console window is closed and the delete is not done in
time, and when the command is interrupted, or the API's answer is lost,
during the request that starts the stream. The API deletes such a webhook by
itself only once 100 events have waited for it with no listener connected.

The command runs until Ctrl+C or until the connection closes. It exits 0
when the server closes the connection normally, and 7 when the connection
drops or the server closes it with an error; it does not reconnect. A
message it cannot read, such as one that is not JSON, is skipped with a
warning on stderr.

The stream sends only the events the API key's scopes can read. When the
webhook turns on events whose read scope the key does not have, such as
suppressions:read, the command prints the server's warning on stderr as it
connects.`,
		Example: `  # Listen for all webhook events
  ahasend webhooks listen

  # Use existing webhook
  ahasend webhooks listen --webhook-id abcd1234-5678-90ef-abcd-1234567890ab

  # Forward events to local endpoint
  ahasend webhooks listen --forward-to http://localhost:3000/webhook

  # Filter specific event types
  ahasend webhooks listen --events message.opened --events message.clicked

  # Slim output (only event types)
  ahasend webhooks listen --slim-output`,
		Args:         cmdutil.NoArgs,
		RunE:         runWebhooksListen,
		SilenceUsage: true,
	}

	cmd.Flags().String("webhook-id", "", "Use existing webhook instead of creating temporary one")
	cmdutil.AddRepeatableFlag(cmd, "events", "Show only this event type, such as message.opened or opened (can be used multiple times)")
	cmdutil.CompleteValues(cmd, "events", webhooks.EventValues())
	listen.AddListenFlags(cmd)

	return cmd
}

func runWebhooksListen(cmd *cobra.Command, args []string) error {
	webhookID, _ := cmd.Flags().GetString("webhook-id")
	var webhookUUID *uuid.UUID
	if webhookID != "" {
		id, err := validation.ParseUUIDField("webhook ID", webhookID)
		if err != nil {
			return err
		}
		webhookUUID = &id
	}
	events, err := cmdutil.ReadRepeatable(cmd, "events", "")
	if err != nil {
		return err
	}
	skipVerify, _ := cmd.Flags().GetBool("skip-verify")
	slimOutput, _ := cmd.Flags().GetBool("slim-output")

	// Listening filters by the event types of the payloads
	filterEvents, err := webhooks.ParseEvents(events)
	if err != nil {
		return err
	}
	filters := webhooks.EventTypes(filterEvents)
	forwardTo, err := listen.ReadForwardTo(cmd, cmdutil.WebhookSends)
	if err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"webhook_id":  webhookID,
		"events":      events,
		"forward_to":  forwardTo,
		"slim_output": slimOutput,
	}).Debug("Executing webhooks listen command")

	// Initiate webhook stream
	streamResponse, err := apiClient.InitiateWebhookStream(webhookUUID)
	if err != nil {
		return err
	}

	listener := listen.Listener{
		Kind:       "webhook",
		Title:      "Webhook",
		ID:         streamResponse.WebhookID,
		WsURL:      streamResponse.WsURL,
		Waiting:    "events",
		ForwardTo:  forwardTo,
		SkipVerify: skipVerify,
		SlimOutput: slimOutput,
	}
	if webhookUUID == nil {
		listener.DeleteTemporary = func() error {
			id, err := uuid.Parse(streamResponse.WebhookID)
			if err != nil {
				return fmt.Errorf("the API returned no valid webhook ID")
			}
			_, err = apiClient.DeleteWebhook(id)
			return err
		}
	}
	if len(filters) > 0 {
		listener.Keep = func(eventType string) bool {
			return slices.Contains(filters, eventType)
		}
	}
	return listen.Listen(cmd, apiClient, listener)
}
