package routes

import (
	"testing"

	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

const routeBody = `{"object":"route","id":"11111111-1111-4111-8111-111111111111","name":"Inbound","url":"http://example.com/hook","enabled":true}`

func TestCreateWarnsAboutHTTPOnStderr(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateRoute", mock.Anything).Return(mocks.APIResponse[responses.Route](t, routeBody), nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewCreateCommand(), "json", "",
		"--name", "Inbound", "--url", "http://example.com/hook", "--recipient", "*@example.com")

	require.NoError(t, err)
	assert.Equal(t, testutil.IndentedJSON(t, routeBody), stdout)
	assert.Contains(t, stderr, "uses plain HTTP, so each email is sent unencrypted. Use HTTPS.")
}

func TestUpdateWarnsAboutHTTPOnStderr(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateRoute", mock.Anything, mock.Anything).Return(mocks.APIResponse[responses.Route](t, routeBody), nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewUpdateCommand(), "json", "",
		"11111111-1111-4111-8111-111111111111", "--url", "http://example.com/hook")

	require.NoError(t, err)
	assert.Equal(t, testutil.IndentedJSON(t, routeBody), stdout)
	assert.Contains(t, stderr, "uses plain HTTP, so each email is sent unencrypted. Use HTTPS.")
}

func TestCreatePromptsOnStderrForMissingRequiredFlags(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateRoute", mock.MatchedBy(func(req requests.CreateRouteRequest) bool {
		return req.Name == "Inbound" && req.URL == "https://example.com/hook" && req.Recipient == "*@example.com"
	})).Return(mocks.APIResponse[responses.Route](t, routeBody), nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewCreateCommand(), "json", "Inbound\nhttps://example.com/hook\n*@example.com\n")

	require.NoError(t, err)
	assert.Equal(t, testutil.IndentedJSON(t, routeBody), stdout)
	assert.Equal(t, "Route name: Webhook URL: Recipient filter (e.g., support@example.com): ", stderr)
	mockClient.AssertExpectations(t)
}

func TestCreateRejectsABadURLBeforeAuthenticating(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, _, err := testutil.RunCommand(t, NewCreateCommand(), "json", "", "--name", "Inbound", "--url", "ftp://example.com")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--url must be an HTTP or HTTPS URL with a host")
	assert.False(t, *called)
}
