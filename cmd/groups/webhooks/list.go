package webhooks

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// NewListCommand creates the list command
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List webhooks",
		Long: `List the webhooks in your AhaSend account with their configuration,
event types, and status information.

The list shows webhook names, URLs, enabled status, and configured event types
to help you manage your webhook endpoints effectively.`,
		Example: `  # List all webhooks
  ahasend webhooks list

  # List webhooks with JSON output
  ahasend webhooks list --output json

  # List webhooks with pagination
  ahasend webhooks list --limit 10

  # List only enabled webhooks
  ahasend webhooks list --enabled`,
		Args:         cmdutil.NoArgs,
		RunE:         runWebhooksList,
		SilenceUsage: true,
	}

	cmdutil.AddPaginationFlags(cmd)
	cmdutil.AddLegacyCursorAlias(cmd)
	cmd.Flags().Bool("enabled", false, "Filter by enabled (--enabled) or disabled (--enabled=false) state")

	return cmd
}

func runWebhooksList(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	pagination, err := cmdutil.ReadPaginationParams(cmd)
	if err != nil {
		return err
	}
	// The API filters by enabled status, so every output format, JSON
	// included, shows the page the API returned.
	enabled := cmdutil.ChangedBool(cmd, "enabled")

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"enabled": enabled,
	}).Debug("Executing webhooks list command")

	// Fetch webhooks
	response, err := client.ListWebhooks(pagination, enabled)
	if err != nil {
		return err
	}

	// Use the new ResponseHandler to display webhooks list
	emptyMessage := "No webhooks found"
	if enabled != nil && *enabled {
		emptyMessage = "No enabled webhooks found"
	} else if enabled != nil {
		emptyMessage = "No disabled webhooks found"
	}

	return handler.HandleWebhookList(response, printer.ListConfig{
		SuccessMessage: "Webhooks retrieved successfully",
		EmptyMessage:   emptyMessage,
	})
}
