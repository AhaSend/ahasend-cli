package webhooks

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
		Short: "Listen for webhook events in real-time",
		Long: `Listen for webhook events in real-time using WebSocket connection.

This command establishes a WebSocket connection to receive webhook events and can:
- Forward events to a local endpoint for development
- Filter events by type
- Display full or slim output
- Handle disconnections with buffered event replay

The command generates a webhook secret for signing forwarded events using
the standard-webhooks specification.`,
		Example: `  # Listen for all webhook events
  ahasend webhooks listen

  # Use existing webhook
  ahasend webhooks listen --webhook-id abcd1234-5678-90ef-abcd-1234567890ab

  # Forward events to local endpoint
  ahasend webhooks listen --forward-to http://localhost:3000/webhook

  # Filter specific event types
  ahasend webhooks listen --events message.opened,message.clicked

  # Slim output (only event types)
  ahasend webhooks listen --slim-output`,
		Args:         cobra.NoArgs,
		RunE:         runWebhooksListen,
		SilenceUsage: true,
	}

	cmd.Flags().String("webhook-id", "", "Use existing webhook instead of creating temporary one")
	cmd.Flags().StringSlice("events", []string{}, "Filter specific events (client-side)\nValid types: message.reception, message.delivered, message.transient_error,\nmessage.failed, message.bounced, message.suppressed, message.opened,\nmessage.clicked, suppression.created, domain.dns_error")
	cmd.Flags().String("forward-to", "", "Local endpoint to forward events to")
	cmd.Flags().Bool("skip-verify", false, "Skip SSL certificate verification for local endpoints when forwarding events")
	cmd.Flags().Bool("slim-output", false, "Slim down the payload for printing to the console")

	return cmd
}

func runWebhooksListen(cmd *cobra.Command, args []string) error {
	// Get authenticated client
	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	webhookID, _ := cmd.Flags().GetString("webhook-id")
	events, _ := cmd.Flags().GetStringSlice("events")
	forwardTo, _ := cmd.Flags().GetString("forward-to")
	skipVerify, _ := cmd.Flags().GetBool("skip-verify")
	slimOutput, _ := cmd.Flags().GetBool("slim-output")

	// Validate event types for listening (different from webhook creation)
	if err := validateListenEventTypes(events); err != nil {
		return err
	}

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"webhook_id":  webhookID,
		"events":      events,
		"forward_to":  forwardTo,
		"slim_output": slimOutput,
	}).Debug("Executing webhooks listen command")

	// Initiate webhook stream
	streamResponse, err := apiClient.InitiateWebhookStream(webhookID)
	if err != nil {
		return fmt.Errorf("failed to initiate webhook stream: %w", err)
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

	// Connect to WebSocket
	wsClient, err := connectWithRetry(apiClient, streamResponse.WsURL, streamResponse.WebhookID, skipVerify)
	if err != nil {
		return fmt.Errorf("failed to connect to websocket: %w", err)
	}
	defer wsClient.Close()

	// Print webhook info
	fmt.Println()
	color.New(color.FgGreen, color.Bold).Println("🔌 Webhook connected!")
	fmt.Println()
	fmt.Printf("Webhook ID: %s\n", color.YellowString(streamResponse.WebhookID))
	fmt.Printf("Secret: %s\n", color.YellowString(secret))
	if forwardTo != "" {
		fmt.Printf("Forwarding to: %s\n", color.CyanString(forwardTo))
	}
	fmt.Printf("Connected at: %s\n", color.GreenString(time.Now().Format("15:04:05")))
	fmt.Println()
	color.New(color.FgWhite).Println("Listening for events... (Press Ctrl+C to stop)")
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
		fmt.Println("\n\n🛑 Stopping webhook listener...")
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
					// Check if we should filter this event
					if shouldFilterEvent(msg.Event, events) {
						continue
					}

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

func connectWithRetry(apiClient client.AhaSendClient, wsURL, webhookID string, skipVerify bool) (*client.WebSocketClient, error) {
	// Try initial connection
	wsClient, err := apiClient.ConnectWebSocket(wsURL, webhookID, false, skipVerify)
	if err != nil {
		if strings.Contains(err.Error(), "another websocket connection exists") {
			// Prompt user for force reconnect
			fmt.Println()
			color.New(color.FgYellow).Println("⚠️  Another connection exists for this webhook.")
			fmt.Print("Do you want to force reconnect? (y/N): ")

			var response string
			fmt.Scanln(&response)

			if strings.ToLower(response) == "y" {
				wsClient, err = apiClient.ConnectWebSocket(wsURL, webhookID, true, skipVerify)
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

func shouldFilterEvent(event *client.Event, filters []string) bool {
	if len(filters) == 0 {
		return false
	}

	// Extract event type from Data
	if dataMap, ok := event.Data.(map[string]interface{}); ok {
		if eventType, ok := dataMap["type"].(string); ok {
			for _, filter := range filters {
				if eventType == filter {
					return false // Don't filter, this event matches
				}
			}
		}
	}

	return true // Filter out this event
}

func displayEvent(msg *client.WebSocketMessage, slimOutput bool) {
	timestamp := time.Unix(msg.Timestamp, 0).Format("15:04:05")

	if msg.Event == nil {
		return
	}

	// Extract event type from Data
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
		// Only show event type
		color.New(color.FgWhite, color.Bold).Println(eventType)
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
	}).Debug("Preparing to forward webhook event")

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
	}).Debug("Sending webhook forward request")

	// Send request
	startTime := time.Now()
	resp, err := httpClient.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		logger.Get().WithError(err).WithFields(map[string]interface{}{
			"url":      forwardTo,
			"duration": duration.String(),
		}).Error("Failed to forward webhook")
		return
	}
	defer resp.Body.Close()

	logger.Get().WithFields(map[string]interface{}{
		"url":      forwardTo,
		"status":   resp.StatusCode,
		"duration": duration.String(),
	}).Debug("Received webhook forward response")

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logger.Get().WithFields(map[string]interface{}{
			"url":    forwardTo,
			"status": resp.StatusCode,
		}).Debug("Successfully forwarded webhook event")
	} else {
		logger.Get().WithFields(map[string]interface{}{
			"url":    forwardTo,
			"status": resp.StatusCode,
		}).Warn("Webhook forward returned non-2xx status")
	}
}

func validateListenEventTypes(events []string) error {
	if len(events) == 0 {
		return nil // No events specified means no filtering
	}

	validEventTypes := map[string]bool{
		"message.reception":       true,
		"message.delivered":       true,
		"message.transient_error": true,
		"message.failed":          true,
		"message.bounced":         true,
		"message.suppressed":      true,
		"message.opened":          true,
		"message.clicked":         true,
		"suppression.created":     true,
		"domain.dns_error":        true,
	}

	var invalidEvents []string
	for _, event := range events {
		event = strings.TrimSpace(event)
		if !validEventTypes[event] {
			invalidEvents = append(invalidEvents, event)
		}
	}

	if len(invalidEvents) > 0 {
		validKeys := make([]string, 0, len(validEventTypes))
		for key := range validEventTypes {
			validKeys = append(validKeys, key)
		}
		return fmt.Errorf("invalid event types: %s\n\nValid event types are:\n%s",
			strings.Join(invalidEvents, ", "),
			strings.Join(validKeys, "\n"))
	}

	return nil
}
