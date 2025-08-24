package routes

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
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
		Short: "Create a new inbound email route",
		Long: `Create a new inbound email route to process incoming emails through webhooks.
Routes allow you to configure how incoming emails are handled, filtered,
and forwarded to your application endpoints.

You can create routes either interactively (guided configuration) or
non-interactively using command-line flags.

Interactive mode provides step-by-step guidance for configuring:
- Route name and webhook URL
- Recipient filtering patterns
- Processing options (attachments, headers, etc.)
- Route status (enabled/disabled)

Non-interactive mode allows automation and scripting by providing
all configuration through flags.`,
		Example: `  # Interactive route creation
  ahasend routes create

  # Non-interactive with required parameters
  ahasend routes create --name "Support Route" --url "https://api.example.com/webhook"

  # Route with recipient filtering
  ahasend routes create \
    --name "Help Desk" \
    --url "https://api.example.com/support" \
    --recipient "support@*" \
    --include-attachments \
    --enabled

  # Route with advanced processing options
  ahasend routes create \
    --name "Sales Inquiries" \
    --url "https://api.example.com/sales" \
    --recipient "*sales*" \
    --include-headers \
    --group-by-message-id \
    --strip-replies \
    --enabled`,
		RunE:         runRoutesCreate,
		SilenceUsage: true,
	}

	// Add flags
	cmd.Flags().String("name", "", "Route name")
	cmd.Flags().String("url", "", "Webhook URL for processing emails")
	cmd.Flags().String("recipient", "", "Recipient filter pattern (e.g., 'support@*', '*@example.com')")
	cmd.Flags().Bool("include-attachments", false, "Include email attachments in webhook payload")
	cmd.Flags().Bool("include-headers", false, "Include email headers in webhook payload")
	cmd.Flags().Bool("group-by-message-id", false, "Group related emails by message ID (conversation threading)")
	cmd.Flags().Bool("strip-replies", false, "Strip reply content from emails")
	cmd.Flags().Bool("enabled", false, "Enable the route immediately after creation")
	cmd.Flags().Bool("interactive", true, "Use interactive mode for route configuration")

	return cmd
}

func runRoutesCreate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Get flags
	name, _ := cmd.Flags().GetString("name")
	webhookURL, _ := cmd.Flags().GetString("url")
	recipient, _ := cmd.Flags().GetString("recipient")
	includeAttachments, _ := cmd.Flags().GetBool("include-attachments")
	includeHeaders, _ := cmd.Flags().GetBool("include-headers")
	groupByMessageID, _ := cmd.Flags().GetBool("group-by-message-id")
	stripReplies, _ := cmd.Flags().GetBool("strip-replies")
	enabled, _ := cmd.Flags().GetBool("enabled")
	interactive, _ := cmd.Flags().GetBool("interactive")

	// Determine if we need interactive mode
	needsInteractive := interactive && (name == "" || webhookURL == "")

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"interactive": needsInteractive,
		"name":        name,
		"url":         webhookURL,
		"recipient":   recipient,
	}).Debug("Executing routes create command")

	// Collect route configuration
	var config RouteCreateConfig
	if needsInteractive {
		config, err = collectRouteConfigInteractive()
		if err != nil {
			return err
		}
	} else {
		if name == "" || webhookURL == "" {
			return fmt.Errorf("name and url are required. Use --interactive for guided setup or provide both --name and --url flags")
		}

		config = RouteCreateConfig{
			Name:               name,
			URL:                webhookURL,
			Recipient:          recipient,
			IncludeAttachments: includeAttachments,
			IncludeHeaders:     includeHeaders,
			GroupByMessageID:   groupByMessageID,
			StripReplies:       stripReplies,
			Enabled:            enabled,
		}
	}

	// Validate configuration
	if err := validateRouteConfig(config); err != nil {
		return err
	}

	// Create the route
	route, err := createRoute(client, config)
	if err != nil {
		return err
	}

	// Use the new ResponseHandler to display created route
	return handler.HandleCreateRoute(route, printer.CreateConfig{
		SuccessMessage: "Route created successfully",
		ItemName:       "route",
		FieldOrder:     []string{"id", "name", "url", "enabled", "recipient", "attachments", "headers", "group_by_message_id", "strip_replies", "created_at", "updated_at"},
	})
}

type RouteCreateConfig struct {
	Name               string
	URL                string
	Recipient          string
	IncludeAttachments bool
	IncludeHeaders     bool
	GroupByMessageID   bool
	StripReplies       bool
	Enabled            bool
}

