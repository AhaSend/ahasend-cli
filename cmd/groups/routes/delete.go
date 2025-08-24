package routes

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
		Use:   "delete <route-id>",
		Short: "Delete an inbound email route",
		Long: `Delete an inbound email route permanently from your account.

⚠️  WARNING: This action is irreversible!

Deleting a route will:
- Permanently remove the route configuration
- Stop processing emails that match the route
- Cannot be undone

Before deletion, you'll be shown the route details and asked for confirmation
unless you use the --force flag for automation.

Consider disabling the route instead of deleting it if you might need to
restore it later: ahasend routes update <route-id> --disabled`,
		Example: `  # Delete route with confirmation
  ahasend routes delete abcd1234-5678-90ef-abcd-1234567890ab

  # Delete route without confirmation (automation)
  ahasend routes delete abcd1234-5678-90ef-abcd-1234567890ab --force

  # Delete route with JSON output
  ahasend routes delete abcd1234-5678-90ef-abcd-1234567890ab --force --output json`,
		Args:         cobra.ExactArgs(1),
		RunE:         runRoutesDelete,
		SilenceUsage: true,
	}

	cmd.Flags().Bool("force", false, "Skip confirmation prompt (for automation)")

	return cmd
}

func runRoutesDelete(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	routeID := args[0]
	force, _ := cmd.Flags().GetBool("force")

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"route_id": routeID,
		"force":    force,
	}).Debug("Executing routes delete command")

	// Get route details for confirmation (unless force is used)
	var route *responses.Route
	if !force {
		route, err = client.GetRoute(routeID)
		if err != nil {
			return err
		}

		// Show route details and get confirmation
		// TODO: Interactive prompts should ideally be handled by the printer
		// to maintain format-aware behavior, but keeping here for now
		confirmed, err := confirmRouteDeletion(route)
		if err != nil {
			return err
		}

		if !confirmed {
			return handler.HandleSimpleSuccess("Route deletion cancelled")
		}
	}

	// Delete the route
	err = deleteRoute(client, routeID)
	if err != nil {
		return err
	}

	// Use the new ResponseHandler to display deletion success
	return handler.HandleDeleteRoute(true, printer.DeleteConfig{
		SuccessMessage: fmt.Sprintf("Route %s deleted successfully", routeID),
		ItemName:       "route",
	})
}

func confirmRouteDeletion(route *responses.Route) (bool, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("⚠️  You are about to DELETE the following route:\n\n")
	fmt.Printf("Route Details:\n")
	fmt.Printf("  ID:              %s\n", route.ID)
	fmt.Printf("  Name:            %s\n", route.Name)
	fmt.Printf("  URL:             %s\n", route.URL)
	status := "Disabled"
	if route.Enabled {
		status = "Enabled"
	}
	fmt.Printf("  Status:          %s\n", status)

	if route.Recipient != "" {
		fmt.Printf("  Recipient Filter: %s\n", route.Recipient)
	} else {
		fmt.Printf("  Recipient Filter: All emails\n")
	}

	fmt.Printf("  Created:         %s\n", output.FormatTimeLocalValue(route.CreatedAt))

	fmt.Printf("\n🚨 WARNING: This action cannot be undone!\n")
	fmt.Printf("The route configuration will be permanently deleted and\n")
	fmt.Printf("emails matching this route will no longer be processed.\n\n")

	fmt.Printf("Consider disabling instead: ahasend routes update %s --disabled\n\n", route.ID)

	fmt.Print("Type 'delete' to confirm deletion: ")
	confirmation, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read confirmation: %w", err)
	}

	confirmation = strings.TrimSpace(confirmation)
	return strings.ToLower(confirmation) == "delete", nil
}

func deleteRoute(client client.AhaSendClient, routeID string) error {
	err := client.DeleteRoute(routeID)
	if err != nil {
		return err
	}
	return nil
}
