package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
)

func TestInitiateRouteStream_Success(t *testing.T) {
	routeID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	tests := []struct {
		name        string
		routeID     *uuid.UUID
		recipient   string
		wantRouteID string
		wantQuery   map[string]string
	}{
		{
			name:        "with route ID",
			routeID:     &routeID,
			recipient:   "",
			wantRouteID: routeID.String(),
			wantQuery:   map[string]string{"route_id": routeID.String()},
		},
		{
			name:        "with recipient pattern",
			routeID:     nil,
			recipient:   "*@example.com",
			wantRouteID: "temp-route-456",
			wantQuery:   map[string]string{"recipient": "*@example.com"},
		},
		{
			name:        "with complex recipient pattern",
			routeID:     nil,
			recipient:   "support-*@company.com",
			wantRouteID: "temp-route-789",
			wantQuery:   map[string]string{"recipient": "support-*@company.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and path
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/routes/stream")

				// Verify authorization header
				assert.NotEmpty(t, r.Header.Get("Authorization"))
				assert.Contains(t, r.Header.Get("Authorization"), "Bearer")

				// Verify query parameters
				for key, value := range tt.wantQuery {
					assert.Equal(t, value, r.URL.Query().Get(key))
				}

				// Send mock response
				response := RouteStreamResponse{
					RouteID: tt.wantRouteID,
					WsURL:   fmt.Sprintf("wss://ws.example.com/routes/%s", tt.wantRouteID),
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			// Create client with test server
			client := createTestClient(t, server.URL)

			// Call InitiateRouteStream
			resp, err := client.InitiateRouteStream(tt.routeID, tt.recipient)

			// Verify response
			require.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tt.wantRouteID, resp.RouteID)
			assert.Contains(t, resp.WsURL, "wss://")
		})
	}
}

// TestInitiateStream_APIErrorKeepsStatusAndBody checks that a failed stream
// request returns the API's error with its HTTP status and body, so the
// command exits by that status and JSON output prints the body.
func TestInitiateStream_APIErrorKeepsStatusAndBody(t *testing.T) {
	calls := map[string]func(*Client) error{
		"route": func(c *Client) error {
			_, err := c.InitiateRouteStream(nil, "*@example.com")
			return err
		},
		"webhook": func(c *Client) error {
			_, err := c.InitiateWebhookStream(nil)
			return err
		},
	}
	statuses := []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError}

	for name, call := range calls {
		for _, status := range statuses {
			t.Run(fmt.Sprintf("%s %d", name, status), func(t *testing.T) {
				body := `{"message":"stream refused"}`
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(status)
					_, _ = w.Write([]byte(body))
				}))
				defer server.Close()

				err := call(createTestClient(t, server.URL))

				var apiErr *api.APIError
				require.ErrorAs(t, err, &apiErr)
				assert.Equal(t, status, apiErr.StatusCode)
				assert.Equal(t, "stream refused", apiErr.Message)
				assert.JSONEq(t, body, string(apiErr.Raw))
			})
		}
	}
}

func TestInitiateWebhookStream_Success(t *testing.T) {
	webhookID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	tests := []struct {
		name          string
		webhookID     *uuid.UUID
		wantWebhookID string
	}{
		{
			name:          "with webhook ID",
			webhookID:     &webhookID,
			wantWebhookID: webhookID.String(),
		},
		{
			name:          "without webhook ID (creates temporary)",
			webhookID:     nil,
			wantWebhookID: "temp-webhook-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and path
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/webhooks/stream")

				// webhook_id is sent only when a webhook ID is given
				_, sent := r.URL.Query()["webhook_id"]
				assert.Equal(t, tt.webhookID != nil, sent)
				if tt.webhookID != nil {
					assert.Equal(t, tt.webhookID.String(), r.URL.Query().Get("webhook_id"))
				}

				// Send mock response
				response := WebhookStreamResponse{
					WebhookID: tt.wantWebhookID,
					WsURL:     fmt.Sprintf("wss://ws.example.com/webhooks/%s", tt.wantWebhookID),
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			// Create client with test server
			client := createTestClient(t, server.URL)

			// Call InitiateWebhookStream
			resp, err := client.InitiateWebhookStream(tt.webhookID)

			// Verify response
			require.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tt.wantWebhookID, resp.WebhookID)
			assert.Contains(t, resp.WsURL, "wss://")
		})
	}
}

func TestInitiateRouteStream_JSONDecodeError(t *testing.T) {
	// Create test server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	// Create client with test server
	client := createTestClient(t, server.URL)

	// Call InitiateRouteStream
	resp, err := client.InitiateRouteStream(nil, "*@example.com")

	// Verify error
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, "could not read the API response: the response is not JSON", err.Error())
}

