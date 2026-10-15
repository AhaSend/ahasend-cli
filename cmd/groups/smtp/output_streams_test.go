package smtp

import (
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

func TestCreatePromptsOnStderrForAMissingName(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
	const body = `{"object":"smtp_credential","id":"11111111-1111-4111-8111-111111111111","name":"App","scope":"global"}`
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSMTPCredential", requests.CreateSMTPCredentialRequest{Name: "App", Scope: "global"}).
		Return(mocks.APIResponse[responses.SMTPCredential](t, body), nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewCreateCommand(), "json", "App\n")

	require.NoError(t, err)
	assert.Equal(t, testutil.IndentedJSON(t, body), stdout)
	assert.Equal(t, "Credential name: ", stderr)
	mockClient.AssertExpectations(t)
}

func TestCreateRejectsBadInputBeforeAuthenticating(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{nil, "--name is required"},
		{[]string{"--name", "App", "--scope", "everything"}, "--scope must be one of: global, scoped"},
		{[]string{"--name", "App", "--scope", "scoped"}, "--domains is required when --scope is scoped"},
		{[]string{"--name", "App", "--scope", "scoped", "--domains", ""}, "--domains cannot be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			_, stderr, err := testutil.RunCommand(t, NewCreateCommand(), "plain", "App\n", tt.args...)

			require.Error(t, err)
			assert.Equal(t, tt.message, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.NotContains(t, stderr, "Credential name:")
			assert.False(t, *called)
		})
	}
}

func TestCredentialIDsAreCheckedBeforeAuthenticating(t *testing.T) {
	for _, command := range []func() *cobra.Command{NewGetCommand, NewDeleteCommand} {
		called := testutil.RefuseAuth(t)

		_, stderr, err := testutil.RunCommand(t, command(), "plain", "y\n", "abc")

		require.Error(t, err)
		assert.Equal(t, `invalid credential ID value "abc": not a UUID`, err.Error())
		assert.NotContains(t, stderr, "(y/N)")
		assert.False(t, *called)
	}
}

func TestCreateRejectsABadScopeBeforeAuthenticating(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, _, err := testutil.RunCommand(t, NewCreateCommand(), "json", "", "--name", "App", "--scope", "everything")

	require.Error(t, err)
	assert.False(t, *called)
}

func TestSendPromptsOnStderrForMissingRequiredFlags(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())

	stdout, stderr, err := testutil.RunCommand(t, NewSendCommand(), "plain",
		"sender@example.com\nuser@example.com\nHello\nHi\nsmtp-user\nsmtp-pass\n", "--server", "127.0.0.1:1")

	require.Error(t, err, "nothing listens on the test server")
	assert.Equal(t, 7, clierrors.GetExitCode(err))
	assert.NotContains(t, stdout, "address:")
	assert.True(t, strings.HasPrefix(stderr, "Sender email address: Recipient email address: "+
		"Email subject: Email content (plain text): SMTP username: SMTP password: "), "stderr: %s", stderr)
}

func TestSendWithoutATerminalDoesNotPrompt(t *testing.T) {
	_, stderr, err := testutil.RunCommand(t, NewSendCommand(), "plain", "sender@example.com\n", "--server", "127.0.0.1:1")

	require.Error(t, err)
	assert.Equal(t, "--from is required", err.Error())
	assert.Equal(t, 4, clierrors.GetExitCode(err))
	assert.NotContains(t, stderr, "Sender email address:")
}

