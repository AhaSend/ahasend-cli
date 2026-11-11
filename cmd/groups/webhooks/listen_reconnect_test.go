package webhooks

import (
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

func TestListenDecliningTheReconnectPromptExits4(t *testing.T) {
	webhookID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	mockClient := &mocks.MockClient{}
	mockClient.On("InitiateWebhookStream", &webhookID).Return(&client.WebhookStreamResponse{WsURL: "wss://stream"}, nil)
	mockClient.On("ConnectWebSocket", "wss://stream", false).
		Return(nil, client.ErrStreamTaken)
	testutil.InjectClient(t, mockClient)

	_, stderr, err := testutil.RunCommand(t, NewListenCommand(), "plain", "n\n", "--webhook-id", webhookID.String())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "operation cancelled: another connection exists")
	assert.Equal(t, 4, errors.GetExitCode(err))
	assert.Contains(t, stderr, "Another connection exists for this webhook.")
	assert.Contains(t, stderr, "Do you want to force reconnect? (y/N): ")
	mockClient.AssertNotCalled(t, "ConnectWebSocket", mock.Anything, true)
}

// TestListenNamesTheTemporaryWebhookWhenConnectingFails checks that the note
// on the temporary webhook, which stays after the command, is printed even
// when connecting to its stream fails.
func TestListenNamesTheTemporaryWebhookWhenConnectingFails(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("InitiateWebhookStream", (*uuid.UUID)(nil)).
		Return(&client.WebhookStreamResponse{WebhookID: "11111111-1111-4111-8111-111111111111", WsURL: "wss://stream"}, nil)
	mockClient.On("ConnectWebSocket", "wss://stream", false).
		Return(nil, stderrors.New("websocket: bad handshake (HTTP 502)"))
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewListenCommand(), "plain", "")

	require.EqualError(t, err, "failed to connect to websocket: websocket: bad handshake (HTTP 502)")
	assert.True(t, strings.HasPrefix(stderr, "Note: temporary webhook 11111111-1111-4111-8111-111111111111 stays after the listener stops; "+
		"delete it with 'ahasend webhooks delete 11111111-1111-4111-8111-111111111111'\n"), stderr)
	assert.Empty(t, stdout)
}

func TestListenWithAWebhookIDPrintsNoNote(t *testing.T) {
	webhookID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	mockClient := &mocks.MockClient{}
	mockClient.On("InitiateWebhookStream", &webhookID).Return(&client.WebhookStreamResponse{WsURL: "wss://stream"}, nil)
	mockClient.On("ConnectWebSocket", "wss://stream", false).Return(nil, stderrors.New("refused"))
	testutil.InjectClient(t, mockClient)

	_, stderr, err := testutil.RunCommand(t, NewListenCommand(), "plain", "", "--webhook-id", webhookID.String())

	require.Error(t, err)
	assert.NotContains(t, stderr, "Note:")
}

// listenSlim runs webhooks listen with --slim-output and args on a stream
// that sends messages, then closes normally, and returns its stdout.
func listenSlim(t *testing.T, messages []string, args ...string) string {
	t.Helper()
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for _, message := range messages {
			_ = conn.WriteMessage(websocket.TextMessage, []byte(message))
		}
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_, _, _ = conn.ReadMessage()
	}))
	t.Cleanup(server.Close)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	streamClient, err := client.NewClient("test-key", testWebhookID, "http://127.0.0.1:1")
	require.NoError(t, err)
	stream, err := streamClient.ConnectWebSocket(wsURL, false)
	require.NoError(t, err)

	webhookID := uuid.MustParse(testWebhookID)
	mockClient := &mocks.MockClient{}
	mockClient.On("InitiateWebhookStream", &webhookID).Return(&client.WebhookStreamResponse{WebhookID: testWebhookID, WsURL: wsURL}, nil)
	mockClient.On("ConnectWebSocket", wsURL, false).Return(stream, nil)
	testutil.InjectClient(t, mockClient)

	stdout, _, err := testutil.RunCommand(t, NewListenCommand(), "plain", "",
		append([]string{"--webhook-id", testWebhookID, "--slim-output"}, args...)...)
	require.NoError(t, err)
	return stdout
}

// TestListenTakesAContactEventFilter checks that --events takes a contact
// event, so listen shows that event and drops a message event.
func TestListenTakesAContactEventFilter(t *testing.T) {
	stdout := listenSlim(t, []string{
		`{"type":"event","timestamp":0,"event":{"data":{"type":"message.delivered","recipient":"a@example.com"}}}`,
		`{"type":"event","timestamp":0,"event":{"data":{"type":"contact.created","data":{"email":"b@example.com"}}}}`,
	}, "--events", "contact.created")

	assert.Contains(t, stdout, "] contact.created\n")
	assert.NotContains(t, stdout, "message.delivered")
}

// TestListenFiltersBySendingTypeChanged checks that --events takes the
// sending type event by its Name and by its Type, so listen shows that event
// and drops the other domain event and a message event.
func TestListenFiltersBySendingTypeChanged(t *testing.T) {
	messages := []string{
		`{"type":"event","timestamp":0,"event":{"data":{"type":"message.delivered","recipient":"a@example.com"}}}`,
		`{"type":"event","timestamp":0,"event":{"data":{"type":"domain.dns_error","data":{"domain":"example.com"}}}}`,
		`{"type":"event","timestamp":0,"event":{"data":{"type":"domain.sending_type_changed","data":{"domain":"example.com",` +
			`"previous_sending_type":"transactional","sending_type":"marketing","source":"automatic"}}}}`,
	}
	for _, value := range []string{"sending_type_changed", "domain.sending_type_changed"} {
		t.Run(value, func(t *testing.T) {
			stdout := listenSlim(t, messages, "--events", value)

			assert.Contains(t, stdout, "] domain.sending_type_changed\n")
			assert.NotContains(t, stdout, "domain.dns_error")
			assert.NotContains(t, stdout, "message.delivered")
		})
	}
}
