package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/api"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type WebhookStreamResponse struct {
	WebhookID string `json:"webhook_id"`
	WsURL     string `json:"ws_url"`
}

type RouteStreamResponse struct {
	RouteID string `json:"route_id"`
	WsURL   string `json:"ws_url"`
}

type WebSocketMessage struct {
	Type      string `json:"type"`
	Event     *Event `json:"event,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Stream    string `json:"stream,omitempty"`
	Timestamp int64  `json:"timestamp"`
	// Warnings are what the connected frame of a webhook stream says the
	// stream will not send, such as the events of a family whose read scope
	// the API key lacks. Route streams, and older servers, send none.
	Warnings []string `json:"warnings,omitempty"`
}

type Event struct {
	Type      string `json:"type"`
	StreamID  string `json:"stream_id"`
	AccountID string `json:"account_id"`
	// Data is the event's payload exactly as the server sent it, so it is
	// forwarded byte for byte.
	Data      json.RawMessage   `json:"data"`
	Metadata  map[string]string `json:"metadata"`
	Timestamp int64             `json:"timestamp"`
}

// ErrStreamTaken is the error ConnectWebSocket returns when another
// connection holds the stream of a webhook or a route. Connecting with
// forceReconnect drops the other connection.
var ErrStreamTaken = errors.New("another websocket connection holds the stream")

// ErrStreamClosed is the error ReadMessage returns when the server closes the
// connection normally, and on every read after the connection was closed or
// failed.
var ErrStreamClosed = errors.New("websocket connection closed")

// ErrStreamFailed is the error ReadMessage returns when the connection
// failed: it dropped, or the server closed it with an error code.
var ErrStreamFailed = errors.New("websocket connection failed")

// ErrUndecodableMessage is the error ReadMessage returns for a message that
// is not a JSON text message. The connection stays open, so the next read
// returns the next message.
var ErrUndecodableMessage = errors.New("stream message cannot be read")

// errMessageTooLarge is the error ReadMessage returns for a message larger
// than streamReadLimit. The connection is closed, so the stream ends.
var errMessageTooLarge = fmt.Errorf("a stream message is larger than %d MiB", streamReadLimit>>20)

type WebSocketClient struct {
	conn *websocket.Conn
	// closed is set by Close, from any goroutine, and by a failed read.
	closed atomic.Bool
}

// InitiateWebhookStream asks the API for a websocket URL that streams the
// events of webhookID, or of a temporary webhook when webhookID is nil.
func (c *Client) InitiateWebhookStream(webhookID *uuid.UUID) (*WebhookStreamResponse, error) {
	endpoint := fmt.Sprintf("/v2/accounts/%s/webhooks/stream", c.accountUUID)
	query := url.Values{}
	if webhookID != nil {
		query.Set("webhook_id", webhookID.String())
	}
	return post[WebhookStreamResponse](c, endpoint, query, nil)
}

// InitiateRouteStream asks the API for a websocket URL that streams the
// emails of routeID, or of a temporary route for recipient. The caller sets
// exactly one of them.
func (c *Client) InitiateRouteStream(routeID *uuid.UUID, recipient string) (*RouteStreamResponse, error) {
	endpoint := fmt.Sprintf("/v2/accounts/%s/routes/stream", c.accountUUID)
	query := url.Values{}
	if routeID != nil {
		query.Set("route_id", routeID.String())
	}
	if recipient != "" {
		query.Set("recipient", recipient)
	}
	return post[RouteStreamResponse](c, endpoint, query, nil)
}

// streamReadLimit is the size of the largest stream message ReadMessage
// accepts; a larger one ends the stream with errMessageTooLarge.
const streamReadLimit = 16 << 20

// ConnectWebSocket connects to the stream at wsURL, sending the API key in the
// Authorization header. The URL must use wss, and the connection always
// verifies the server's TLS certificate; only a loopback host may use plain
// ws. The connection goes through the proxy the environment sets, as API
// requests do.
func (c *Client) ConnectWebSocket(wsURL string, forceReconnect bool) (*WebSocketClient, error) {
	if wsURL == "" {
		return nil, errors.New("the API returned no stream URL")
	}
	u, err := url.Parse(wsURL)
	if err != nil {
		return nil, fmt.Errorf("invalid websocket URL: %w", err)
	}
	if u.Scheme != "wss" && (u.Scheme != "ws" || !validation.IsLoopbackHost(u.Hostname())) {
		return nil, fmt.Errorf("refusing to send the API key to %s://%s: the stream URL must use wss", u.Scheme, u.Host)
	}

	apiKey := c.auth.Value(api.ContextAccessToken).(string)

	if forceReconnect {
		q := u.Query()
		q.Add("force_reconnect", "true")
		u.RawQuery = q.Encode()
	}

	headers := http.Header{}
	headers.Set("User-Agent", c.config.UserAgent)
	headers.Set("Authorization", "Bearer "+apiKey)

	conn, resp, err := newStreamDialer().Dial(u.String(), headers)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusConflict {
			return nil, ErrStreamTaken
		}
		if resp != nil {
			return nil, fmt.Errorf("the stream refused the connection: %w", handshakeError(resp))
		}
		return nil, fmt.Errorf("websocket connection failed: %w", err)
	}
	conn.SetReadLimit(streamReadLimit)

	// Answer a ping with a pong, as the websocket library does, and wait
	// for the next message as long again
	conn.SetPingHandler(func(appData string) error {
		logger.Get().Debug("Received ping, responding with pong")
		if err := conn.SetReadDeadline(time.Now().Add(streamReadTimeout)); err != nil {
			return err
		}
		err := conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(controlWriteTimeout))
		if err != nil && !errors.Is(err, websocket.ErrCloseSent) {
			logger.Get().WithError(err).Debug("Failed to send pong")
			return err
		}
		return nil
	})

	// Answer the server's close frame with one, as the websocket library
	// does, so the server sees the connection closed cleanly
	conn.SetCloseHandler(func(code int, text string) error {
		logger.Get().WithFields(map[string]interface{}{
			"code": code,
			"text": text,
		}).Debug("WebSocket close received")
		message := websocket.FormatCloseMessage(code, "")
		if code == websocket.CloseNoStatusReceived {
			message = websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
		}
		_ = conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(controlWriteTimeout))
		return nil
	})

	return &WebSocketClient{conn: conn}, nil
}

// The stream server pings every 30 seconds. streamReadTimeout is how long a
// read waits for a message or a ping, three of the server's ping intervals,
// before the connection counts as lost; the server likewise drops a client
// that has not answered for 90 seconds.
var streamReadTimeout = 90 * time.Second

// controlWriteTimeout is how long writing a pong or a close frame may take,
// as the websocket library allows its own.
const controlWriteTimeout = time.Second

// newStreamDialer returns the dialer of stream connections.
func newStreamDialer() *websocket.Dialer {
	return &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 10 * time.Second,
	}
}

// handshakeError is the API error of resp, the server's answer refusing the
// websocket handshake, so its status sets the exit code as for any other
// request. Its message is the "message" of a JSON body, or the status text.
func handshakeError(resp *http.Response) *api.APIError {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	apiErr := api.ParseAPIError(resp, body)
	var errorBody struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &errorBody) == nil && errorBody.Message != "" {
		apiErr.Message = errorBody.Message
	} else {
		apiErr.Message = http.StatusText(resp.StatusCode)
	}
	return apiErr
}

// ReadMessage waits for the next message of the stream. It returns
// ErrStreamClosed once the server closed the connection normally,
// ErrStreamFailed when the connection failed, errMessageTooLarge for a message
// over the size limit, which ends the stream, and ErrUndecodableMessage for a
// message it cannot read, after which the stream goes on.
func (wsc *WebSocketClient) ReadMessage() (*WebSocketMessage, error) {
	// Check if connection is already closed
	if wsc.closed.Load() {
		return nil, ErrStreamClosed
	}

	// The ping handler moves the deadline on with each ping
	if err := wsc.conn.SetReadDeadline(time.Now().Add(streamReadTimeout)); err != nil {
		wsc.closed.Store(true)
		return nil, fmt.Errorf("%w: %w", ErrStreamFailed, err)
	}
	messageType, data, err := wsc.conn.ReadMessage()
	if err != nil {
		// Mark connection as closed on any read error to prevent future panics
		wsc.closed.Store(true)

		if errors.Is(err, websocket.ErrReadLimit) {
			return nil, errMessageTooLarge
		}
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			return nil, fmt.Errorf("%w: %w", ErrStreamClosed, err)
		}
		return nil, fmt.Errorf("%w: %w", ErrStreamFailed, err)
	}

	if messageType != websocket.TextMessage {
		return nil, fmt.Errorf("%w: a binary message of %d bytes", ErrUndecodableMessage, len(data))
	}

	var msg WebSocketMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("%w: a text message that is not JSON: %w", ErrUndecodableMessage, err)
	}

	return &msg, nil
}

// Close sends the server a close frame, waiting at most
// controlWriteTimeout, and closes the connection.
func (wsc *WebSocketClient) Close() error {
	wsc.closed.Store(true)
	if wsc.conn == nil {
		return nil
	}
	_ = wsc.conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(controlWriteTimeout))
	return wsc.conn.Close()
}
