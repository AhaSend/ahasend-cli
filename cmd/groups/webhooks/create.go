package webhooks

import (
	"fmt"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-cli/internal/webhooks"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewCreateCommand creates the create command
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create --name <name> --url <url>",
		Short: "Create a webhook",
		Long: `Create a webhook endpoint to receive event notifications from AhaSend,
such as message deliveries, bounces, opens and clicks.

--name and --url are required. When one is missing and stdin is a terminal,
the command asks for it; --non-interactive never asks. On a terminal, it
asks for the events too when neither --events nor --all-events is given; a
webhook created without events receives none until "webhooks update" gives
it some. --scope is global unless set: a global webhook receives the events
of every domain; a scoped one only those of the domains given with
--domains, which it requires.

The webhook URL must be publicly accessible and support HTTPS for production
use. A webhook is enabled unless --enabled=false is set.`,
		Example: `  # Create a webhook, asking for the name, URL and events
  ahasend webhooks create

  # Create webhook with basic settings
  ahasend webhooks create --name "My Webhook" --url "https://example.com/webhook" --scope global

  # Create webhook with specific events
  ahasend webhooks create \
    --name "Delivery Webhook" \
    --url "https://api.example.com/webhooks/delivery" \
    --scope global \
    --events delivered --events bounced --events failed

  # Create a webhook for the events of one domain
  ahasend webhooks create \
    --name "Example Webhook" \
    --url "https://api.example.com/webhooks/example" \
    --scope scoped --domains example.com

  # Create disabled webhook for testing
  ahasend webhooks create \
    --name "Test Webhook" \
    --url "https://test.example.com/webhook" \
    --scope global \
    --enabled=false`,
		Args:         cmdutil.NoArgs,
		RunE:         runWebhooksCreate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "Webhook name (required)")
	cmd.Flags().String("url", "", "Webhook URL (required)")
	cmdutil.AddRepeatableFlag(cmd, "events", "Event type, such as delivered or message.delivered (can be used multiple times)")
	cmdutil.CompleteValues(cmd, "events", webhooks.EventValues())
	cmd.Flags().Bool("all-events", false, "Select every event type")
	cmd.Flags().Bool("enabled", true, "Enable the webhook (the API enables it when omitted)")
	cmdutil.AddNegatedFlagAlias(cmd, "disabled", "enabled")
	cmdutil.AddOneOfFlag(cmd, "scope", "global", "Webhook scope: %s", cmdutil.Scopes)
	cmdutil.AddRepeatableFlag(cmd, "domains", "Domain a scoped webhook is limited to (can be used multiple times)")
	cmdutil.AddIdempotencyKeyFlag(cmd)
	prompt.AddNonInteractiveFlag(cmd)
	cmdutil.AddInteractiveAlias(cmd)

	return cmd
}

func runWebhooksCreate(cmd *cobra.Command, args []string) error {
	events, err := cmdutil.ReadRepeatable(cmd, "events", "")
	if err != nil {
		return err
	}
	domains, err := cmdutil.ReadRepeatable(cmd, "domains", "")
	if err != nil {
		return err
	}
	if err := validation.ValidateDomainNames("--domains", domains); err != nil {
		return err
	}
	if err := cmdutil.RejectTogether(cmd, "events", "all-events"); err != nil {
		return err
	}
	parsedEvents, err := webhooks.ParseEvents(events)
	if err != nil {
		return err
	}
	allEvents, _ := cmd.Flags().GetBool("all-events")
	scope, _ := cmd.Flags().GetString("scope")
	scope, err = cmdutil.MatchOneOf(cmd, "scope", scope)
	if err != nil {
		return err
	}
	if err := cmdutil.CheckScopeDomains(scope, domains); err != nil {
		return err
	}

	name, err := cmdutil.ReadOrAsk(cmd, "name", "Webhook name")
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
	cmdutil.WarnIfInsecureURL(cmd, "url", webhookURL, cmdutil.WebhookSends)
	if events == nil && !allEvents && prompt.CanPrompt(cmd) {
		if parsedEvents, allEvents, err = askForEvents(cmd); err != nil {
			return err
		}
	}

	req := requests.CreateWebhookRequest{
		Name:    name,
		URL:     webhookURL,
		Enabled: cmdutil.ChangedBool(cmd, "enabled"),
		Scope:   scope,
	}
	setCreateEvents(&req, eventSettings(parsedEvents, allEvents))
	if domains != nil {
		req.Domains = &domains
	}

	logger.Get().WithFields(map[string]interface{}{
		"name":       name,
		"url":        webhookURL,
		"events":     events,
		"all_events": allEvents,
		"scope":      scope,
		"domains":    domains,
	}).Debug("Executing webhooks create command")

	idempotency, err := cmdutil.IdempotencyOptions(cmd)
	if err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	webhook, err := client.CreateWebhook(req, idempotency...)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleCreateWebhook(webhook, printer.ResultConfig{
		SuccessMessage: "Webhook created successfully",
	})
}

// askForEvents asks for the events a new webhook sends, as --events takes
// them, separated by commas, or all, and returns them, or whether all were
// asked for. A blank answer is a validation error, since a webhook without
// events receives nothing, and so is an answer that names something other
// than an event, or an event twice.
func askForEvents(cmd *cobra.Command) ([]webhooks.Event, bool, error) {
	answer, err := prompt.Prompt(cmd, fmt.Sprintf("Events to send (%s), separated by commas, or all: ",
		strings.Join(webhooks.EventNames(), ", ")))
	if err != nil {
		return nil, false, err
	}
	if strings.EqualFold(answer, "all") {
		return nil, true, nil
	}
	var values []string
	for _, value := range strings.Split(answer, ",") {
		if value = strings.TrimSpace(value); value != "" {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return nil, false, errors.NewValidationError("one of --events or --all-events is required: a webhook without events receives none", nil)
	}
	var events []webhooks.Event
	given := map[webhooks.Event]string{}
	for _, value := range values {
		event, ok := webhooks.FindEvent(value)
		if !ok {
			return nil, false, errors.NewValidationError(fmt.Sprintf(
				"%q is not an event; answer with events of the list, separated by commas, or all", value), nil)
		}
		if earlier, ok := given[event]; ok {
			return nil, false, errors.NewValidationError(fmt.Sprintf(
				"%q names the same event as %q; answer with each event once", value, earlier), nil)
		}
		given[event] = value
		events = append(events, event)
	}
	return events, false, nil
}

// eventSettings returns, for the Name of each webhook event, whether events
// holds it, or true for every event when all is set.
func eventSettings(events []webhooks.Event, all bool) map[string]bool {
	settings := make(map[string]bool, len(webhooks.Events))
	for _, event := range webhooks.Events {
		settings[event.Name] = all
	}
	for _, event := range events {
		settings[event.Name] = true
	}
	return settings
}

// setCreateEvents sets the event subscriptions of req from settings.
func setCreateEvents(req *requests.CreateWebhookRequest, settings map[string]bool) {
	req.OnReception = settings["reception"]
	req.OnDelivered = settings["delivered"]
	req.OnTransientError = settings["transient_error"]
	req.OnFailed = settings["failed"]
	req.OnBounced = settings["bounced"]
	req.OnSuppressed = settings["suppressed"]
	req.OnOpened = settings["opened"]
	req.OnClicked = settings["clicked"]
	req.OnSuppressionCreated = settings["suppression_created"]
	req.OnDnsError = settings["dns_error"]
}
