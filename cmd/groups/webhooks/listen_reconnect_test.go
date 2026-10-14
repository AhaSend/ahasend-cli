package webhooks

import (
	stderrors "errors"
	"strings"
	"testing"

	"github.com/google/uuid"
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
