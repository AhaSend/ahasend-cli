package routes

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
)

// NewListCommand creates the list command
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List inbound email routes",
		Long: `List the inbound email routes configured for your account.
Routes control how incoming emails are processed and forwarded to your
application endpoints.

This command displays:
- Route name and ID
- Webhook URL for processing
- Recipient filtering patterns
- Route status (enabled/disabled)
- Processing options (attachments, headers, etc.)
- Creation and last update times

Use --limit to set the page size, and --after or --before with the cursor a
page ends with to fetch the next or the previous page.

--enabled keeps only the enabled routes of the page the API returned, and
--enabled=false only the disabled ones. It cannot be used with --output json,
which prints the API's response unchanged.`,
		Example: `  # List all routes
  ahasend routes list

  # List with pagination
  ahasend routes list --limit 10

  # Fetch the next page
  ahasend routes list --after "abc123"

  # List the routes of one recipient domain
  ahasend routes list --domain example.com

  # Show only enabled routes (table, plain and CSV output)
  ahasend routes list --enabled

  # Show only disabled routes
  ahasend routes list --enabled=false

  # JSON output for automation
  ahasend routes list --output json`,
		Args:         cmdutil.NoArgs,
		RunE:         runRoutesList,
		SilenceUsage: true,
	}

	// Add flags
	cmdutil.AddPaginationFlags(cmd)
	cmdutil.AddLegacyCursorAlias(cmd)
	cmd.Flags().Bool("enabled", false, "Show only enabled (--enabled) or disabled (--enabled=false) routes of the page (not with --output json)")
	cmd.Flags().String("domain", "", "Filter by recipient domain (required with a domain-scoped API key)")

	return cmd
}

func runRoutesList(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	pagination, err := cmdutil.ReadPaginationParams(cmd)
	if err != nil {
		return err
	}
	domain, err := cmdutil.ReadNonEmpty(cmd, "domain", "")
	if err != nil {
		return err
	}
	if err := validation.ValidateOptionalDomainName("--domain", domain); err != nil {
		return err
	}
	enabledFilter := cmdutil.ChangedBool(cmd, "enabled")
	// The API cannot filter routes by status, so --enabled filters the page
	// here. JSON output prints only the API's response, so it refuses the
	// flag instead of ignoring it.
	if enabledFilter != nil && handler.GetFormat() == "json" {
		return errors.NewValidationError("--enabled and --output json cannot be used together; filter the routes' \"enabled\" field instead", nil)
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"filter_enabled": enabledFilter,
	}).Debug("Executing routes list command")

	routes, err := client.ListRoutes(requests.GetRoutesParams{Domain: domain, PaginationParams: pagination})
	if err != nil {
		return err
	}

	if enabledFilter != nil && routes != nil && routes.Data != nil {
		var filteredRoutes []responses.Route
		for _, route := range routes.Data {
			if route.Enabled == *enabledFilter {
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
	if enabledFilter != nil && *enabledFilter {
		emptyMessage = "No enabled routes found"
	} else if enabledFilter != nil {
		emptyMessage = "No disabled routes found"
	}

	// Use the new ResponseHandler to display route list
	return handler.HandleRouteList(routes, printer.ListConfig{
		SuccessMessage: "Routes retrieved successfully",
		EmptyMessage:   emptyMessage,
	})
}
