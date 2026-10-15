package messages

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

const sendResponseBody = `{"object":"list","data":[{"object":"message","id":"11111111-1111-4111-8111-111111111111","recipient":{"email":"user@example.com"},"status":"queued"}]}`

func TestSendPromptsOnStderrAndPrintsOnlyTheResponse(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
	mockClient := &mocks.MockClient{}
	mockClient.On("SendMessageWithIdempotencyKey", mock.MatchedBy(func(request requests.CreateMessageRequest) bool {
		return request.From.Email == "sender@example.com" && request.Subject == "Hello" &&
			len(request.Recipients) == 1 && request.Recipients[0].Email == "user@example.com" &&
			request.TextContent != nil && *request.TextContent == "Hi there"
	}), mock.Anything).Return(mocks.APIResponse[responses.CreateMessageResponse](t, sendResponseBody), nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewSendCommand(), "json",
		"sender@example.com\nuser@example.com\nHello\nHi there\n")

	require.NoError(t, err)
	assert.Equal(t, testutil.IndentedJSON(t, sendResponseBody), stdout)
	assert.Equal(t, "Sender email address: Recipient email address: "+
		"Email subject: Email content (plain text): ", stderr)
	mockClient.AssertExpectations(t)
}

func TestSendWithOnlyAMPContentAsksForTheText(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
	mockClient := &mocks.MockClient{}
	sends := expectInlineSends(mockClient, nil)
	testutil.InjectClient(t, mockClient)

	_, stderr, err := testutil.RunCommand(t, NewSendCommand(), "plain", "Hi there\n",
		"--from", "sender@example.com", "--to", "user@example.com", "--subject", "Hello", "--amp", "<p>Hi</p>")

	require.NoError(t, err)
	assert.Equal(t, "Email content (plain text): ", stderr)
	require.Len(t, sends(), 1)
	assert.Equal(t, "Hi there", *sends()[0].request.TextContent)
	assert.Equal(t, "<p>Hi</p>", *sends()[0].request.AmpContent)
}

func TestSendPromptsBeforeAuthenticating(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
	called := testutil.RefuseAuth(t)

	_, _, err := testutil.RunCommand(t, NewSendCommand(), "json", "not-an-email\nuser@example.com\nHello\nHi\n")

	require.Error(t, err)
	assert.False(t, *called)
}

func TestSendWithoutATerminalReportsTheMissingFlags(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{nil, "--from is required"},
		{[]string{"--from", "sender@example.com"}, "one of --to or --recipients must be provided"},
		{[]string{"--from", "sender@example.com", "--to", "user@example.com"}, "--subject is required"},
		{[]string{"--from", "sender@example.com", "--subject", "Hi", "--to", "user@example.com"},
			"one of --text, --html, --text-file or --html-file must be provided; --amp and --amp-file only add an AMP part"},
	}
	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			_, stderr, err := testutil.RunCommand(t, NewSendCommand(), "plain", "sender@example.com\n", tt.args...)

			require.Error(t, err)
			assert.Equal(t, tt.message, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.NotContains(t, stderr, "address:")
			assert.False(t, *called)
		})
	}
}

func TestSendNonInteractiveDoesNotPromptOnATerminal(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
	testutil.RefuseAuth(t)

	_, stderr, err := testutil.RunCommand(t, NewSendCommand(), "plain", "sender@example.com\n", "--non-interactive")

	require.Error(t, err)
	assert.Equal(t, "--from is required", err.Error())
	assert.NotContains(t, stderr, "Sender email address:")
}
