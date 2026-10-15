package webhooks

import (
	"testing"

	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

const webhookBody = `{"object":"webhook","id":"11111111-1111-4111-8111-111111111111","name":"Events","url":"http://example.com/hook","enabled":true}`

func TestCreateWarnsAboutHTTPOnStderr(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateWebhook", mock.Anything).Return(mocks.APIResponse[responses.Webhook](t, webhookBody), nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewCreateCommand(), "json", "",
		"--name", "Events", "--url", "http://example.com/hook", "--scope", "global", "--events", "delivered")

	require.NoError(t, err)
	assert.Equal(t, testutil.IndentedJSON(t, webhookBody), stdout)
	assert.Contains(t, stderr, "uses plain HTTP, so each event is sent unencrypted. Use HTTPS.")
}

func TestUpdateWarnsAboutHTTPOnStderr(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateWebhook", mock.Anything, mock.Anything).Return(mocks.APIResponse[responses.Webhook](t, webhookBody), nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewUpdateCommand(), "json", "",
		"11111111-1111-4111-8111-111111111111", "--url", "http://example.com/hook")

	require.NoError(t, err)
	assert.Equal(t, testutil.IndentedJSON(t, webhookBody), stdout)
	assert.Contains(t, stderr, "uses plain HTTP, so each event is sent unencrypted. Use HTTPS.")
}

// eventsPrompt is the question webhooks create asks for the events.
const eventsPrompt = "Events to send (reception, delivered, transient_error, failed, bounced, suppressed, opened, " +
	"clicked, suppression_created, dns_error), separated by commas, or all: "

// TestCreateRefusesAWrongEventsAnswerAsAnAnswer checks that an answer to the
// events prompt that names something other than an event, or an event
// twice, is refused in words for the prompt, not for --events.
func TestCreateRefusesAWrongEventsAnswerAsAnAnswer(t *testing.T) {
	tests := map[string]string{
		"delivered, sent\n":              `"sent" is not an event; answer with events of the list, separated by commas, or all`,
		"delivered, message.delivered\n": `"message.delivered" names the same event as "delivered"; answer with each event once`,
	}
	for answer, message := range tests {
		t.Run(answer, func(t *testing.T) {
			t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
			authCalled := testutil.RefuseAuth(t)

			_, _, err := testutil.RunCommand(t, NewCreateCommand(), "plain", "Events\nhttps://example.com/hook\n"+answer)

			require.EqualError(t, err, message)
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.False(t, *authCalled)
		})
	}
}

func TestCreatePromptsOnStderrForMissingRequiredFlags(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateWebhook", mock.MatchedBy(func(req requests.CreateWebhookRequest) bool {
		return req.Name == "Events" && req.URL == "https://example.com/hook" && req.Scope == "global" &&
			req.OnDelivered && req.OnBounced && !req.OnOpened
	})).Return(mocks.APIResponse[responses.Webhook](t, webhookBody), nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewCreateCommand(), "json", "Events\nhttps://example.com/hook\ndelivered, message.bounced\n")

	require.NoError(t, err)
	assert.Equal(t, testutil.IndentedJSON(t, webhookBody), stdout)
	assert.Equal(t, "Webhook name: Webhook URL: "+eventsPrompt, stderr)
	mockClient.AssertExpectations(t)
}

// TestCreateAsksForTheEventsOfAWebhook checks that, on a terminal, a webhook
// created without --events or --all-events gets the events asked for, all of
// them for "all", and that a blank answer fails before any request, since a
// webhook without events receives none.
func TestCreateAsksForTheEventsOfAWebhook(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
	var sent requests.CreateWebhookRequest
	mockClient := &mocks.MockClient{}
	testutil.CaptureRequest(mockClient, "CreateWebhook", &sent, mocks.APIResponse[responses.Webhook](t, webhookBody))
	testutil.InjectClient(t, mockClient)

	_, _, err := testutil.RunCommand(t, NewCreateCommand(), "json", "All\n", "--name", "Events", "--url", "https://example.com/hook")

	require.NoError(t, err)
	assert.True(t, sent.OnReception && sent.OnDelivered && sent.OnDnsError, "%+v", sent)

	called := testutil.RefuseAuth(t)
	_, _, err = testutil.RunCommand(t, NewCreateCommand(), "json", " , \n", "--name", "Events", "--url", "https://example.com/hook")

	require.Error(t, err)
	assert.Equal(t, "one of --events or --all-events is required: a webhook without events receives none", err.Error())
	assert.Equal(t, 4, clierrors.GetExitCode(err))
	assert.False(t, *called)
}

func TestCreateRejectsABadURLBeforeAuthenticating(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, _, err := testutil.RunCommand(t, NewCreateCommand(), "json", "", "--name", "Events", "--url", "ftp://example.com")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--url must be an HTTP or HTTPS URL with a host")
	assert.False(t, *called)
}
