package routes

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/webhooks"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// NewListenCommand creates the listen command
func NewListenCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "listen",
		Short: "Listen for inbound email events in real-time",
		Long: `Listen for inbound email routing events in real-time using WebSocket connection.

This command establishes a WebSocket connection to receive inbound email events and can:
- Forward events to a local endpoint for development
- Display events in full or slim output format
- Handle disconnections with buffered event replay
- Use existing routes or create temporary routes with recipient patterns

The command generates a webhook secret for signing forwarded events using
the standard-webhooks specification.`,
		Example: `  # Listen with existing route
  ahasend routes listen --route-id abcd1234-5678-90ef-abcd-1234567890ab

  # Listen with recipient pattern (backend creates temporary route)
  ahasend routes listen --recipient "*@example.com"

  # Forward events to local endpoint
  ahasend routes listen --recipient "support-*@example.com" \
    --forward-to http://localhost:3000/webhook

  # Slim output (minimal event display)
  ahasend routes listen --route-id abc123 --slim-output`,
		Args:         cobra.NoArgs,
		RunE:         runRoutesListen,
		SilenceUsage: true,
	}

	cmd.Flags().String("route-id", "", "Use existing route instead of creating temporary one")
	cmd.Flags().String("recipient", "", "Recipient pattern for temporary route (e.g., *@domain.com)")
	cmd.Flags().String("forward-to", "", "Local endpoint to forward events to")
	cmd.Flags().Bool("skip-verify", false, "Skip SSL certificate verification for local endpoints when forwarding events")
	cmd.Flags().Bool("slim-output", false, "Slim down the payload for printing to the console")

	return cmd
}