func collectRouteConfigInteractive() (RouteCreateConfig, error) {
	reader := bufio.NewReader(os.Stdin)
	var config RouteCreateConfig

	fmt.Println("🔧 Creating a new inbound email route")
	fmt.Println("This will guide you through setting up email processing via webhooks.")
	fmt.Println()

	// Route name
	fmt.Print("Route name: ")
	name, err := reader.ReadString('\n')
	if err != nil {
		return config, fmt.Errorf("failed to read route name: %w", err)
	}
	config.Name = strings.TrimSpace(name)

	// Webhook URL
	fmt.Print("Webhook URL (where emails will be sent): ")
	webhookURL, err := reader.ReadString('\n')
	if err != nil {
		return config, fmt.Errorf("failed to read webhook URL: %w", err)
	}
	config.URL = strings.TrimSpace(webhookURL)

	// Recipient filtering (optional)
	fmt.Print("Recipient filter (optional, e.g., 'support@*', '*@example.com'): ")
	recipient, err := reader.ReadString('\n')
	if err != nil {
		return config, fmt.Errorf("failed to read recipient filter: %w", err)
	}
	config.Recipient = strings.TrimSpace(recipient)

	fmt.Println("\n📧 Processing Options:")

	// Include attachments
	fmt.Print("Include email attachments in webhook payload? (y/N): ")
	attachments, err := reader.ReadString('\n')
	if err != nil {
		return config, fmt.Errorf("failed to read attachments option: %w", err)
	}
	config.IncludeAttachments = strings.ToLower(strings.TrimSpace(attachments)) == "y"

	// Include headers
	fmt.Print("Include email headers in webhook payload? (y/N): ")
	headers, err := reader.ReadString('\n')
	if err != nil {
		return config, fmt.Errorf("failed to read headers option: %w", err)
	}
	config.IncludeHeaders = strings.ToLower(strings.TrimSpace(headers)) == "y"

	// Group by message ID
	fmt.Print("Group related emails by message ID (conversation threading)? (y/N): ")
	grouping, err := reader.ReadString('\n')
	if err != nil {
		return config, fmt.Errorf("failed to read grouping option: %w", err)
	}
	config.GroupByMessageID = strings.ToLower(strings.TrimSpace(grouping)) == "y"

	// Strip replies
	fmt.Print("Strip reply content from emails (cleaner processing)? (y/N): ")
	stripReplies, err := reader.ReadString('\n')
	if err != nil {
		return config, fmt.Errorf("failed to read strip replies option: %w", err)
	}
	config.StripReplies = strings.ToLower(strings.TrimSpace(stripReplies)) == "y"

	// Enable immediately
	fmt.Print("Enable route immediately? (Y/n): ")
	enable, err := reader.ReadString('\n')
	if err != nil {
		return config, fmt.Errorf("failed to read enable option: %w", err)
	}
	enableInput := strings.ToLower(strings.TrimSpace(enable))
	config.Enabled = enableInput == "" || enableInput == "y"

	fmt.Println()
	return config, nil
}

func validateRouteConfig(config RouteCreateConfig) error {
	// Validate required fields
	if config.Name == "" {
		return fmt.Errorf("route name is required")
	}

	if config.URL == "" {
		return fmt.Errorf("webhook URL is required")
	}

	// Validate URL format
	parsedURL, err := url.Parse(config.URL)
	if err != nil {
		return fmt.Errorf("invalid webhook URL format: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("webhook URL must use http or https scheme")
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("webhook URL must include a valid host")
	}

	// Security warning for HTTP URLs
	if parsedURL.Scheme == "http" {
		fmt.Println("⚠️  Warning: Using HTTP URL for webhook. Consider using HTTPS for production.")
	}

	return nil
}

func createRoute(client client.AhaSendClient, config RouteCreateConfig) (*responses.Route, error) {
	// Build create request
	req := requests.CreateRouteRequest{
		Name: config.Name,
		URL:  config.URL,
	}

	// Set optional fields
	req.Recipient = config.Recipient
	req.Attachments = config.IncludeAttachments
	req.Headers = config.IncludeHeaders
	req.GroupByMessageId = config.GroupByMessageID
	req.StripReplies = config.StripReplies
	req.Enabled = config.Enabled

	// Create the route
	route, err := client.CreateRoute(req)
	if err != nil {
		return nil, err
	}

	return route, nil
}
