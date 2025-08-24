package routes

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the get command
func NewGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <route-id>",
		Short: "Get detailed information about a specific route",
		Long: `Get detailed information about a specific route including its
configuration, webhook URL, recipient filtering, processing options,
status, and metadata.

This command shows comprehensive route details including:
- Basic information (name, URL, status)
- Recipient filtering pattern
- Processing options (attachments, headers, grouping, reply stripping)
- Timestamps (created, last updated)
- Complete route configuration

The route ID can be found using the 'ahasend routes list' command.`,
		Example: `  # Get route details
  ahasend routes get abcd1234-5678-90ef-abcd-1234567890ab

  # Get route details in JSON format
  ahasend routes get abcd1234-5678-90ef-abcd-1234567890ab --output json

  # Get route configuration for backup/restore
  ahasend routes get abcd1234-5678-90ef-abcd-1234567890ab --output json > route-backup.json`,
		Args:         cobra.ExactArgs(1),
		RunE:         runRoutesGet,
		SilenceUsage: true,
	}

	return cmd
}

func runRoutesGet(cmd *cobra.Command, args []string) error {
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
	}).Debug("Executing routes get command")

	// Get the route
	route, err := client.GetRoute(routeID)
	if err != nil {
		return err
	}

	// Use the new ResponseHandler to display route details
	return handler.HandleSingleRoute(route, printer.SingleConfig{
		SuccessMessage: fmt.Sprintf("Route details for %s", routeID),
		EmptyMessage:   "Route not found",
		FieldOrder:     []string{"ID", "Name", "URL", "Enabled", "Recipient Filter", "Include Attachments", "Include Headers", "Group by Message ID", "Strip Replies", "Created at", "Updated at"},
	})
}