func TestSendRejectsBadInputBeforePrompting(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"--text-file", "/no/such/file.txt"}, "failed to read --text-file: " + testutil.OpenError(t, "/no/such/file.txt")},
		{[]string{"--html-file", "/no/such/file.html"}, "failed to read --html-file: " + testutil.OpenError(t, "/no/such/file.html")},
		{[]string{"--attach", "/no/such/file.pdf"}, "failed to read --attach: " + testutil.StatError(t, "/no/such/file.pdf")},
		{[]string{"--attach", "inv,2026.txt"}, `invalid --attach value "inv": no such file; a file name with a comma must be in double quotes: --attach '"a,b.pdf"'`},
		{[]string{"--sandbox", "--sandbox-result", "explode"}, "--sandbox-result must be one of: deliver, bounce, defer, fail, suppress"},
		{[]string{"--server", "mail.example.com:smtp"}, `invalid --server value "mail.example.com:smtp": the port must be a number from 1 to 65535`},
		{[]string{"--server", ""}, "--server cannot be empty"},
		{[]string{"--tag", ""}, "--tag cannot be empty"},
		{[]string{"--from", "not-an-email"}, `invalid --from value "not-an-email": not an email address`},
		{[]string{"--header", "X-Test"}, "--header must be Name: value"},
		// A header a given flag sets cannot come from --header too, in any case
		{[]string{"--tag", "a", "--header", "AhaSend-Tags: b"}, "--header AhaSend-Tags and --tag cannot be used together"},
		{[]string{"--tags", "a", "--header", "AhaSend-Tags: b"}, "--header AhaSend-Tags and --tags cannot be used together"},
		{[]string{"--track-opens=false", "--header", "ahasend-track-opens: true"},
			"--header ahasend-track-opens and --track-opens cannot be used together"},
		{[]string{"--track-clicks", "--header", "AhaSend-Track-Clicks: false"},
			"--header AhaSend-Track-Clicks and --track-clicks cannot be used together"},
		{[]string{"--sandbox", "--header", "AhaSend-Sandbox: false"}, "--header AhaSend-Sandbox and --sandbox cannot be used together"},
		{[]string{"--sandbox", "--header", "AhaSend-Sandbox-Result: bounce"},
			"--header AhaSend-Sandbox-Result and --sandbox cannot be used together"},
		{[]string{"--sandbox", "--sandbox-result", "fail", "--header", "AhaSend-Sandbox-Result: bounce"},
			"--header AhaSend-Sandbox-Result and --sandbox-result cannot be used together"},
		// A header the command always builds cannot come from --header, in any case
		{[]string{"--header", "From: other@example.com"}, "--header From cannot be used; set it with --from"},
		{[]string{"--header", "sender: other@example.com"}, "--header sender cannot be used; the sender is --from"},
		{[]string{"--header", "TO: other@example.com"}, "--header TO cannot be used; set it with --to"},
		{[]string{"--header", "Cc: other@example.com"}, "--header Cc cannot be used; set it with --cc"},
		{[]string{"--header", "bcc: other@example.com"}, "--header bcc cannot be used; set it with --bcc"},
		{[]string{"--header", "Subject: Hi"}, "--header Subject cannot be used; set it with --subject"},
		{[]string{"--header", "MIME-Version: 1.0"}, "--header MIME-Version cannot be used; the command sets it"},
		{[]string{"--header", "content-type: text/plain"},
			"--header content-type cannot be used; the command sets it from the content and attachments"},
		{[]string{"--header", "Content-Transfer-Encoding: 8bit"},
			"--header Content-Transfer-Encoding cannot be used; the command sets it from the content and attachments"},
		{[]string{"--header", "X-Test: a", "--header", "x-test: b"}, "--header gives the header x-test twice; header names ignore case"},
	}
	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			args := append([]string{"--server", "127.0.0.1:1"}, tt.args...)
			if tt.args[0] == "--server" {
				args = tt.args
			}
			_, stderr, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", args...)

			require.Error(t, err)
			assert.Equal(t, tt.message, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.NotContains(t, stderr, "address:")
		})
	}
}

func TestSendRejectsABadRecipientBeforeConnecting(t *testing.T) {
	complete := []string{"--from", "sender@example.com", "--subject", "Hi", "--text", "Hi",
		"--username", "u", "--password", "p", "--server", "127.0.0.1:1"}
	for _, flag := range []string{"--to", "--cc", "--bcc"} {
		t.Run(flag, func(t *testing.T) {
			args := append([]string{"--to", "user@example.com", flag, "not-an-email"}, complete...)

			_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", args...)

			require.Error(t, err)
			assert.Equal(t, `invalid `+flag+` value "not-an-email": not an email address`, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
		})
	}
}

func TestSendEndingInputAtAPromptCancels(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())

	_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "sender@example.com\n", "--server", "127.0.0.1:1")

	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Equal(t, 4, clierrors.GetExitCode(err))
}

func TestSendTagsBecomeOneHeader(t *testing.T) {
	cmd := NewSendCommand()
	require.NoError(t, cmd.ParseFlags([]string{"--tag", "a", "--tag", "b,c"}))
	tags, err := cmdutil.ReadRepeatable(cmd, "tag", "")
	require.NoError(t, err)

	message := buildGomailMessage(smtpMessage{
		from: "s@example.com", to: []string{"r@example.com"}, subject: "Hi", text: "Hi", tags: tags,
	})

	assert.Equal(t, []string{"a,b,c"}, message.GetHeader("AhaSend-Tags"))
}

// TestSendSetsOnlyTheTrackingHeadersGiven checks that each tracking header is
// set only when its flag is set, with the flag's value, so --track-opens=false
// turns open tracking off.
func TestSendSetsOnlyTheTrackingHeadersGiven(t *testing.T) {
	tests := map[string]struct {
		args         []string
		opens, click []string
	}{
		"no tracking flags":   {nil, nil, nil},
		"--track-opens=false": {[]string{"--track-opens=false"}, []string{"false"}, nil},
		"--track-clicks":      {[]string{"--track-clicks"}, nil, []string{"true"}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cmd := NewSendCommand()
			require.NoError(t, cmd.ParseFlags(test.args))

			message := buildGomailMessage(smtpMessage{
				from: "s@example.com", to: []string{"r@example.com"}, subject: "Hi", text: "Hi",
				trackOpens: cmdutil.ChangedBool(cmd, "track-opens"), trackClicks: cmdutil.ChangedBool(cmd, "track-clicks"),
			})

			assert.Equal(t, test.opens, message.GetHeader("AhaSend-Track-Opens"))
			assert.Equal(t, test.click, message.GetHeader("AhaSend-Track-Clicks"))
		})
	}
}

func TestSendCredentialIDIsDeprecatedAndHasNoEffect(t *testing.T) {
	flag := NewSendCommand().Flags().Lookup("credential-id")

	assert.True(t, flag.Hidden)
	assert.Contains(t, flag.Deprecated, "no effect")
}

// TestSendTakesAnAhaSendHeaderNoFlagSets checks that --header may set an
// AhaSend header whose flag is not given.
func TestSendTakesAnAhaSendHeaderNoFlagSets(t *testing.T) {
	err := checkFlagHeaders(NewSendCommand(), map[string]string{"AhaSend-Tags": "b", "AhaSend-Sandbox-Result": "bounce"}, false)

	assert.NoError(t, err)
}