func runRoutesListen(cmd *cobra.Command, args []string) error {
	// Get authenticated client
	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	routeID, _ := cmd.Flags().GetString("route-id")
	recipient, _ := cmd.Flags().GetString("recipient")
	forwardTo, _ := cmd.Flags().GetString("forward-to")
	skipVerify, _ := cmd.Flags().GetBool("skip-verify")
	slimOutput, _ := cmd.Flags().GetBool("slim-output")

	// Validate parameters - exactly one must be provided
	if err := validateListenParameters(routeID, recipient); err != nil {
		return err
	}

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"route_id":    routeID,
		"recipient":   recipient,
		"forward_to":  forwardTo,
		"slim_output": slimOutput,
	}).Debug("Executing routes listen command")

	// Initiate route stream
	streamResponse, err := apiClient.InitiateRouteStream(routeID, recipient)
	if err != nil {
		return fmt.Errorf("failed to initiate route stream: %w", err)
	}

	// Generate webhook secret
	secret, err := webhooks.GenerateWebhookSecret()
	if err != nil {
		return fmt.Errorf("failed to generate webhook secret: %w", err)
	}

	// Create signer for forwarding
	var signer *webhooks.Signer
	if forwardTo != "" {
		signer = webhooks.NewSigner(secret)
	}

	// Connect to WebSocket - use the route ID from response as the connection ID
	wsClient, err := connectWithRetry(apiClient, streamResponse.WsURL, streamResponse.RouteID, skipVerify)
	if err != nil {
		return fmt.Errorf("failed to connect to websocket: %w", err)
	}
	defer wsClient.Close()

	// Print connection info
	fmt.Println()
	color.New(color.FgGreen, color.Bold).Println("📧 Route connected!")
	fmt.Println()
	fmt.Printf("Route ID: %s\n", color.YellowString(streamResponse.RouteID))
	if recipient != "" {
		fmt.Printf("Recipient Pattern: %s\n", color.CyanString(recipient))
		color.New(color.FgYellow).Println("Note: This is a temporary route that will be cleaned up automatically")
	}
	fmt.Printf("Secret: %s\n", color.YellowString(secret))
	if forwardTo != "" {
		fmt.Printf("Forwarding to: %s\n", color.CyanString(forwardTo))
	}
	fmt.Printf("Connected at: %s\n", color.GreenString(time.Now().Format("15:04:05")))
	fmt.Println()
	color.New(color.FgWhite).Println("Listening for inbound emails... (Press Ctrl+C to stop)")
	fmt.Println(strings.Repeat("─", 60))

	// Create HTTP client for forwarding
	var httpClient *http.Client
	if forwardTo != "" {
		httpClient = &http.Client{
			Timeout: 10 * time.Second,
		}
		if skipVerify {
			httpClient.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}
	}

	// Handle interrupt signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n\n🛑 Stopping route listener...")
		cancel()
	}()

	// Create channels for WebSocket messages and errors
	msgChan := make(chan *client.WebSocketMessage, 10)
	errChan := make(chan error, 1)

	// Start goroutine to read WebSocket messages without interfering with ping/pong
	go func() {
		defer close(msgChan)
		defer close(errChan)

		for {
			// Read message without any artificial timeouts - let WebSocket handle its own
			msg, err := wsClient.ReadMessage(context.Background())
			if err != nil {
				select {
				case errChan <- err:
				case <-ctx.Done():
				}
				return
			}

			select {
			case msgChan <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Listen for messages and cancellation
	for {
		select {
		case <-ctx.Done():
			// Close WebSocket connection gracefully
			wsClient.Close()
			return nil

		case err := <-errChan:
			if err != nil {
				// Check if it's a connection close or fatal error
				if strings.Contains(err.Error(), "websocket connection closed") ||
					strings.Contains(err.Error(), "repeated read on failed") ||
					strings.Contains(err.Error(), "websocket connection is closed") {
					fmt.Println("\n💔 WebSocket connection closed")
					wsClient.Close()
					return nil
				}
				logger.Get().WithError(err).Error("Failed to read websocket message")
				wsClient.Close()
				return fmt.Errorf("websocket error: %w", err)
			}

		case msg := <-msgChan:
			if msg == nil {
				continue
			}

			// Handle message based on type
			switch msg.Type {
			case "connected":
				color.New(color.FgGreen).Printf("✓ Session established: %s\n", msg.SessionID)
				fmt.Println(strings.Repeat("─", 60))

			case "event", "replay":
				if msg.Event != nil {
					// Display event
					displayEvent(msg, slimOutput)

					// Forward event if configured
					if forwardTo != "" && signer != nil {
						go forwardEvent(httpClient, forwardTo, msg.Event, signer)
					}
				}

			default:
				logger.Get().WithField("type", msg.Type).Debug("Received unknown message type")
			}
		}
	}
}

func validateListenParameters(routeID, recipient string) error {
	// Check that exactly one parameter is provided
	if routeID == "" && recipient == "" {
		return fmt.Errorf("either --route-id or --recipient must be provided")
	}
	if routeID != "" && recipient != "" {
		return fmt.Errorf("only one of --route-id or --recipient can be provided, not both")
	}

	// Basic validation of recipient pattern if provided
	if recipient != "" {
		// Check if it contains @ sign
		if !strings.Contains(recipient, "@") {
			return fmt.Errorf("recipient pattern must be an email pattern (e.g., *@domain.com)")
		}
		// Basic wildcard pattern validation
		if strings.Count(recipient, "*") > 2 {
			return fmt.Errorf("recipient pattern contains too many wildcards")
		}
	}

	return nil
}

func connectWithRetry(apiClient client.AhaSendClient, wsURL, routeID string, skipVerify bool) (*client.WebSocketClient, error) {
	// Try initial connection - for routes, we use the route ID as the connection identifier
	wsClient, err := apiClient.ConnectWebSocket(wsURL, routeID, false, skipVerify)
	if err != nil {
		if strings.Contains(err.Error(), "another websocket connection exists") {
			// Prompt user for force reconnect
			fmt.Println()
			color.New(color.FgYellow).Println("⚠️  Another connection exists for this route.")
			fmt.Print("Do you want to force reconnect? (y/N): ")

			var response string
			fmt.Scanln(&response)

			if strings.ToLower(response) == "y" {
				wsClient, err = apiClient.ConnectWebSocket(wsURL, routeID, true, skipVerify)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, fmt.Errorf("aborted: another connection exists")
			}
		} else {
			return nil, err
		}
	}

	return wsClient, nil
}

func displayEvent(msg *client.WebSocketMessage, slimOutput bool) {
	timestamp := time.Unix(msg.Timestamp, 0).Format("15:04:05")

	if msg.Event == nil {
		return
	}

	// Extract event type from Data - for routes it should be "message.routing"
	var eventType string
	if dataMap, ok := msg.Event.Data.(map[string]interface{}); ok {
		if et, ok := dataMap["type"].(string); ok {
			eventType = et
		}
	}

	// Display based on message type (event vs replay)
	if msg.Type == "replay" {
		color.New(color.FgYellow).Printf("[%s] 🔄 REPLAY: ", timestamp)
	} else {
		color.New(color.FgCyan).Printf("[%s] 📨 ", timestamp)
	}

	if slimOutput {
		// Only show event type and basic info
		color.New(color.FgWhite, color.Bold).Print(eventType)

		// Try to extract and show sender/recipient for routing events
		if dataMap, ok := msg.Event.Data.(map[string]interface{}); ok {
			if from, ok := dataMap["from"].(string); ok {
				fmt.Printf(" from: %s", color.GreenString(from))
			}
			if to, ok := dataMap["to"].(string); ok {
				fmt.Printf(" to: %s", color.CyanString(to))
			}
			if subject, ok := dataMap["subject"].(string); ok {
				fmt.Printf(" subject: %s", color.WhiteString(subject))
			}
		}
		fmt.Println()
	} else {
		// Show full payload
		color.New(color.FgWhite, color.Bold).Println(eventType)

		// Pretty print the data
		jsonData, err := json.MarshalIndent(msg.Event.Data, "  ", "  ")
		if err == nil {
			fmt.Printf("  %s\n", string(jsonData))
		}
	}

	fmt.Println(strings.Repeat("─", 60))
}

func forwardEvent(httpClient *http.Client, forwardTo string, event *client.Event, signer *webhooks.Signer) {
	// Prepare payload
	payload, err := json.Marshal(event.Data)
	if err != nil {
		logger.Get().WithError(err).Error("Failed to marshal event data for forwarding")
		return
	}

	// Generate message ID and timestamp
	msgID := webhooks.GenerateMsgID()
	timestamp := time.Now()

	logger.Get().WithFields(map[string]interface{}{
		"msg_id":       msgID,
		"timestamp":    timestamp.Unix(),
		"url":          forwardTo,
		"payload_size": len(payload),
	}).Debug("Preparing to forward route event")

	// Sign the payload
	signature, err := signer.Sign(msgID, timestamp, payload)
	if err != nil {
		logger.Get().WithError(err).Error("Failed to sign webhook payload")
		return
	}

	// Create request
	req, err := http.NewRequest("POST", forwardTo, bytes.NewReader(payload))
	if err != nil {
		logger.Get().WithError(err).Error("Failed to create forward request")
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("webhook-id", msgID)
	req.Header.Set("webhook-timestamp", fmt.Sprintf("%d", timestamp.Unix()))
	req.Header.Set("webhook-signature", signature)

	logger.Get().WithFields(map[string]interface{}{
		"method": "POST",
		"url":    forwardTo,
		"headers": map[string]string{
			"Content-Type":      "application/json",
			"webhook-id":        msgID,
			"webhook-timestamp": fmt.Sprintf("%d", timestamp.Unix()),
			"webhook-signature": signature,
		},
	}).Debug("Sending route event forward request")

	// Send request
	startTime := time.Now()
	resp, err := httpClient.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		logger.Get().WithError(err).WithFields(map[string]interface{}{
			"url":      forwardTo,
			"duration": duration.String(),
		}).Error("Failed to forward route event")
		return
	}
	defer resp.Body.Close()

	logger.Get().WithFields(map[string]interface{}{
		"url":      forwardTo,
		"status":   resp.StatusCode,
		"duration": duration.String(),
	}).Debug("Received route event forward response")

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logger.Get().WithFields(map[string]interface{}{
			"url":    forwardTo,
			"status": resp.StatusCode,
		}).Debug("Successfully forwarded route event")
	} else {
		logger.Get().WithFields(map[string]interface{}{
			"url":    forwardTo,
			"status": resp.StatusCode,
		}).Warn("Route event forward returned non-2xx status")
	}
}
