package webhooks

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
)

// NewCreateCommand creates the create command
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new webhook",
		Long: `Create a new webhook endpoint to receive event notifications from AhaSend.

Webhooks allow your application to receive real-time notifications when events
occur in your AhaSend account, such as message deliveries, bounces, opens,
clicks, and more.

The command supports both interactive and non-interactive modes. In interactive
mode, you'll be prompted to select which events to listen for. In non-interactive
mode, you can specify event types using flags.

Required fields:
  - Name: A descriptive name for your webhook
  - URL: The endpoint URL where notifications will be sent

The webhook URL must be publicly accessible and support HTTPS for production use.`,
		Example: `  # Interactive webhook creation
  ahasend webhooks create

  # Create webhook with basic settings
  ahasend webhooks create --name "My Webhook" --url "https://example.com/webhook"

  # Create webhook with specific events
  ahasend webhooks create \
    --name "Delivery Webhook" \
    --url "https://api.example.com/webhooks/delivery" \
    --events "delivered,bounced,failed"

  # Create disabled webhook for testing
  ahasend webhooks create \
    --name "Test Webhook" \
    --url "https://test.example.com/webhook" \
    --disabled`,
		RunE:         runWebhooksCreate,
		SilenceUsage: true,
	}

	// Required flags
	cmd.Flags().String("name", "", "Webhook name (required)")
	cmd.Flags().String("url", "", "Webhook URL (required)")

	// Event type flags
	cmd.Flags().StringSlice("events", []string{}, "Comma-separated list of event types to listen for")
	cmd.Flags().Bool("all-events", false, "Listen for all available event types")

	// Optional configuration
	cmd.Flags().Bool("disabled", false, "Create webhook in disabled state")
	cmd.Flags().String("scope", "", "Webhook scope (optional)")
	cmd.Flags().StringSlice("domains", []string{}, "Limit webhook to specific domains")

	// Interactive mode control
	cmd.Flags().Bool("interactive", false, "Force interactive mode even when flags are provided")
	cmd.Flags().Bool("non-interactive", false, "Skip interactive prompts (use flag values only)")

	return cmd
}

func runWebhooksCreate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Get flags
	name, _ := cmd.Flags().GetString("name")
	webhookURL, _ := cmd.Flags().GetString("url")
	events, _ := cmd.Flags().GetStringSlice("events")
	allEvents, _ := cmd.Flags().GetBool("all-events")
	disabled, _ := cmd.Flags().GetBool("disabled")
	scope, _ := cmd.Flags().GetString("scope")
	domains, _ := cmd.Flags().GetStringSlice("domains")
	interactive, _ := cmd.Flags().GetBool("interactive")
	nonInteractive, _ := cmd.Flags().GetBool("non-interactive")

	// Validate conflicting flags
	if interactive && nonInteractive {
		return fmt.Errorf("cannot specify both --interactive and --non-interactive flags")
	}

	// Check if we should run in interactive mode
	shouldInteract := interactive || (!nonInteractive && (name == "" || webhookURL == ""))

	if shouldInteract {
		if nonInteractive {
			return fmt.Errorf("missing required flags: --name and --url are required in non-interactive mode")
		}

		// Run interactive webhook creation
		req, err := runInteractiveWebhookCreate()
		if err != nil {
			return handler.HandleError(err)
		}

		// Create the webhook
		webhook, err := createWebhook(client, *req)
		if err != nil {
			return handler.HandleError(err)
		}

		return handler.HandleCreateWebhook(webhook, printer.CreateConfig{
			SuccessMessage: fmt.Sprintf("Successfully created webhook: %s", webhook.Name),
			ItemName:       "webhook",
			FieldOrder:     []string{"id", "name", "url", "enabled", "event_types", "scope", "domains", "created_at"},
		})
	}

	// Non-interactive mode - validate required flags
	if name == "" {
		return fmt.Errorf("webhook name is required (use --name flag)")
	}
	if webhookURL == "" {
		return fmt.Errorf("webhook URL is required (use --url flag)")
	}

	// Validate URL format
	if err := validateWebhookURL(webhookURL); err != nil {
		return err
	}

	// Validate event types
	validatedEvents, err := validateEventTypes(events)
	if err != nil {
		return err
	}

	// Create webhook request
	req := requests.CreateWebhookRequest{
		Name:    name,
		URL:     webhookURL,
		Enabled: !disabled,
	}

	// Set event types
	if allEvents {
		setAllEventTypes(&req)
	} else if len(validatedEvents) > 0 {
		setEventTypes(&req, validatedEvents)
	}

	// Set optional fields
	if scope != "" {
		req.Scope = scope
	}
	if len(domains) > 0 {
		req.Domains = &domains
	}

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"name":       name,
		"url":        webhookURL,
		"events":     events,
		"all_events": allEvents,
		"disabled":   disabled,
		"scope":      scope,
		"domains":    domains,
	}).Debug("Executing webhooks create command")

	// Create the webhook
	webhook, err := createWebhook(client, req)
	if err != nil {
		return handler.HandleError(err)
	}

	// Use the new ResponseHandler to display created webhook
	return handler.HandleCreateWebhook(webhook, printer.CreateConfig{
		SuccessMessage: fmt.Sprintf("Successfully created webhook: %s", webhook.Name),
		ItemName:       "webhook",
		FieldOrder:     []string{"id", "name", "url", "enabled", "event_types", "scope", "domains", "created_at"},
	})
}

