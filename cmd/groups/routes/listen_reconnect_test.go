package routes

import (
	stderrors "errors"
	"testing"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
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

// errBadHandshake is the error of a stream that refuses the connection.
var errBadHandshake = stderrors.New("websocket: bad handshake (HTTP 502)")

// temporaryRouteStream installs a client whose API creates a temporary route
// with routeID for the stream of *@example.com, which then answers the
// connection with connectErr, and returns it.
func temporaryRouteStream(t *testing.T, routeID string, connectErr error) *mocks.MockClient {
	t.Helper()
	mockClient := &mocks.MockClient{}
	mockClient.On("InitiateRouteStream", (*uuid.UUID)(nil), "*@example.com").
		Return(&client.RouteStreamResponse{RouteID: routeID, WsURL: "wss://stream"}, nil)
	mockClient.On("ConnectWebSocket", "wss://stream", false).Return(nil, connectErr)
	testutil.InjectClient(t, mockClient)
	return mockClient
}

func TestListenDeletesTheTemporaryRouteWhenConnectingFails(t *testing.T) {
	mockClient := temporaryRouteStream(t, testRouteID, errBadHandshake)
	mockClient.On("DeleteRoute", uuid.MustParse(testRouteID)).Return(&common.SuccessResponse{Message: "deleted"}, nil)

	_, stderr, err := testutil.RunCommand(t, NewListenCommand(), "plain", "", "--recipient", "*@example.com")

	require.EqualError(t, err, "failed to connect to websocket: websocket: bad handshake (HTTP 502)")
	assert.Contains(t, stderr, "Deleted temporary route "+testRouteID+"\n")
	mockClient.AssertCalled(t, "DeleteRoute", uuid.MustParse(testRouteID))
}

func TestListenDeletesTheTemporaryRouteWhenTheReconnectIsDeclined(t *testing.T) {
	mockClient := temporaryRouteStream(t, testRouteID, client.ErrStreamTaken)
	mockClient.On("DeleteRoute", uuid.MustParse(testRouteID)).Return(&common.SuccessResponse{Message: "deleted"}, nil)

	_, _, err := testutil.RunCommand(t, NewListenCommand(), "plain", "n\n", "--recipient", "*@example.com")

	require.Error(t, err)
	mockClient.AssertCalled(t, "DeleteRoute", uuid.MustParse(testRouteID))
}

func TestListenWithARouteIDDeletesNothing(t *testing.T) {
	routeID := uuid.MustParse(testRouteID)
	mockClient := &mocks.MockClient{}
	mockClient.On("InitiateRouteStream", &routeID, "").Return(&client.RouteStreamResponse{RouteID: testRouteID, WsURL: "wss://stream"}, nil)
	mockClient.On("ConnectWebSocket", "wss://stream", false).Return(nil, errBadHandshake)
	testutil.InjectClient(t, mockClient)

	_, _, err := testutil.RunCommand(t, NewListenCommand(), "plain", "", "--route-id", testRouteID)

	require.Error(t, err)
	mockClient.AssertNotCalled(t, "DeleteRoute", mock.Anything)
}

// TestListenWarnsWhenTheKeyCannotDeleteTheTemporaryRoute checks that a key
// without the delete scope gets a warning that names the route and the
// command that deletes it, and that the command fails as the stream did.
func TestListenWarnsWhenTheKeyCannotDeleteTheTemporaryRoute(t *testing.T) {
	mockClient := temporaryRouteStream(t, testRouteID, errBadHandshake)
	forbidden := &api.APIError{Type: api.ErrorTypePermission, StatusCode: 403, Message: "the API key lacks the routes:delete:all scope"}
	mockClient.On("DeleteRoute", uuid.MustParse(testRouteID)).Return(nil, forbidden)

	_, stderr, err := testutil.RunCommand(t, NewListenCommand(), "plain", "", "--recipient", "*@example.com")

	require.EqualError(t, err, "failed to connect to websocket: websocket: bad handshake (HTTP 502)")
	assert.Contains(t, stderr, "Warning: temporary route "+testRouteID+" may remain: "+forbidden.Error()+
		"; delete it with 'ahasend routes delete "+testRouteID+"'\n")
}

func TestListenWarnsOfATemporaryRouteIDThatIsNotAUUID(t *testing.T) {
	mockClient := temporaryRouteStream(t, "not-a-uuid", errBadHandshake)

	_, stderr, err := testutil.RunCommand(t, NewListenCommand(), "plain", "", "--recipient", "*@example.com")

	require.Error(t, err)
	assert.Contains(t, stderr, "Warning: temporary route not-a-uuid may remain: the API returned no valid route ID; "+
		"delete it with 'ahasend routes delete not-a-uuid'\n")
	mockClient.AssertNotCalled(t, "DeleteRoute", mock.Anything)
}
