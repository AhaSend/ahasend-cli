package routes

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
)

// NewListCommand creates the list command
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all inbound email routes",
		Long: `List all inbound email routes configured for your account.
Routes control how incoming emails are processed and forwarded to your
application endpoints.

This command displays:
- Route name and ID
- Webhook URL for processing
- Recipient filtering patterns
- Route status (enabled/disabled)
- Processing options (attachments, headers, etc.)
- Creation and last update times

Use --limit to control pagination and --cursor for continued navigation.`,
		Example: `  # List all routes
  ahasend routes list

  # List with pagination
  ahasend routes list --limit 10

  # Continue from cursor
  ahasend routes list --cursor "abc123"

  # Filter by enabled status
  ahasend routes list --enabled

  # JSON output for automation
  ahasend routes list --output json`,
		RunE:         runRoutesList,
		SilenceUsage: true,
	}

	// Add flags
	cmd.Flags().Int32("limit", 50, "Maximum number of routes to return")
	cmd.Flags().String("cursor", "", "Pagination cursor for continued results")
	cmd.Flags().Bool("enabled", false, "Show only enabled routes")

	return cmd
}

func runRoutesList(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Parse flags
	limit, _ := cmd.Flags().GetInt32("limit")
	cursor, _ := cmd.Flags().GetString("cursor")
	enabledFilter, _ := cmd.Flags().GetBool("enabled")

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"limit":          limit,
		"cursor":         cursor,
		"filter_enabled": enabledFilter,
	}).Debug("Executing routes list command")

	// Fetch routes
	cursorPtr := &cursor
	if cursor == "" {
		cursorPtr = nil
	}

	routes, err := client.ListRoutes(&limit, cursorPtr)
	if err != nil {
		return handler.HandleError(err)
	}

	// Apply client-side enabled filter if requested
	// TODO: This should be implemented as a server-side filter for better performance
	if enabledFilter && routes != nil && routes.Data != nil {
		var filteredRoutes []responses.Route
		for _, route := range routes.Data {
			if route.Enabled {
				filteredRoutes = append(filteredRoutes, route)
			}
		}
		// Create a new response with filtered data
		filteredResponse := &responses.PaginatedRoutesResponse{
			Data:       filteredRoutes,
			Pagination: routes.Pagination, // Keep original pagination info
		}
		routes = filteredResponse
	}

	// Determine appropriate message based on filter
	emptyMessage := "No routes found"
	if enabledFilter {
		emptyMessage = "No enabled routes found"
	}

	// Use the new ResponseHandler to display route list
	return handler.HandleRouteList(routes, printer.ListConfig{
		SuccessMessage: "Routes retrieved successfully",
		EmptyMessage:   emptyMessage,
		ShowPagination: true,
		FieldOrder:     []string{"id", "name", "url", "enabled", "recipient", "attachments", "headers", "group_by_message_id", "strip_replies", "created_at", "updated_at"},
	})
}
