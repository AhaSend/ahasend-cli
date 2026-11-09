package webhooks

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/webhooks"
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
to help you manage your webhook endpoints effectively.

--events lists only the webhooks that have every event it names on; the API
filters, as it does for --enabled.`,
		Example: `  # List all webhooks
  ahasend webhooks list

  # List webhooks with JSON output
  ahasend webhooks list --output json

  # List webhooks with pagination
  ahasend webhooks list --limit 10

  # List only enabled webhooks
  ahasend webhooks list --enabled

  # List only the webhooks that get both contact.created and delivered
  ahasend webhooks list --events contact.created --events delivered`,
		Args:         cmdutil.NoArgs,
		RunE:         runWebhooksList,
		SilenceUsage: true,
	}

	cmdutil.AddPaginationFlags(cmd)
	cmdutil.AddLegacyCursorAlias(cmd)
	cmd.Flags().Bool("enabled", false, "Filter by enabled (--enabled) or disabled (--enabled=false) state")
	cmdutil.AddRepeatableFlag(cmd, "events", "List only the webhooks with this event on, such as delivered or message.delivered (can be used multiple times)")
	cmdutil.CompleteValues(cmd, "events", webhooks.EventValues())

	return cmd
}

func runWebhooksList(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	pagination, err := cmdutil.ReadPaginationParams(cmd)
	if err != nil {
		return err
	}
	// The API filters by enabled status and by events, so every output
	// format, JSON included, shows the page the API returned.
	enabled := cmdutil.ChangedBool(cmd, "enabled")
	values, err := cmdutil.ReadRepeatable(cmd, "events", "")
	if err != nil {
		return err
	}
	events, err := webhooks.ParseEvents(values)
	if err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"enabled": enabled,
		"events":  webhooks.EventTypes(events),
	}).Debug("Executing webhooks list command")

	// Fetch webhooks
	response, err := client.ListWebhooks(pagination, enabled, webhooks.EventNames(events))
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
