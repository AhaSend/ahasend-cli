package routes

import (
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
	routeID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	mockClient := &mocks.MockClient{}
	mockClient.On("InitiateRouteStream", &routeID, "").Return(&client.RouteStreamResponse{WsURL: "wss://stream"}, nil)
	mockClient.On("ConnectWebSocket", "wss://stream", false).
		Return(nil, client.ErrStreamTaken)
	testutil.InjectClient(t, mockClient)

	_, stderr, err := testutil.RunCommand(t, NewListenCommand(), "plain", "n\n", "--route-id", routeID.String())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "operation cancelled: another connection exists")
	assert.Equal(t, 4, errors.GetExitCode(err))
	assert.Contains(t, stderr, "Another connection exists for this route.")
	assert.Contains(t, stderr, "Do you want to force reconnect? (y/N): ")
	mockClient.AssertNotCalled(t, "ConnectWebSocket", mock.Anything, true)
}
