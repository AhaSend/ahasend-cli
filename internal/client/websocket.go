package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/version"
	"github.com/AhaSend/ahasend-go/api"
	"github.com/gorilla/websocket"
)

type WebhookStreamResponse struct {
	WebhookID string `json:"webhook_id"`
	WsURL     string `json:"ws_url"`
}

type WebSocketMessage struct {
	Type      string `json:"type"`
	Event     *Event `json:"event,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Stream    string `json:"stream,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

type Event struct {
	Type      string            `json:"type"`
	StreamID  string            `json:"stream_id"`
	AccountID string            `json:"account_id"`
	Data      interface{}       `json:"data"`
	Metadata  map[string]string `json:"metadata"`
	Timestamp int64             `json:"timestamp"`
}

type WebSocketClient struct {
	conn       *websocket.Conn
	apiKey     string
	webhookID  string
	skipVerify bool
	closed     bool
}

func (c *Client) InitiateWebhookStream(webhookID string) (*WebhookStreamResponse, error) {
	endpoint := fmt.Sprintf("/v2/accounts/%s/webhooks/stream", c.accountID)

	// Create request URL using SDK configuration
	baseURL := fmt.Sprintf("%s://%s", c.config.Scheme, c.config.Host)
	fullURL := fmt.Sprintf("%s%s", baseURL, endpoint)

	req, err := http.NewRequest("POST", fullURL, nil)
	if err != nil {
		return nil, err
	}

	// Add authorization header
	apiKey := c.auth.Value(api.ContextAccessToken).(string)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Set("User-Agent", c.config.UserAgent)

	if webhookID != "" {
		q := req.URL.Query()
		q.Add("webhook_id", webhookID)
		req.URL.RawQuery = q.Encode()
	}

	// Execute request with rate limiting
	ctx := context.Background()
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	resp, err := c.config.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response WebhookStreamResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (c *Client) ConnectWebSocket(wsURL, webhookID string, forceReconnect, skipVerify bool) (*WebSocketClient, error) {
	u, err := url.Parse(wsURL)
	if err != nil {
		return nil, fmt.Errorf("invalid websocket URL: %w", err)
	}

	// Get API key from auth context
	apiKey := c.auth.Value(api.ContextAccessToken).(string)

	q := u.Query()
	q.Add("api_secret_key", apiKey)
	if forceReconnect {
		q.Add("force_reconnect", "true")
	}
	u.RawQuery = q.Encode()

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	if skipVerify {
		dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	headers := http.Header{}
	headers.Set("User-Agent", fmt.Sprintf("AhaSend-CLI-%s", version.Version))

	conn, resp, err := dialer.Dial(u.String(), headers)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusConflict {
			return nil, fmt.Errorf("another websocket connection exists for this webhook. Use force_reconnect to drop the old connection")
		}
		return nil, fmt.Errorf("websocket connection failed: %w", err)
	}

	// Set up ping/pong handler
	conn.SetPingHandler(func(appData string) error {
		logger.Get().Debug("Received ping, responding with pong")
		err := conn.WriteMessage(websocket.PongMessage, []byte(appData))
		if err != nil {
			logger.Get().WithError(err).Debug("Failed to send pong")
		}
		return err
	})

	// Set up close handler to detect server-initiated closures
	conn.SetCloseHandler(func(code int, text string) error {
		logger.Get().WithFields(map[string]interface{}{
			"code": code,
			"text": text,
		}).Debug("WebSocket close received")
		return nil
	})

	return &WebSocketClient{
		conn:       conn,
		apiKey:     apiKey,
		webhookID:  webhookID,
		skipVerify: skipVerify,
	}, nil
}

func (wsc *WebSocketClient) ReadMessage(ctx context.Context) (*WebSocketMessage, error) {
	// Check if connection is already closed
	if wsc.closed {
		return nil, fmt.Errorf("websocket connection is closed")
	}

	// Don't set artificial deadlines - let WebSocket handle its own timeouts
	// This allows proper ping/pong handling without interference

	messageType, data, err := wsc.conn.ReadMessage()
	if err != nil {
		// Mark connection as closed on any read error to prevent future panics
		wsc.closed = true

		// Check if it's a websocket close error or connection error
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			return nil, fmt.Errorf("websocket connection closed: %w", err)
		}

		return nil, err
	}

	if messageType != websocket.TextMessage {
		return nil, fmt.Errorf("unexpected message type: %d", messageType)
	}

	var msg WebSocketMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse websocket message: %w", err)
	}

	return &msg, nil
}

func (wsc *WebSocketClient) Close() error {
	wsc.closed = true
	if wsc.conn != nil {
		return wsc.conn.Close()
	}
	return nil
}
