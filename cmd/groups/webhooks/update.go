package webhooks

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-cli/internal/webhooks"
	"github.com/AhaSend/ahasend-go"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewUpdateCommand creates the update command
func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <webhook-id>",
		Short: "Update a webhook",
		Long: `Partially update a webhook. Only the flags you set are sent; other
properties remain unchanged.

--events replaces the webhook's event types with the given ones,
--all-events selects every event type and --clear-events removes them all.
--domains replaces the domain restrictions of a scoped webhook; the API
ignores it on a global webhook unless --scope scoped is given too.
--scope global removes the domain restrictions.`,
		Example: `  # Update webhook name and URL
  ahasend webhooks update abcd1234-5678-90ef-abcd-1234567890ab \
    --name "Updated Webhook" \
    --url "https://new-endpoint.example.com/webhook"

  # Enable webhook and set its event types
  ahasend webhooks update abcd1234-5678-90ef-abcd-1234567890ab \
    --enabled --events delivered --events bounced --events opened

  # Disable webhook
  ahasend webhooks update abcd1234-5678-90ef-abcd-1234567890ab --enabled=false

  # Select all event types
  ahasend webhooks update abcd1234-5678-90ef-abcd-1234567890ab --all-events

  # Clear all event types
  ahasend webhooks update abcd1234-5678-90ef-abcd-1234567890ab --clear-events

  # Update scope and domains
  ahasend webhooks update abcd1234-5678-90ef-abcd-1234567890ab \
    --scope scoped --domains example.com --domains test.com`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runWebhooksUpdate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "New webhook name")
	cmd.Flags().String("url", "", "New webhook URL")
	cmd.Flags().Bool("enabled", false, "Enable (--enabled) or disable (--enabled=false) the webhook")
	cmdutil.AddFlagAlias(cmd, "enable", "enabled")
	cmdutil.AddNegatedFlagAlias(cmd, "disable", "enabled")
	cmdutil.AddRepeatableFlag(cmd, "events", "Event type, such as delivered or message.delivered, replacing all event types (can be used multiple times)")
	cmdutil.CompleteValues(cmd, "events", webhooks.EventValues())
	cmd.Flags().Bool("all-events", false, "Select every event type")
	cmd.Flags().Bool("clear-events", false, "Remove every event type")
	cmdutil.AddFlagAlias(cmd, "no-events", "clear-events")
	cmdutil.AddOneOfFlag(cmd, "scope", "", "New webhook scope: %s", cmdutil.Scopes)
	cmdutil.AddRepeatableFlag(cmd, "domains", "Domain restriction, replacing all domains (can be used multiple times)")
	cmdutil.AddValueAlias(cmd, "clear-domains", "scope", "global")

	return cmd
}

func runWebhooksUpdate(cmd *cobra.Command, args []string) error {
	webhookID := args[0]
	webhookUUID, err := validation.ParseUUIDField("webhook ID", webhookID)
	if err != nil {
		return err
	}
	req, err := readWebhookUpdate(cmd)
	if err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"webhook_id": webhookID,
	}).Debug("Executing webhooks update command")

	webhook, err := client.UpdateWebhook(webhookUUID, *req)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleUpdateWebhook(webhook, printer.ResultConfig{
		SuccessMessage: "Webhook updated successfully",
	})
}

// readWebhookUpdate reads and checks the flags of webhooks update into the
// request, which holds only the fields given. No field given is a
// validation error, and an --url that sends events over plain HTTP gets a
// warning.
func readWebhookUpdate(cmd *cobra.Command) (*requests.UpdateWebhookRequest, error) {
	name, err := cmdutil.ReadNonEmpty(cmd, "name", "")
	if err != nil {
		return nil, err
	}
	if err := cmdutil.CheckMaxLength("name", name, cmdutil.MaxNameLength); err != nil {
		return nil, err
	}
	webhookURL, err := cmdutil.ReadNonEmpty(cmd, "url", "")
	if err != nil {
		return nil, err
	}
	scope, err := cmdutil.ReadOneOf(cmd, "scope")
	if err != nil {
		return nil, err
	}
	events, err := cmdutil.ReadRepeatable(cmd, "events", "clear-events")
	if err != nil {
		return nil, err
	}
	if err := cmdutil.RejectTogether(cmd, "domains", "clear-domains"); err != nil {
		return nil, err
	}
	domainList, err := cmdutil.ReadRepeatable(cmd, "domains", "")
	if err != nil {
		return nil, errors.NewValidationError(err.Error()+"; use --scope global to remove the domain restriction", nil)
	}
	var domains *[]string
	if domainList != nil {
		if err := validation.ValidateDomainNames("--domains", domainList); err != nil {
			return nil, err
		}
		domains = &domainList
	}
	if domains != nil && scope != nil && *scope != "scoped" {
		return nil, errors.NewValidationError("--domains needs --scope scoped", nil)
	}
	for _, pair := range [][2]string{{"events", "all-events"}, {"events", "clear-events"}, {"all-events", "clear-events"}} {
		if err := cmdutil.RejectTogether(cmd, pair[0], pair[1]); err != nil {
			return nil, err
		}
	}
	parsedEvents, err := webhooks.ParseEvents(events)
	if err != nil {
		return nil, err
	}
	if webhookURL != nil {
		if err := cmdutil.ValidateWebhookURL("url", *webhookURL); err != nil {
			return nil, err
		}
	}
	allEvents, _ := cmd.Flags().GetBool("all-events")
	clearEvents, _ := cmd.Flags().GetBool("clear-events")

	req := requests.UpdateWebhookRequest{
		Name:    name,
		URL:     webhookURL,
		Enabled: cmdutil.ChangedBool(cmd, "enabled"),
		Scope:   scope,
		Domains: domains,
	}
	if allEvents || clearEvents || events != nil {
		setUpdateEvents(&req, eventSettings(parsedEvents, allEvents))
	}

	if req.Name == nil && req.URL == nil && req.Enabled == nil && req.Scope == nil && req.Domains == nil &&
		!allEvents && !clearEvents && events == nil {
		return nil, errors.NewValidationError("at least one of --name, --url, --enabled, --events, --all-events, --clear-events, --scope or --domains must be provided", nil)
	}
	if webhookURL != nil {
		cmdutil.WarnIfInsecureURL(cmd, "url", *webhookURL, cmdutil.WebhookSends)
	}
	return &req, nil
}

// setUpdateEvents sets every event subscription of req from settings.
func setUpdateEvents(req *requests.UpdateWebhookRequest, settings map[string]bool) {
	req.OnReception = ahasend.Bool(settings["reception"])
	req.OnDelivered = ahasend.Bool(settings["delivered"])
	req.OnTransientError = ahasend.Bool(settings["transient_error"])
	req.OnFailed = ahasend.Bool(settings["failed"])
	req.OnBounced = ahasend.Bool(settings["bounced"])
	req.OnSuppressed = ahasend.Bool(settings["suppressed"])
	req.OnOpened = ahasend.Bool(settings["opened"])
	req.OnClicked = ahasend.Bool(settings["clicked"])
	req.OnSuppressionCreated = ahasend.Bool(settings["suppression_created"])
	req.OnDnsError = ahasend.Bool(settings["dns_error"])
}
