package listen

import (
	stderrors "errors"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestConnectStream_ConnectsWithoutAPrompt(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("ConnectWebSocket", "wss://stream", false).Return(&client.WebSocketClient{}, nil)
	cmd, _, stderr := confirmCommand(t, "")

	_, err := connectStream(cmd, mockClient, "wss://stream", "webhook")

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
}

func TestConnectStream_OtherErrorsPassThrough(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("ConnectWebSocket", "wss://stream", false).Return(nil, stderrors.New("dial failed"))
	cmd, _, _ := confirmCommand(t, "y\n")

	_, err := connectStream(cmd, mockClient, "wss://stream", "webhook")

	assert.EqualError(t, err, "dial failed")
	mockClient.AssertNotCalled(t, "ConnectWebSocket", mock.Anything, true)
}

func TestConnectStream_ForcesAReconnectWhenConfirmed(t *testing.T) {
	for _, answer := range []string{"y", "yes", "YES"} {
		t.Run(answer, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("ConnectWebSocket", "wss://stream", false).Return(nil, client.ErrStreamTaken)
			mockClient.On("ConnectWebSocket", "wss://stream", true).Return(&client.WebSocketClient{}, nil)
			cmd, stdout, stderr := confirmCommand(t, answer+"\n")

			_, err := connectStream(cmd, mockClient, "wss://stream", "route")

			require.NoError(t, err)
			assert.Equal(t, "\nAnother connection exists for this route.\nDo you want to force reconnect? (y/N): \n", stderr.String())
			assert.Empty(t, stdout.String())
			mockClient.AssertExpectations(t)
		})
	}
}

// TestConnectStream_ColorsStderrOnlyWhenItIsATerminal checks that the
// warning on stderr is not colored when stderr is not a terminal, even when
// stdout is one.
func TestConnectStream_ColorsStderrOnlyWhenItIsATerminal(t *testing.T) {
	previous := color.NoColor
	color.NoColor = false // as with stdout a terminal
	t.Cleanup(func() { color.NoColor = previous })
	mockClient := &mocks.MockClient{}
	mockClient.On("ConnectWebSocket", "wss://stream", false).Return(nil, client.ErrStreamTaken)
	mockClient.On("ConnectWebSocket", "wss://stream", true).Return(&client.WebSocketClient{}, nil)
	cmd, _, stderr := confirmCommand(t, "y\n")

	_, err := connectStream(cmd, mockClient, "wss://stream", "route")

	require.NoError(t, err)
	assert.NotContains(t, stderr.String(), "\x1b[")
}

func TestConnectStream_DecliningIsAValidationError(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("ConnectWebSocket", "wss://stream", false).Return(nil, client.ErrStreamTaken)
	cmd, _, _ := confirmCommand(t, "no\n")

	_, err := connectStream(cmd, mockClient, "wss://stream", "webhook")

	assertValidationError(t, err, "operation cancelled: another connection exists")
	mockClient.AssertNotCalled(t, "ConnectWebSocket", mock.Anything, true)
}
