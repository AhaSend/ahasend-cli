package routes

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewUpdateCommand creates the update command
func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <route-id>",
		Short: "Update an inbound email route",
		Long: `Partially update an inbound email route. Only the flags you set are sent;
other properties remain unchanged. --recipient takes local@domain, where the
domain is one of your domains and the local part may be * or hold *
wildcards; a route's recipient cannot be cleared. Each on/off option is
turned off with =false, such as --enabled=false.`,
		Example: `  # Update route name
  ahasend routes update abcd1234-5678-90ef-abcd-1234567890ab --name "New Route Name"

  # Update webhook URL
  ahasend routes update abcd1234-5678-90ef-abcd-1234567890ab --url "https://api.example.com/new-webhook"

  # Enable a route
  ahasend routes update abcd1234-5678-90ef-abcd-1234567890ab --enabled

  # Disable a route
  ahasend routes update abcd1234-5678-90ef-abcd-1234567890ab --enabled=false

  # Update recipient filter
  ahasend routes update abcd1234-5678-90ef-abcd-1234567890ab --recipient "support-*@example.com"

  # Enable processing options
  ahasend routes update abcd1234-5678-90ef-abcd-1234567890ab \
    --include-attachments \
    --include-headers \
    --group-by-message-id

  # Disable specific processing options
  ahasend routes update abcd1234-5678-90ef-abcd-1234567890ab \
    --include-attachments=false \
    --strip-replies=false`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runRoutesUpdate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "New route name")
	cmd.Flags().String("url", "", "New webhook URL")
	cmd.Flags().String("recipient", "", "New recipient filter, local@domain, such as '*@example.com'")
	cmd.Flags().Bool("enabled", false, "Enable (--enabled) or disable (--enabled=false) the route")
	cmdutil.AddNegatedFlagAlias(cmd, "disabled", "enabled")
	for _, option := range routeOptions {
		cmd.Flags().Bool(option.flag, false, option.usage)
		cmdutil.AddNegatedFlagAlias(cmd, "no-"+option.flag, option.flag)
	}

	return cmd
}

// routeOptions lists the on/off processing options of a route.
var routeOptions = []struct {
	flag  string
	usage string
}{
	{"include-attachments", "Include attachments in the webhook payload (=false to stop)"},
	{"include-headers", "Include headers in the webhook payload (=false to stop)"},
	{"group-by-message-id", "Group related emails by message ID (=false to stop)"},
	{"strip-replies", "Strip reply content from emails (=false to stop)"},
}

func runRoutesUpdate(cmd *cobra.Command, args []string) error {
	routeID := args[0]
	routeUUID, err := validation.ParseUUIDField("route ID", routeID)
	if err != nil {
		return err
	}

	name, err := cmdutil.ReadNonEmpty(cmd, "name", "")
	if err != nil {
		return err
	}
	if err := cmdutil.CheckMaxLength("name", name, cmdutil.MaxNameLength); err != nil {
		return err
	}
	webhookURL, err := cmdutil.ReadNonEmpty(cmd, "url", "")
	if err != nil {
		return err
	}
	if webhookURL != nil {
		if err := cmdutil.ValidateWebhookURL("url", *webhookURL); err != nil {
			return err
		}
	}

	recipient := cmdutil.ChangedString(cmd, "recipient")
	if recipient != nil {
		if err := validateRecipient(*recipient); err != nil {
			return err
		}
	}

	req := requests.UpdateRouteRequest{
		Name:             name,
		URL:              webhookURL,
		Recipient:        recipient,
		Enabled:          cmdutil.ChangedBool(cmd, "enabled"),
		Attachments:      cmdutil.ChangedBool(cmd, "include-attachments"),
		Headers:          cmdutil.ChangedBool(cmd, "include-headers"),
		GroupByMessageId: cmdutil.ChangedBool(cmd, "group-by-message-id"),
		StripReplies:     cmdutil.ChangedBool(cmd, "strip-replies"),
	}
	if req == (requests.UpdateRouteRequest{}) {
		return errors.NewValidationError("at least one of --name, --url, --recipient, --enabled, --include-attachments, --include-headers, --group-by-message-id or --strip-replies must be provided", nil)
	}
	if webhookURL != nil {
		cmdutil.WarnIfInsecureURL(cmd, "url", *webhookURL, cmdutil.RouteSends)
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"route_id": routeID,
	}).Debug("Executing routes update command")

	route, err := client.UpdateRoute(routeUUID, req)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleUpdateRoute(route, printer.ResultConfig{
		SuccessMessage: "Route updated successfully",
	})
}
