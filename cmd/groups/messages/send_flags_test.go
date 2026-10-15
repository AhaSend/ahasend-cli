package messages

import (
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// sentInlineRequest runs an inline send to one recipient with args and
// returns the request it sent.
func sentInlineRequest(t *testing.T, args ...string) (requests.CreateMessageRequest, string) {
	t.Helper()
	var sent requests.CreateMessageRequest
	mockClient := &mocks.MockClient{}
	mockClient.On("SendMessageWithIdempotencyKey", mock.Anything, mock.Anything).Run(func(arguments mock.Arguments) {
		sent = arguments.Get(0).(requests.CreateMessageRequest)
	}).Return(mocks.NewMessageResponse("msg-1"), nil)
	testutil.InjectClient(t, mockClient)

	_, stderr, err := testutil.RunCommand(t, NewSendCommand(), "plain", "",
		append([]string{"--from", "sender@example.com", "--to", "user@example.com", "--subject", "Hi", "--text", "Hi"}, args...)...)
	require.NoError(t, err)
	return sent, stderr
}

func TestSendRepeatedTagFlagsAndTheDeprecatedTagsFlag(t *testing.T) {
	sent, _ := sentInlineRequest(t, "--tag", "a", "--tag", "b")
	assert.Equal(t, []string{"a", "b"}, sent.Tags)

	sent, _ = sentInlineRequest(t, "--tags", "c")
	assert.Equal(t, []string{"c"}, sent.Tags)
	tags := NewSendCommand().Flags().Lookup("tags")
	assert.True(t, tags.Hidden)
	assert.Equal(t, "use --tag instead", tags.Deprecated)
}

func TestSendSetsTheSenderNameAndTheReplyTo(t *testing.T) {
	sent, _ := sentInlineRequest(t, "--from-name", "Acme, Inc.", "--reply-to", "replies@example.com")

	assert.Equal(t, common.SenderAddress{Email: "sender@example.com", Name: ahasend.String("Acme, Inc.")}, sent.From)
	assert.Equal(t, &common.SenderAddress{Email: "replies@example.com"}, sent.ReplyTo)
}

func TestTemplateSendSetsTheSenderNameAndTheReplyTo(t *testing.T) {
	mockClient := &mocks.MockClient{}
	calls := expectTemplateSends(mockClient)

	_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain", "--template-id", sendTemplateID,
		"--to", "a@example.com", "--from", "sender@example.com", "--from-name", "Acme", "--reply-to", "replies@example.com")

	require.NoError(t, err)
	require.Len(t, calls(), 1)
	assert.Equal(t, &common.SenderAddress{Email: "sender@example.com", Name: ahasend.String("Acme")}, calls()[0].request.From)
	assert.Equal(t, &common.SenderAddress{Email: "replies@example.com"}, calls()[0].request.ReplyTo)
}

func TestSendTrimsTheValuesOfRepeatableFlags(t *testing.T) {
	mockClient := &mocks.MockClient{}
	sends := expectInlineSends(mockClient, nil)

	_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain", "--from", "sender@example.com",
		"--to", "a@example.com, b@example.com", "--subject", "Hi", "--text", "Hi", "--tag", "news, weekly")

	require.NoError(t, err)
	require.Len(t, sends(), 1)
	request := sends()[0].request
	assert.Equal(t, []common.Recipient{{Email: "a@example.com"}, {Email: "b@example.com"}}, request.Recipients)
	assert.Equal(t, []string{"news", "weekly"}, request.Tags)
}

func TestSendReadsContentFilesUnderTheirNamesAndTheDeprecatedTemplateNames(t *testing.T) {
	path := testutil.WriteTempFile(t, "body", "From a file")
	content := func(request requests.CreateMessageRequest, kind string) *string {
		return map[string]*string{"text": request.TextContent, "html": request.HtmlContent, "amp": request.AmpContent}[kind]
	}
	for _, kind := range []string{"text", "html", "amp"} {
		for _, flag := range []string{kind + "-file", kind + "-template"} {
			t.Run(flag, func(t *testing.T) {
				mockClient := &mocks.MockClient{}
				sends := expectInlineSends(mockClient, nil)

				args := []string{"--from", "sender@example.com", "--to", "user@example.com", "--subject", "Hi", "--" + flag, path}
				if kind == "amp" {
					// AMP content goes with text or HTML
					args = append(args, "--text", "Hi")
				}
				_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain", args...)
				require.NoError(t, err)
				require.Len(t, sends(), 1)
				require.NotNil(t, content(sends()[0].request, kind))
				assert.Equal(t, "From a file", *content(sends()[0].request, kind))
			})
		}
		old := NewSendCommand().Flags().Lookup(kind + "-template")
		assert.True(t, old.Hidden)
		assert.Equal(t, "use --"+kind+"-file instead", old.Deprecated)
	}
}