func runInteractiveWebhookCreate() (*requests.CreateWebhookRequest, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Creating a new webhook...")
	fmt.Println()

	// Get webhook name
	fmt.Print("Webhook name: ")
	name, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read webhook name: %w", err)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("webhook name cannot be empty")
	}

	// Get webhook URL
	fmt.Print("Webhook URL: ")
	webhookURL, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read webhook URL: %w", err)
	}
	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL == "" {
		return nil, fmt.Errorf("webhook URL cannot be empty")
	}

	// Validate URL
	if err := validateWebhookURL(webhookURL); err != nil {
		return nil, err
	}

	// Create request
	req := requests.CreateWebhookRequest{
		Name: name,
		URL:  webhookURL,
	}

	// Ask about enabled state
	fmt.Printf("Enable webhook immediately? (y/N): ")
	enabled, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read enabled preference: %w", err)
	}
	enabled = strings.TrimSpace(strings.ToLower(enabled))
	req.Enabled = enabled == "y" || enabled == "yes"

	// Interactive event selection
	fmt.Println("\nSelect event types to listen for:")
	fmt.Println("You can select multiple events by entering their numbers separated by commas (e.g., 1,3,5)")
	fmt.Println("Or enter 'all' to select all events, 'none' to select no events")
	fmt.Println()

	eventTypes := getAvailableEventTypes()
	for i, event := range eventTypes {
		fmt.Printf("  %d. %s - %s\n", i+1, event.Key, event.Description)
	}
	fmt.Println()

	fmt.Print("Select events (enter numbers, 'all', or 'none'): ")
	selection, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read event selection: %w", err)
	}
	selection = strings.TrimSpace(strings.ToLower(selection))

	switch selection {
	case "all":
		setAllEventTypes(&req)
	case "none", "":
		// No events selected - this is valid
	default:
		// Parse comma-separated numbers
		selectedEvents, err := parseEventSelection(selection, eventTypes)
		if err != nil {
			return nil, err
		}
		setEventTypes(&req, selectedEvents)
	}

	// Optional: Ask about scope and domains
	fmt.Print("\nWebhook scope (optional, press Enter to skip): ")
	scope, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read scope: %w", err)
	}
	scope = strings.TrimSpace(scope)
	if scope != "" {
		req.Scope = scope
	}

	fmt.Print("Limit to specific domains (optional, comma-separated, press Enter to skip): ")
	domainsInput, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read domains: %w", err)
	}
	domainsInput = strings.TrimSpace(domainsInput)
	if domainsInput != "" {
		domains := strings.Split(domainsInput, ",")
		for i, domain := range domains {
			domains[i] = strings.TrimSpace(domain)
		}
		req.Domains = &domains
	}

	return &req, nil
}

