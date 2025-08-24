package webhooks

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
)

// NewUpdateCommand creates the update command
func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <webhook-id>",
		Short: "Update an existing webhook",
		Long: `Update an existing webhook's configuration including name, URL,
enabled status, event types, scope, and domain restrictions.

You can update any combination of webhook properties by providing the
corresponding flags. Only the specified properties will be updated;
others will remain unchanged.

The webhook ID can be found using the 'ahasend webhooks list' command.`,
		Example: `  # Update webhook name and URL
  ahasend webhooks update abcd1234-5678-90ef-abcd-1234567890ab \
    --name "Updated Webhook" \
    --url "https://new-endpoint.example.com/webhook"

  # Enable webhook and add event types
  ahasend webhooks update abcd1234-5678-90ef-abcd-1234567890ab \
    --enable \
    --events "delivered,bounced,opened"

  # Disable webhook
  ahasend webhooks update abcd1234-5678-90ef-abcd-1234567890ab --disable

  # Update event types (replaces existing events)
  ahasend webhooks update abcd1234-5678-90ef-abcd-1234567890ab \
    --events "delivered,failed"

  # Add all event types
  ahasend webhooks update abcd1234-5678-90ef-abcd-1234567890ab --all-events

  # Clear all event types
  ahasend webhooks update abcd1234-5678-90ef-abcd-1234567890ab --no-events

  # Update scope and domains
  ahasend webhooks update abcd1234-5678-90ef-abcd-1234567890ab \
    --scope "domain" \
    --domains "example.com,test.com"`,
		Args:         cobra.ExactArgs(1),
		RunE:         runWebhooksUpdate,
		SilenceUsage: true,
	}

	// Basic properties
	cmd.Flags().String("name", "", "Update webhook name")
	cmd.Flags().String("url", "", "Update webhook URL")

	// Status flags
	cmd.Flags().Bool("enable", false, "Enable the webhook")
	cmd.Flags().Bool("disable", false, "Disable the webhook")

	// Event type flags
	cmd.Flags().StringSlice("events", []string{}, "Set event types (replaces existing)")
	cmd.Flags().Bool("all-events", false, "Enable all available event types")
	cmd.Flags().Bool("no-events", false, "Disable all event types")

	// Optional configuration
	cmd.Flags().String("scope", "", "Update webhook scope")
	cmd.Flags().StringSlice("domains", []string{}, "Set domain restrictions (replaces existing)")
	cmd.Flags().Bool("clear-domains", false, "Clear all domain restrictions")

	return cmd
}

