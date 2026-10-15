package routes

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewCreateCommand creates the create command
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create --name <name> --url <url> --recipient <pattern>",
		Short: "Create an inbound email route",
		Long: `Create an inbound email route that posts incoming emails to a webhook.
Routes configure how incoming emails are filtered and forwarded to your
application endpoints.

--name, --url and --recipient are required. When one is missing and stdin
is a terminal, the command asks for it; --non-interactive never asks.
--recipient takes local@domain, where the domain is one of your domains and
the local part may be * or hold * wildcards.

A route is enabled unless --enabled=false is set, and the processing options
are off unless their flags are set.`,
		Example: `  # Create a route, asking for the name, URL and recipient
  ahasend routes create

  # Create a route with the required flags
  ahasend routes create --name "Support Route" --url "https://api.example.com/webhook" \
    --recipient "support@example.com"

  # Route for every address of a domain
  ahasend routes create \
    --name "Help Desk" \
    --url "https://api.example.com/support" \
    --recipient "*@example.com" \
    --include-attachments

  # Route with advanced processing options, created disabled
  ahasend routes create \
    --name "Sales Inquiries" \
    --url "https://api.example.com/sales" \
    --recipient "sales-*@example.com" \
    --include-headers \
    --group-by-message-id \
    --strip-replies \
    --enabled=false`,
		Args:         cmdutil.NoArgs,
		RunE:         runRoutesCreate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "Route name (required)")
	cmd.Flags().String("url", "", "Webhook URL for processing emails (required)")
	cmd.Flags().String("recipient", "", "Recipient filter, local@domain, such as 'support@example.com' or '*@example.com' (required)")
	cmd.Flags().Bool("include-attachments", false, "Include email attachments in webhook payload")
	cmd.Flags().Bool("include-headers", false, "Include email headers in webhook payload")
	cmd.Flags().Bool("group-by-message-id", false, "Group related emails by message ID (conversation threading)")
	cmd.Flags().Bool("strip-replies", false, "Strip reply content from emails")
	cmd.Flags().Bool("enabled", true, "Enable the route (the API enables it when omitted)")
	cmdutil.AddIdempotencyKeyFlag(cmd)
	prompt.AddNonInteractiveFlag(cmd)
	cmdutil.AddInteractiveAlias(cmd)

	return cmd
}

func runRoutesCreate(cmd *cobra.Command, args []string) error {
	name, err := cmdutil.ReadOrAsk(cmd, "name", "Route name")
	if err != nil {
		return err
	}
	if err := cmdutil.CheckMaxLength("name", &name, cmdutil.MaxNameLength); err != nil {
		return err
	}
	webhookURL, err := cmdutil.ReadOrAsk(cmd, "url", "Webhook URL")
	if err != nil {
		return err
	}
	if err := cmdutil.ValidateWebhookURL("url", webhookURL); err != nil {
		return err
	}
	recipient, err := cmdutil.ReadOrAsk(cmd, "recipient", "Recipient filter (e.g., support@example.com)")
	if err != nil {
		return err
	}
	if err := validateRecipient(recipient); err != nil {
		return err
	}
	cmdutil.WarnIfInsecureURL(cmd, "url", webhookURL, cmdutil.RouteSends)

	includeAttachments, _ := cmd.Flags().GetBool("include-attachments")
	includeHeaders, _ := cmd.Flags().GetBool("include-headers")
	groupByMessageID, _ := cmd.Flags().GetBool("group-by-message-id")
	stripReplies, _ := cmd.Flags().GetBool("strip-replies")
	req := requests.CreateRouteRequest{
		Name:             name,
		URL:              webhookURL,
		Recipient:        recipient,
		Attachments:      includeAttachments,
		Headers:          includeHeaders,
		GroupByMessageId: groupByMessageID,
		StripReplies:     stripReplies,
		Enabled:          cmdutil.ChangedBool(cmd, "enabled"),
	}

	logger.Get().WithFields(map[string]interface{}{
		"name":      name,
		"url":       webhookURL,
		"recipient": recipient,
	}).Debug("Executing routes create command")

	idempotency, err := cmdutil.IdempotencyOptions(cmd)
	if err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	route, err := client.CreateRoute(req, idempotency...)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleCreateRoute(route, printer.ResultConfig{
		SuccessMessage: "Route created successfully",
	})
}