func TestSendScheduleAcceptsARelativeTime(t *testing.T) {
	before := time.Now()
	sent, _ := sentInlineRequest(t, "--schedule", "2h")

	require.NotNil(t, sent.Schedule)
	assert.WithinDuration(t, before.Add(2*time.Hour), *sent.Schedule.FirstAttempt, time.Minute)
}

func TestSendShowMetricsPrintsMetricsOnStderr(t *testing.T) {
	_, stderr := sentInlineRequest(t, "--show-metrics")

	assert.Contains(t, stderr, "Performance Metrics:")
	assert.Contains(t, stderr, "Successfully sent: 1")
}

func TestSendRejectsBadInputFilesAsValidationErrors(t *testing.T) {
	inline := []string{"--from", "sender@example.com", "--to", "user@example.com", "--subject", "Hi", "--text", "Hi"}
	badJSON := testutil.WriteTempFile(t, "recipients.json", "[")
	attachDir := t.TempDir()
	recipientsFile := func(name, content string) []string {
		return []string{"--from", "sender@example.com", "--subject", "Hi", "--text", "Hi",
			"--recipients", testutil.WriteTempFile(t, name, content)}
	}
	globalsFile := func(content string) []string {
		return append(inline, "--global-substitutions", testutil.WriteTempFile(t, "globals.json", content))
	}
	latin1 := testutil.WriteTempFile(t, "latin1.csv", "email,name\nuser@example.com,Jos\xe9\n")
	utf16 := testutil.WriteTempFile(t, "utf16.txt", "\xff\xfeH\x00i\x00")
	latin1Globals := testutil.WriteTempFile(t, "globals.json", "{\"city\": \"M\xfcnchen\"}")
	tests := []struct {
		args    []string
		message string
	}{
		// A file in another encoding would be sent with its characters replaced
		{[]string{"--from", "sender@example.com", "--subject", "Hi", "--text", "Hi", "--recipients", latin1},
			latin1 + " is not UTF-8 text; save it as UTF-8"},
		{[]string{"--from", "sender@example.com", "--to", "user@example.com", "--subject", "Hi", "--text-file", utf16},
			utf16 + " is UTF-16 text, not UTF-8; save it as UTF-8"},
		{append(inline, "--global-substitutions", latin1Globals), latin1Globals + " is not UTF-8 text; save it as UTF-8"},
		{[]string{"--from", "sender@example.com", "--to", "user@example.com", "--subject", "Hi", "--html-file", "/no/such.html"},
			"failed to read --html-file: " + testutil.OpenError(t, "/no/such.html")},
		// The error names a deprecated alias as the user gave it
		{[]string{"--from", "sender@example.com", "--to", "user@example.com", "--subject", "Hi", "--text-template", "/no/such.txt"},
			"failed to read --text-template: " + testutil.OpenError(t, "/no/such.txt")},
		{[]string{"--from", "sender@example.com", "--to", "user@example.com", "--subject", "Hi", "--html-template", "/no/such.html"},
			"failed to read --html-template: " + testutil.OpenError(t, "/no/such.html")},
		{append(inline, "--amp-template", "/no/such.amp"), "failed to read --amp-template: " + testutil.OpenError(t, "/no/such.amp")},
		{[]string{"--from", "sender@example.com", "--subject", "Hi", "--text", "Hi", "--recipients", "/no/such.csv"},
			"failed to read --recipients: " + testutil.OpenError(t, "/no/such.csv")},
		{[]string{"--from", "sender@example.com", "--subject", "Hi", "--text", "Hi", "--recipients", badJSON},
			"--recipients must be a JSON array"},
		{recipientsFile("recipients.json", `[]`), "--recipients file has no recipients"},
		{recipientsFile("recipients.json", `null`), "--recipients must be a JSON array"},
		{recipientsFile("recipients.json", ``), "--recipients is empty"},
		{recipientsFile("recipients.json", `{"email": "user@example.com"}`), "--recipients must be a JSON array"},
		{recipientsFile("recipients.json", `[{"email": "a@example.com"}, {"email": 5}]`), "--recipients index 1 is invalid: email must be a string"},
		{recipientsFile("recipients.json", `[null]`), "--recipients index 0 must be a JSON object"},
		// A misspelt key would drop the substitutions it holds
		{recipientsFile("recipients.json", `[{"email": "user@example.com", "substitution": {"name": "Ann"}}]`),
			`--recipients index 0 is invalid: unknown field "substitution"`},
		{recipientsFile("recipients.csv", "email\n"), "--recipients file has no recipients"},
		{recipientsFile("recipients.csv", ""), "--recipients file has no recipients"},
		{recipientsFile("recipients.json", `[{"email": "user@example.com"}] junk`),
			"--recipients must contain exactly one JSON array"},
		{globalsFile(`{"a": 1} {"b": 2}`), "--global-substitutions must contain exactly one JSON object"},
		{globalsFile(`[]`), "--global-substitutions must be a JSON object"},
		{globalsFile(`null`), "--global-substitutions must be a JSON object"},
		{append(inline, "--global-substitutions", "/no/such.json"),
			"failed to read --global-substitutions: " + testutil.OpenError(t, "/no/such.json")},
		{append(inline, "--attach", "/no/such.pdf"), "failed to read --attach: " + testutil.StatError(t, "/no/such.pdf")},
		{append(inline, "--attach", attachDir), "--attach " + attachDir + " is a directory"},
		{append(inline, "--schedule", "soon"), `invalid --schedule value "soon": use RFC 3339 or a relative time such as 30m or 2h`},
		{append(inline, "--tag", ""), "--tag cannot be empty"},
		// The error names a deprecated alias as the user gave it
		{append(inline, "--tags", ""), "--tags cannot be empty"},
		{append(inline, "--reply-to", ""), "--reply-to cannot be empty"},
		{append(inline, "--reply-to", "Replies <r@example.com>"), `invalid --reply-to value "Replies <r@example.com>": not an email address`},
		{[]string{"--template-id", sendTemplateID, "--to", "a@example.com", "--from-name", "Acme"}, "--from-name needs --from"},
		// Without --sandbox the send would deliver real mail
		{append(inline, "--sandbox-result", "bounce"), "--sandbox-result needs --sandbox"},
		{append(inline, "--sandbox=false", "--sandbox-result", "bounce"), "--sandbox-result needs --sandbox"},
		// The API refuses AMP content without text or HTML
		{[]string{"--from", "sender@example.com", "--to", "user@example.com", "--subject", "Hi", "--amp", "<p>Hi</p>"},
			contentRequiredMessage},
		{[]string{"--from", "sender@example.com", "--subject", "Hi", "--amp-file", testutil.WriteTempFile(t, "body.amp", "<p>Hi</p>"),
			"--recipients", testutil.WriteTempFile(t, "recipients.csv", "email\nuser@example.com\n")},
			contentRequiredMessage},
		{[]string{"--from", "sender@example.com", "--to", "user@example.com", "--subject", "Hi", "--amp", "<p>Hi</p>",
			"--text-file", testutil.WriteTempFile(t, "empty.txt", "")},
			contentRequiredMessage},
	}
	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", tt.args...)

			require.Error(t, err)
			assert.Equal(t, tt.message, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.False(t, *called)
		})
	}
}

// TestInlineSendWithFromNameAndNoFromFailsBeforePrompting checks that
// --from-name without --from fails on an inline send too, as on every command
// with these flags, before the command asks for a sender.
func TestInlineSendWithFromNameAndNoFromFailsBeforePrompting(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
	called := testutil.RefuseAuth(t)

	_, stderr, err := testutil.RunCommand(t, NewSendCommand(), "plain", "sender@example.com\n",
		"--from-name", "Acme", "--to", "a@example.com", "--subject", "Hi", "--text", "Hi")

	require.Error(t, err)
	assert.Equal(t, "--from-name needs --from", err.Error())
	assert.NotContains(t, stderr, "Sender email address")
	assert.False(t, *called)
}
