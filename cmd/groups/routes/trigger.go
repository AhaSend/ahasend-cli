package routes

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// NewTriggerCommand creates the trigger command
func NewTriggerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trigger <route-id>",
		Short: "Trigger route events for testing",
		Long: `Trigger route events for development and testing purposes.

This command allows you to manually trigger a message.routing event to test your
route endpoints without waiting for actual inbound emails. This is particularly
useful during development and integration testing.

The route ID can be found using the 'ahasend routes list' command.

Note: This is a development-only feature and may not be available in
production environments.`,
		Example: `  # Trigger a route event
  ahasend routes trigger abcd1234-5678-90ef-abcd-1234567890ab`,
		Args:         cobra.ExactArgs(1),
		RunE:         runRoutesTrigger,
		SilenceUsage: true,
	}

	return cmd
}

func runRoutesTrigger(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	routeID := args[0]

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"route_id": routeID,
	}).Debug("Executing routes trigger command")

	// Trigger the route
	err = client.TriggerRoute(routeID)
	if err != nil {
		return err
	}

	// Show success message
	successMsg := fmt.Sprintf("Successfully triggered route event for route ID: %s", routeID)

	return handler.HandleTriggerRoute(routeID, printer.TriggerConfig{
		SuccessMessage: successMsg,
	})
}