// Helper function to create a test client with custom server URL
func createTestClient(t *testing.T, serverURL string) *Client {
	t.Helper()
	apiKey := "test-api-key"
	accountID := uuid.New().String()

	// Create SDK config with test server URL
	config := api.NewConfiguration()
	config.Host = serverURL[7:] // Remove http:// prefix
	config.Scheme = "http"
	config.UserAgent = "test-client"
	config.HTTPClient = http.DefaultClient
	config.RetryConfig = api.RetryConfig{
		Enabled: false,
	}

	// Create auth context
	auth := context.WithValue(context.Background(), api.ContextAccessToken, apiKey)

	// Create our wrapper client
	client := &Client{
		APIClient: api.NewAPIClientWithConfig(config),
		auth:      auth,
		accountID: accountID,
		config:    config,
	}

	return client
}

func TestConnectWebSocketReportsATakenStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient("test-api-key", "11111111-1111-4111-8111-111111111111")
	require.NoError(t, err)

	_, err = client.ConnectWebSocket("ws"+strings.TrimPrefix(server.URL, "http"), false)

	assert.ErrorIs(t, err, ErrStreamTaken)
}

// TestConnectWebSocketSendsTheKeyOnlyOverTLS checks that a stream URL that
// would carry the API key in plain text is refused before connecting.
func TestConnectWebSocketSendsTheKeyOnlyOverTLS(t *testing.T) {
	client, err := NewClient("test-api-key", "11111111-1111-4111-8111-111111111111")
	require.NoError(t, err)

	for _, wsURL := range []string{"ws://stream.example.com/s", "http://stream.example.com/s", "ws://10.0.0.1:8080/s"} {
		t.Run(wsURL, func(t *testing.T) {
			_, err := client.ConnectWebSocket(wsURL, false)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "the stream URL must use wss")
			assert.NotContains(t, err.Error(), "test-api-key")
		})
	}
}

// TestWebSocketClientCanBeClosedWhileAReadWaits checks that Close, called
// from another goroutine while ReadMessage waits for a message, ends the
// read with ErrStreamFailed and is free of data races.
func TestWebSocketClientCanBeClosedWhileAReadWaits(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		<-release
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })
	client, err := NewClient("test-api-key", "11111111-1111-4111-8111-111111111111")
	require.NoError(t, err)
	wsClient, err := client.ConnectWebSocket("ws"+strings.TrimPrefix(server.URL, "http"), false)
	require.NoError(t, err)

	readErr := make(chan error, 1)
	go func() {
		_, err := wsClient.ReadMessage()
		readErr <- err
	}()
	time.Sleep(20 * time.Millisecond)
	require.NoError(t, wsClient.Close())

	select {
	case err := <-readErr:
		assert.ErrorIs(t, err, ErrStreamFailed)
	case <-time.After(5 * time.Second):
		t.Fatal("the read did not end")
	}
	_, err = wsClient.ReadMessage()
	assert.ErrorIs(t, err, ErrStreamClosed)
}

// TestConnectWebSocketReportsARefusedHandshakeByItsStatus checks that a
// handshake the server refuses fails with the API error of its status, which
// sets the exit code, and names the status and the server's message.
func TestConnectWebSocketReportsARefusedHandshakeByItsStatus(t *testing.T) {
	tests := map[int]int{
		http.StatusUnauthorized:        2,
		http.StatusForbidden:           6,
		http.StatusNotFound:            5,
		http.StatusInternalServerError: 1,
	}
	for status, code := range tests {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"message":"stream refused"}`))
			}))
			t.Cleanup(server.Close)
			client, err := NewClient("test-api-key", "11111111-1111-4111-8111-111111111111")
			require.NoError(t, err)

			_, err = client.ConnectWebSocket("ws"+strings.TrimPrefix(server.URL, "http"), false)

			require.Error(t, err)
			assert.Equal(t, code, clierrors.GetExitCode(err), err.Error())
			assert.True(t, strings.HasPrefix(err.Error(), "the stream refused the connection: "), err.Error())
			assert.Contains(t, err.Error(), fmt.Sprintf(" error (HTTP %d): stream refused", status))
			assert.NotContains(t, err.Error(), "test-api-key")
		})
	}
}

// TestPathsTheCLIBuildsUseTheCanonicalAccountID checks that the trigger and
// stream requests, whose paths the CLI builds, name the account by its
// canonical UUID whatever form of it the profile holds.
func TestPathsTheCLIBuildsUseTheCanonicalAccountID(t *testing.T) {
	const accountID = "11111111-1111-4111-8111-111111111111"
	id := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"ok"}`))
	}))
	t.Cleanup(server.Close)

	for _, form := range []string{"{" + accountID + "}", "urn:uuid:" + accountID, strings.ToUpper(accountID)} {
		t.Run(form, func(t *testing.T) {
			paths = nil
			client, err := NewClient("test-api-key", form, server.URL)
			require.NoError(t, err)

			_, err = client.TriggerWebhook(id, []string{"message.opened"})
			require.NoError(t, err)
			_, err = client.TriggerRoute(id)
			require.NoError(t, err)
			_, err = client.InitiateWebhookStream(nil)
			require.NoError(t, err)
			_, err = client.InitiateRouteStream(nil, "*@example.com")
			require.NoError(t, err)

			assert.Equal(t, []string{
				"/v2/accounts/" + accountID + "/webhooks/" + id.String() + "/trigger",
				"/v2/accounts/" + accountID + "/routes/" + id.String() + "/trigger",
				"/v2/accounts/" + accountID + "/webhooks/stream",
				"/v2/accounts/" + accountID + "/routes/stream",
			}, paths)
		})
	}
}