func createWebhook(client client.AhaSendClient, req requests.CreateWebhookRequest) (*responses.Webhook, error) {
	webhook, err := client.CreateWebhook(req)
	if err != nil {
		return nil, err
	}
	return webhook, nil
}

func validateWebhookURL(webhookURL string) error {
	parsedURL, err := url.Parse(webhookURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %s", err)
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("webhook URL must include scheme and host (e.g., https://example.com/webhook)")
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("webhook URL must use HTTP or HTTPS")
	}

	// Recommend HTTPS for production
	if parsedURL.Scheme == "http" && !strings.Contains(parsedURL.Host, "localhost") && !strings.Contains(parsedURL.Host, "127.0.0.1") {
		// This is a warning, not an error
		fmt.Println("⚠️  Warning: HTTP URLs are not recommended for production webhooks. Consider using HTTPS.")
	}

	return nil
}

type EventType struct {
	Key         string
	Description string
}

func getAvailableEventTypes() []EventType {
	return []EventType{
		{"reception", "Message reception (inbound email received)"},
		{"delivered", "Message delivered successfully"},
		{"transient_error", "Temporary delivery failure (will retry)"},
		{"failed", "Permanent delivery failure"},
		{"bounced", "Message bounced (invalid recipient)"},
		{"suppressed", "Message suppressed (recipient opted out)"},
		{"opened", "Message opened by recipient"},
		{"clicked", "Link clicked in message"},
		{"suppression_created", "New suppression entry created"},
		{"dns_error", "DNS configuration error for domain"},
	}
}

func validateEventTypes(events []string) ([]string, error) {
	if len(events) == 0 {
		return []string{}, nil
	}

	validEventTypes := make(map[string]bool)
	for _, event := range getAvailableEventTypes() {
		validEventTypes[event.Key] = true
	}

	var validatedEvents []string
	var invalidEvents []string

	for _, event := range events {
		event = strings.TrimSpace(event)
		if validEventTypes[event] {
			validatedEvents = append(validatedEvents, event)
		} else {
			invalidEvents = append(invalidEvents, event)
		}
	}

	if len(invalidEvents) > 0 {
		return nil, fmt.Errorf("invalid event types: %s. Valid types: %s",
			strings.Join(invalidEvents, ", "),
			strings.Join(getValidEventTypeKeys(), ", "))
	}

	return validatedEvents, nil
}

func getValidEventTypeKeys() []string {
	eventTypes := getAvailableEventTypes()
	keys := make([]string, len(eventTypes))
	for i, event := range eventTypes {
		keys[i] = event.Key
	}
	return keys
}

func parseEventSelection(selection string, eventTypes []EventType) ([]string, error) {
	parts := strings.Split(selection, ",")
	var selectedEvents []string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		num, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid selection: %s (must be a number)", part)
		}

		if num < 1 || num > len(eventTypes) {
			return nil, fmt.Errorf("invalid selection: %d (must be between 1 and %d)", num, len(eventTypes))
		}

		selectedEvents = append(selectedEvents, eventTypes[num-1].Key)
	}

	return selectedEvents, nil
}

func setAllEventTypes(req *requests.CreateWebhookRequest) {
	req.OnReception = true
	req.OnDelivered = true
	req.OnTransientError = true
	req.OnFailed = true
	req.OnBounced = true
	req.OnSuppressed = true
	req.OnOpened = true
	req.OnClicked = true
	req.OnSuppressionCreated = true
	req.OnDnsError = true
}

func setEventTypes(req *requests.CreateWebhookRequest, events []string) {
	for _, event := range events {
		switch event {
		case "reception":
			req.OnReception = true
		case "delivered":
			req.OnDelivered = true
		case "transient_error":
			req.OnTransientError = true
		case "failed":
			req.OnFailed = true
		case "bounced":
			req.OnBounced = true
		case "suppressed":
			req.OnSuppressed = true
		case "opened":
			req.OnOpened = true
		case "clicked":
			req.OnClicked = true
		case "suppression_created":
			req.OnSuppressionCreated = true
		case "dns_error":
			req.OnDnsError = true
		}
	}
}
