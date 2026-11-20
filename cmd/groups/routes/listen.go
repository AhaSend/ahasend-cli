package routes

import (
	"fmt"
	"io"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/listen"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// NewListenCommand creates the listen command
func NewListenCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "listen",
		Short: "Listen for inbound email events in real-time",
		Long: `Listen for inbound email routing events in real-time over a WebSocket
connection. Set exactly one of --route-id or --recipient.

The command receives inbound email events over the connection and can:
- Forward events to a local endpoint for development
- Display events in full or slim output format
- Use existing routes or create temporary routes for a recipient,
  local@domain, such as *@example.com

The command generates a webhook secret for signing forwarded events using
the standard-webhooks specification, and forwards events one at a time in
the order they arrive, with their data exactly as received.

The API key needs the routes:write:all scope, and messages:read for the
domain of the route's recipient. With --recipient, the API creates a
temporary route, "DevMode route for <API key label>", which routes list does
not show, and the command deletes it when it ends, Ctrl+C included. Once
deleted, it cannot be listened on again by its ID, and the events that
waited for it are gone. To listen again after the command ends and get the
events that waited for a listener, listen on a route of your own with
--route-id.

Deleting the temporary route needs the routes:delete:all scope too, or
routes:delete:{domain} for the domain of the route's recipient. Without it
the route stays, and a warning on stderr names it and the command that
deletes it, 'ahasend routes delete <route-id>', to run with a key that has
the scope. The delete does not change the exit code. The route also stays
when the command cannot delete it: when it is killed, when its terminal is
closed on Linux or macOS, on Windows when the console window is closed and
the delete is not done in time, and when it is interrupted, or the API's
answer is lost, during the request that starts the stream. The API deletes
such a route by itself only once 100 events have waited for it with no
listener connected.

The command runs until Ctrl+C or until the connection closes. It exits 0
when the server closes the connection normally, and 7 when the connection
drops or the server closes it with an error; it does not reconnect. A
message it cannot read, such as one that is not JSON, is skipped with a
warning on stderr.`,
		Example: `  # Listen with existing route
  ahasend routes listen --route-id abcd1234-5678-90ef-abcd-1234567890ab

  # Listen with recipient pattern (backend creates temporary route)
  ahasend routes listen --recipient "*@example.com"

  # Forward events to local endpoint
  ahasend routes listen --recipient "support-*@example.com" \
    --forward-to http://localhost:3000/webhook

  # Slim output (minimal event display)
  ahasend routes listen --route-id abcd1234-5678-90ef-abcd-1234567890ab --slim-output`,
		Args:         cmdutil.NoArgs,
		RunE:         runRoutesListen,
		SilenceUsage: true,
	}

	cmd.Flags().String("route-id", "", "Use existing route instead of creating temporary one")
	cmd.Flags().String("recipient", "", "Recipient pattern for temporary route (e.g., *@domain.com)")
	listen.AddListenFlags(cmd)

	return cmd
}

func runRoutesListen(cmd *cobra.Command, args []string) error {
	routeID, _ := cmd.Flags().GetString("route-id")
	recipient, _ := cmd.Flags().GetString("recipient")
	skipVerify, _ := cmd.Flags().GetBool("skip-verify")
	slimOutput, _ := cmd.Flags().GetBool("slim-output")

	// Validate parameters - exactly one must be provided
	if err := cmdutil.RejectTogether(cmd, "route-id", "recipient"); err != nil {
		return err
	}
	routeUUID, err := parseListenParameters(routeID, recipient)
	if err != nil {
		return err
	}
	forwardTo, err := listen.ReadForwardTo(cmd, cmdutil.RouteSends)
	if err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"route_id":    routeID,
		"recipient":   recipient,
		"forward_to":  forwardTo,
		"slim_output": slimOutput,
	}).Debug("Executing routes listen command")

	// Initiate route stream
	streamResponse, err := apiClient.InitiateRouteStream(routeUUID, recipient)
	if err != nil {
		return err
	}

	listener := listen.Listener{
		Kind:        "route",
		Title:       "Route",
		ID:          streamResponse.RouteID,
		WsURL:       streamResponse.WsURL,
		Waiting:     "inbound emails",
		SlimDetails: printRoutingSummary,
		ForwardTo:   forwardTo,
		SkipVerify:  skipVerify,
		SlimOutput:  slimOutput,
	}
	if recipient != "" {
		listener.Details = recipientPatternDetails(recipient)
		listener.DeleteTemporary = func() error {
			id, err := uuid.Parse(streamResponse.RouteID)
			if err != nil {
				return fmt.Errorf("the API returned no valid route ID")
			}
			_, err = apiClient.DeleteRoute(id)
			return err
		}
	}
	return listen.Listen(cmd, apiClient, listener)
}

// recipientPatternDetails returns the banner line of a temporary route for
// recipient: its pattern.
func recipientPatternDetails(recipient string) func(io.Writer) {
	return func(out io.Writer) {
		fmt.Fprintf(out, "Recipient Pattern: %s\n", color.CyanString(recipient))
	}
}

// parseListenParameters checks --route-id and --recipient, of which the
// caller allows only one, and returns the route ID, or nil to listen on a
// temporary route for recipient.
func parseListenParameters(routeID, recipient string) (*uuid.UUID, error) {
	if routeID == "" && recipient == "" {
		return nil, errors.NewValidationError("one of --route-id or --recipient must be provided", nil)
	}
	if routeID != "" {
		id, err := validation.ParseUUIDField("route ID", routeID)
		if err != nil {
			return nil, err
		}
		return &id, nil
	}
	return nil, validateRecipient(recipient)
}

// printRoutingSummary prints the sender, recipient and subject of a routing
// event on its slim line. The event holds them in its "data" object.
func printRoutingSummary(out io.Writer, event map[string]interface{}) {
	data, _ := event["data"].(map[string]interface{})
	if from, ok := data["from"].(string); ok {
		fmt.Fprintf(out, " from: %s", color.GreenString(printer.EscapeControl(from)))
	}
	if to, ok := data["to"].(string); ok {
		fmt.Fprintf(out, " to: %s", color.CyanString(printer.EscapeControl(to)))
	}
	if subject, ok := data["subject"].(string); ok {
		fmt.Fprintf(out, " subject: %s", color.WhiteString(printer.EscapeControl(subject)))
	}
}