// TestConnectWebSocketSendsTheKeyInTheAuthorizationHeader checks that the
// API key goes in the Authorization header and never in the URL, which
// proxies and servers log.
func TestConnectWebSocketSendsTheKeyInTheAuthorizationHeader(t *testing.T) {
	requests := make(chan *http.Request, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		_ = conn.Close()
	}))
	t.Cleanup(server.Close)
	client, err := NewClient("test-api-key", "11111111-1111-4111-8111-111111111111")
	require.NoError(t, err)

	for _, force := range []bool{false, true} {
		wsClient, err := client.ConnectWebSocket("ws"+strings.TrimPrefix(server.URL, "http")+"/s?session=1", force)
		require.NoError(t, err)
		require.NoError(t, wsClient.Close())

		r := <-requests
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		assert.Equal(t, client.config.UserAgent, r.Header.Get("User-Agent"), "the stream is asked for as the API is")
		assert.NotContains(t, r.URL.RawQuery, "test-api-key")
		assert.Equal(t, "1", r.URL.Query().Get("session"))
		if force {
			assert.Equal(t, "true", r.URL.Query().Get("force_reconnect"))
		} else {
			assert.False(t, r.URL.Query().Has("force_reconnect"))
		}
	}
}

// TestConnectWebSocketFailsWithoutAStreamURL checks that a stream response
// without a URL fails with a message that says so.
func TestConnectWebSocketFailsWithoutAStreamURL(t *testing.T) {
	client, err := NewClient("test-api-key", "11111111-1111-4111-8111-111111111111")
	require.NoError(t, err)

	_, err = client.ConnectWebSocket("", false)

	require.EqualError(t, err, "the API returned no stream URL")
	assert.Equal(t, 1, clierrors.GetExitCode(err))
}

// TestStreamDialerUsesTheEnvironmentProxy checks that stream connections go
// through the proxy the environment sets.
func TestStreamDialerUsesTheEnvironmentProxy(t *testing.T) {
	assert.NotNil(t, newStreamDialer().Proxy)
}

// TestReadMessageEndsTheStreamOnAMessageOverTheLimit checks that a message
// larger than the read limit ends the stream with errMessageTooLarge.
func TestReadMessageEndsTheStreamOnAMessageOverTheLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, make([]byte, streamReadLimit+1))
		_, _, _ = conn.ReadMessage()
	}))
	t.Cleanup(server.Close)
	client, err := NewClient("test-api-key", "11111111-1111-4111-8111-111111111111")
	require.NoError(t, err)
	wsClient, err := client.ConnectWebSocket("ws"+strings.TrimPrefix(server.URL, "http"), false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = wsClient.Close() })

	_, err = wsClient.ReadMessage()

	require.ErrorIs(t, err, errMessageTooLarge)
	assert.EqualError(t, err, "a stream message is larger than 16 MiB")
	_, err = wsClient.ReadMessage()
	assert.ErrorIs(t, err, ErrStreamClosed)
}

