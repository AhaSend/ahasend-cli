package routes

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewTriggerCommand creates the trigger command
func NewTriggerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trigger <route-id>",
		Short: "Trigger route events for testing",
		Long: `Trigger route events for development and testing purposes.

The command triggers a message.routing event to test your route endpoints
without waiting for actual inbound emails. This is particularly useful during
development and integration testing.

The route ID can be found using the 'ahasend routes list' command.

A disabled route gets no event: the API answers that it is queued, and then
drops it.`,
		Example: `  # Trigger a route event
  ahasend routes trigger abcd1234-5678-90ef-abcd-1234567890ab`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runRoutesTrigger,
		SilenceUsage: true,
	}

	return cmd
}

func runRoutesTrigger(cmd *cobra.Command, args []string) error {
	routeID := args[0]
	routeUUID, err := validation.ParseUUIDField("route ID", routeID)
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
		"route_id": routeID,
	}).Debug("Executing routes trigger command")

	// Trigger the route
	response, err := client.TriggerRoute(routeUUID)
	if err != nil {
		return err
	}

	// Show success message
	successMsg := fmt.Sprintf("Route '%s' triggered successfully", routeID)

	return handler.HandleSuccessResponse(response, printer.ResultConfig{
		SuccessMessage: successMsg,
	})
}
