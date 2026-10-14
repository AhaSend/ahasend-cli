package routes

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
		Use:   "delete <route-id>",
		Short: "Delete an inbound email route",
		Long: `Delete an inbound email route permanently from your account.

WARNING: This action is irreversible!

Deleting a route will:
- Permanently remove the route configuration
- Stop processing emails that match the route
- Cannot be undone

Delete asks for confirmation unless --force is set.

Consider disabling the route instead of deleting it if you might need to
restore it later: ahasend routes update <route-id> --enabled=false`,
		Example: `  # Delete route with confirmation
  ahasend routes delete abcd1234-5678-90ef-abcd-1234567890ab

  # Delete route without confirmation (automation)
  ahasend routes delete abcd1234-5678-90ef-abcd-1234567890ab --force

  # Delete route with JSON output
  ahasend routes delete abcd1234-5678-90ef-abcd-1234567890ab --force --output json`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runRoutesDelete,
		SilenceUsage: true,
	}

	prompt.AddForceFlag(cmd)

	return cmd
}

func runRoutesDelete(cmd *cobra.Command, args []string) error {
	routeID := args[0]
	routeUUID, err := validation.ParseUUIDField("route ID", routeID)
	if err != nil {
		return err
	}
	if err := prompt.Confirm(cmd, fmt.Sprintf("Are you sure you want to delete route %s?", routeID)); err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"route_id": routeID,
	}).Debug("Executing routes delete command")

	response, err := client.DeleteRoute(routeUUID)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleSuccessResponse(response, printer.ResultConfig{
		SuccessMessage: fmt.Sprintf("Route '%s' deleted successfully", routeID),
	})
}