// TestCLIPostsSendJSONOnlyWithABody checks the Content-Type of the requests
// the CLI builds: application/json for a trigger, which sends a JSON body,
// and none for a stream request, which sends no body.
func TestCLIPostsSendJSONOnlyWithABody(t *testing.T) {
	const accountID = "11111111-1111-4111-8111-111111111111"
	id := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	contentTypes := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentTypes[strings.TrimPrefix(r.URL.Path, "/v2/accounts/"+accountID+"/")] = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"ok"}`))
	}))
	t.Cleanup(server.Close)
	client, err := NewClient("test-api-key", accountID, server.URL)
	require.NoError(t, err)

	_, err = client.TriggerWebhook(id, []string{"message.opened"})
	require.NoError(t, err)
	_, err = client.TriggerRoute(id)
	require.NoError(t, err)
	_, err = client.InitiateWebhookStream(nil)
	require.NoError(t, err)
	_, err = client.InitiateRouteStream(nil, "*@example.com")
	require.NoError(t, err)

	assert.Equal(t, map[string]string{
		"webhooks/" + id.String() + "/trigger": "application/json",
		"routes/" + id.String() + "/trigger":   "application/json",
		"webhooks/stream":                      "",
		"routes/stream":                        "",
	}, contentTypes)
}

// streamTestServer serves a websocket that handle drives and returns its URL
// and a client connected to it.
func streamTestServer(t *testing.T, handle func(conn *websocket.Conn)) *WebSocketClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		handle(conn)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient("test-api-key", "11111111-1111-4111-8111-111111111111")
	require.NoError(t, err)
	wsClient, err := client.ConnectWebSocket("ws"+strings.TrimPrefix(server.URL, "http"), false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = wsClient.Close() })
	return wsClient
}

// closeCode returns the code of the close frame err reports, or -1 when err
// is not a close frame, such as a connection dropped without one.
func closeCode(err error) int {
	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) {
		return closeErr.Code
	}
	return -1
}

func TestWebSocketClientAnswersTheServersCloseFrame(t *testing.T) {
	answer := make(chan error, 1)
	wsClient := streamTestServer(t, func(conn *websocket.Conn) {
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, "restart"))
		_, _, err := conn.ReadMessage()
		answer <- err
	})

	_, err := wsClient.ReadMessage()

	require.ErrorIs(t, err, ErrStreamClosed)
	select {
	case err := <-answer:
		assert.Equal(t, websocket.CloseGoingAway, closeCode(err), "%v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("the server got no answer")
	}
}

func TestWebSocketClientCloseSendsACloseFrame(t *testing.T) {
	closed := make(chan error, 1)
	wsClient := streamTestServer(t, func(conn *websocket.Conn) {
		_, _, err := conn.ReadMessage()
		closed <- err
	})

	require.NoError(t, wsClient.Close())

	select {
	case err := <-closed:
		assert.Equal(t, websocket.CloseNormalClosure, closeCode(err), "%v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("the server saw no close")
	}
}

func TestWebSocketClientAnswersAPingWithAPong(t *testing.T) {
	pong := make(chan string, 1)
	errPong := errors.New("pong received")
	wsClient := streamTestServer(t, func(conn *websocket.Conn) {
		conn.SetPongHandler(func(data string) error {
			pong <- data
			return errPong
		})
		_ = conn.WriteControl(websocket.PingMessage, []byte("p1"), time.Now().Add(time.Second))
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, _, _ = conn.ReadMessage()
	})
	go func() { _, _ = wsClient.ReadMessage() }()

	select {
	case data := <-pong:
		assert.Equal(t, "p1", data)
	case <-time.After(5 * time.Second):
		t.Fatal("the ping got no pong")
	}
}

// shortenStreamReadTimeout makes a read wait at most timeout for a message
// or a ping.
func shortenStreamReadTimeout(t *testing.T, timeout time.Duration) {
	previous := streamReadTimeout
	streamReadTimeout = timeout
	t.Cleanup(func() { streamReadTimeout = previous })
}

func TestReadMessageFailsWhenTheServerSendsNothingForTooLong(t *testing.T) {
	shortenStreamReadTimeout(t, 200*time.Millisecond)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	wsClient := streamTestServer(t, func(conn *websocket.Conn) { <-release })

	readErr := make(chan error, 1)
	go func() {
		_, err := wsClient.ReadMessage()
		readErr <- err
	}()

	select {
	case err := <-readErr:
		require.ErrorIs(t, err, ErrStreamFailed)
		assert.Contains(t, err.Error(), "timeout")
	case <-time.After(5 * time.Second):
		t.Fatal("the read still waits")
	}
}

func TestPingsKeepAQuietStreamOpen(t *testing.T) {
	shortenStreamReadTimeout(t, 300*time.Millisecond)
	wsClient := streamTestServer(t, func(conn *websocket.Conn) {
		// Answers to the pings are read by no one here; the client's pongs
		// only need to be written
		for range 8 {
			_ = conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second))
			time.Sleep(100 * time.Millisecond)
		}
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"connected","session_id":"s1"}`))
		_, _, _ = conn.ReadMessage()
	})

	msg, err := wsClient.ReadMessage()

	require.NoError(t, err)
	assert.Equal(t, "s1", msg.SessionID)
}