func runWebhooksUpdate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	webhookID := args[0]

	// Get flags
	name, _ := cmd.Flags().GetString("name")
	webhookURL, _ := cmd.Flags().GetString("url")
	enable, _ := cmd.Flags().GetBool("enable")
	disable, _ := cmd.Flags().GetBool("disable")
	events, _ := cmd.Flags().GetStringSlice("events")
	allEvents, _ := cmd.Flags().GetBool("all-events")
	noEvents, _ := cmd.Flags().GetBool("no-events")
	scope, _ := cmd.Flags().GetString("scope")
	domains, _ := cmd.Flags().GetStringSlice("domains")
	clearDomains, _ := cmd.Flags().GetBool("clear-domains")

	// Validate conflicting flags
	if enable && disable {
		return fmt.Errorf("cannot specify both --enable and --disable")
	}

	eventFlagsCount := 0
	if len(events) > 0 {
		eventFlagsCount++
	}
	if allEvents {
		eventFlagsCount++
	}
	if noEvents {
		eventFlagsCount++
	}
	if eventFlagsCount > 1 {
		return fmt.Errorf("cannot specify multiple event flags: choose one of --events, --all-events, or --no-events")
	}

	if len(domains) > 0 && clearDomains {
		return fmt.Errorf("cannot specify both --domains and --clear-domains")
	}

	// Check if any update flags are provided
	hasUpdates := name != "" || webhookURL != "" || enable || disable ||
		len(events) > 0 || allEvents || noEvents ||
		scope != "" || len(domains) > 0 || clearDomains

	if !hasUpdates {
		return fmt.Errorf("no update flags provided. Use --help to see available options")
	}

	// Validate URL format if provided
	if webhookURL != "" {
		if err := validateWebhookURL(webhookURL); err != nil {
			return err
		}
	}

	// Validate event types if provided
	if len(events) > 0 {
		_, err := validateEventTypes(events)
		if err != nil {
			return err
		}
	}

	// Create update request
	req := &requests.UpdateWebhookRequest{}

	// Set basic properties
	if name != "" {
		req.Name = ahasend.String(name)
	}
	if webhookURL != "" {
		req.URL = ahasend.String(webhookURL)
	}

	// Set enabled status
	if enable {
		req.Enabled = ahasend.Bool(true)
	} else if disable {
		req.Enabled = ahasend.Bool(false)
	}

	// Set event types
	if allEvents {
		setAllEventTypesUpdate(req)
	} else if noEvents {
		clearAllEventTypes(req)
	} else if len(events) > 0 {
		clearAllEventTypes(req) // Clear existing first
		setEventTypesUpdate(req, events)
	}

	// Set scope
	if scope != "" {
		req.Scope = ahasend.String(scope)
	}

	// Set domains
	if clearDomains {
		emptyDomains := []string{}
		req.Domains = &emptyDomains // Empty slice to clear
	} else if len(domains) > 0 {
		req.Domains = &domains
	}

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"webhook_id":    webhookID,
		"name":          name,
		"url":           webhookURL,
		"enable":        enable,
		"disable":       disable,
		"events":        events,
		"all_events":    allEvents,
		"no_events":     noEvents,
		"scope":         scope,
		"domains":       domains,
		"clear_domains": clearDomains,
	}).Debug("Executing webhooks update command")

	// Update the webhook
	webhook, err := updateWebhook(client, webhookID, *req)
	if err != nil {
		return err
	}

	// Use the new ResponseHandler to display updated webhook
	return handler.HandleUpdateWebhook(webhook, printer.UpdateConfig{
		SuccessMessage: fmt.Sprintf("Successfully updated webhook: %s", webhook.Name),
		ItemName:       "webhook",
		FieldOrder:     []string{"id", "name", "url", "enabled", "event_types", "scope", "domains", "created_at", "updated_at"},
	})
}

func updateWebhook(client client.AhaSendClient, webhookID string, req requests.UpdateWebhookRequest) (*responses.Webhook, error) {
	webhook, err := client.UpdateWebhook(webhookID, req)
	if err != nil {
		return nil, err
	}
	return webhook, nil
}

func setAllEventTypesUpdate(req *requests.UpdateWebhookRequest) {
	req.OnReception = ahasend.Bool(true)
	req.OnDelivered = ahasend.Bool(true)
	req.OnTransientError = ahasend.Bool(true)
	req.OnFailed = ahasend.Bool(true)
	req.OnBounced = ahasend.Bool(true)
	req.OnSuppressed = ahasend.Bool(true)
	req.OnOpened = ahasend.Bool(true)
	req.OnClicked = ahasend.Bool(true)
	req.OnSuppressionCreated = ahasend.Bool(true)
	req.OnDnsError = ahasend.Bool(true)
}

func clearAllEventTypes(req *requests.UpdateWebhookRequest) {
	req.OnReception = ahasend.Bool(false)
	req.OnDelivered = ahasend.Bool(false)
	req.OnTransientError = ahasend.Bool(false)
	req.OnFailed = ahasend.Bool(false)
	req.OnBounced = ahasend.Bool(false)
	req.OnSuppressed = ahasend.Bool(false)
	req.OnOpened = ahasend.Bool(false)
	req.OnClicked = ahasend.Bool(false)
	req.OnSuppressionCreated = ahasend.Bool(false)
	req.OnDnsError = ahasend.Bool(false)
}

func setEventTypesUpdate(req *requests.UpdateWebhookRequest, events []string) {
	for _, event := range events {
		switch event {
		case "reception":
			req.OnReception = ahasend.Bool(true)
		case "delivered":
			req.OnDelivered = ahasend.Bool(true)
		case "transient_error":
			req.OnTransientError = ahasend.Bool(true)
		case "failed":
			req.OnFailed = ahasend.Bool(true)
		case "bounced":
			req.OnBounced = ahasend.Bool(true)
		case "suppressed":
			req.OnSuppressed = ahasend.Bool(true)
		case "opened":
			req.OnOpened = ahasend.Bool(true)
		case "clicked":
			req.OnClicked = ahasend.Bool(true)
		case "suppression_created":
			req.OnSuppressionCreated = ahasend.Bool(true)
		case "dns_error":
			req.OnDnsError = ahasend.Bool(true)
		}
	}
}
