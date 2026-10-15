package routes

import (
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

const testRouteID = "11111111-1111-4111-8111-111111111111"

func TestCommandsRejectBadInputBeforeAuthenticating(t *testing.T) {
	tests := []struct {
		name    string
		command func() *cobra.Command
		args    []string
		message string
	}{
		{"create without a name", NewCreateCommand, []string{"--url", "https://example.com/hook"}, "--name is required"},
		{"create without a URL", NewCreateCommand, []string{"--name", "Inbound"}, "--url is required"},
		{"create without a recipient", NewCreateCommand, []string{"--name", "Inbound", "--url", "https://example.com/hook"},
			"--recipient is required"},
		{"create with a bad URL", NewCreateCommand, []string{"--name", "Inbound", "--url", "https://"},
			"--url must be an HTTP or HTTPS URL with a host, such as https://example.com/webhook"},
		{"get with a bad ID", NewGetCommand, []string{"abc"}, `invalid route ID value "abc": not a UUID`},
		{"update with a bad ID", NewUpdateCommand, []string{"abc", "--name", "x"}, `invalid route ID value "abc": not a UUID`},
		{"update with nothing to update", NewUpdateCommand, []string{testRouteID},
			"at least one of --name, --url, --recipient, --enabled, --include-attachments, --include-headers, --group-by-message-id or --strip-replies must be provided"},
		{"update with an empty name", NewUpdateCommand, []string{testRouteID, "--name", ""}, "--name cannot be empty"},
		{"update with an empty URL", NewUpdateCommand, []string{testRouteID, "--url", ""}, "--url cannot be empty"},
		{"update with a bad URL", NewUpdateCommand, []string{testRouteID, "--url", "ftp://example.com"},
			"--url must be an HTTP or HTTPS URL with a host, such as https://example.com/webhook"},
		{"trigger with a bad ID", NewTriggerCommand, []string{"abc"}, `invalid route ID value "abc": not a UUID`},
		{"listen without a route", NewListenCommand, nil, "one of --route-id or --recipient must be provided"},
		{"listen with both", NewListenCommand, []string{"--route-id", testRouteID, "--recipient", "*@example.com"},
			"--route-id and --recipient cannot be used together"},
		{"listen with a bad route ID", NewListenCommand, []string{"--route-id", "abc"}, `invalid route ID value "abc": not a UUID`},
		{"listen with a long recipient", NewListenCommand, []string{"--recipient", strings.Repeat("a", 256)},
			"--recipient must be at most 255 characters"},
		{"create with a long recipient", NewCreateCommand, []string{"--name", "Inbound", "--url", "https://example.com/hook", "--recipient", strings.Repeat("a", 256)},
			"--recipient must be at most 255 characters"},
		{"update with a long recipient", NewUpdateCommand, []string{testRouteID, "--recipient", strings.Repeat("a", 256)},
			"--recipient must be at most 255 characters"},
		{"listen with a bad forward URL", NewListenCommand, []string{"--recipient", "*@example.com", "--forward-to", "localhost:3000"},
			"--forward-to must be an HTTP or HTTPS URL with a host, such as https://example.com/webhook"},
		{"listen with a forward URL with a password", NewListenCommand, []string{"--recipient", "*@example.com", "--forward-to", "http://user:pass@localhost:3000"},
			"--forward-to must have no user name or password, such as https://example.com/webhook"},
		{"listen with skip-verify and no forward URL", NewListenCommand, []string{"--recipient", "*@example.com", "--skip-verify"},
			"--skip-verify needs --forward-to"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			_, stderr, err := testutil.RunCommand(t, tt.command(), "plain", "", tt.args...)

			require.Error(t, err)
			assert.Equal(t, tt.message, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.Equal(t, "Error: "+err.Error()+"\n", stderr, "the command must not prompt")
			assert.False(t, *called, "the command must not authenticate")
		})
	}
}

func TestDeleteRejectsBadIDBeforeConfirming(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "y\n", "abc")

	require.Error(t, err)
	assert.Equal(t, `invalid route ID value "abc": not a UUID`, err.Error())
	assert.NotContains(t, stderr, "(y/N)")
	assert.False(t, *called)
}

func TestCreateNonInteractiveDoesNotPromptOnATerminal(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
	testutil.RefuseAuth(t)

	for _, flag := range []string{"--non-interactive"} {
		_, stderr, err := testutil.RunCommand(t, NewCreateCommand(), "plain", "Inbound\n", flag)

		require.Error(t, err, flag)
		assert.Equal(t, "--name is required", err.Error(), flag)
		assert.NotContains(t, stderr, "Route name:", flag)
	}
}

// createdRequest runs routes create with args and returns the request it
// sent.
func createdRequest(t *testing.T, args ...string) requests.CreateRouteRequest {
	t.Helper()
	var sent requests.CreateRouteRequest
	mockClient := &mocks.MockClient{}
	testutil.CaptureRequest(mockClient, "CreateRoute", &sent, mocks.APIResponse[responses.Route](t, routeBody))
	testutil.InjectClient(t, mockClient)

	_, _, err := testutil.RunCommand(t, NewCreateCommand(), "json", "",
		append([]string{"--name", "Inbound", "--url", "https://example.com/hook", "--recipient", "*@example.com"}, args...)...)
	require.NoError(t, err)
	return sent
}

func TestCreateSendsTheRecipient(t *testing.T) {
	assert.Equal(t, "*@example.com", createdRequest(t).Recipient)
}

func TestCreateLeavesEnabledToTheAPIUnlessSet(t *testing.T) {
	assert.Nil(t, createdRequest(t).Enabled)
	assert.Equal(t, false, *createdRequest(t, "--enabled=false").Enabled)
	assert.Equal(t, true, *createdRequest(t, "--enabled").Enabled)
}

// updatedRequest runs routes update with args and returns the request it
// sent.
func updatedRequest(t *testing.T, args ...string) requests.UpdateRouteRequest {
	t.Helper()
	var sent requests.UpdateRouteRequest
	mockClient := &mocks.MockClient{}
	testutil.CaptureRequest(mockClient, "UpdateRoute", &sent, mocks.APIResponse[responses.Route](t, routeBody), uuid.MustParse(testRouteID))
	testutil.InjectClient(t, mockClient)

	_, _, err := testutil.RunCommand(t, NewUpdateCommand(), "json", "", append([]string{testRouteID}, args...)...)
	require.NoError(t, err)
	return sent
}

func TestUpdateSendsOnlyTheOptionsThatAreSet(t *testing.T) {
	sent := updatedRequest(t, "--enabled=false", "--include-attachments", "--strip-replies=false")

	assert.Equal(t, false, *sent.Enabled)
	assert.Equal(t, true, *sent.Attachments)
	assert.Equal(t, false, *sent.StripReplies)
	assert.Nil(t, sent.Headers)
	assert.Nil(t, sent.GroupByMessageId)
	assert.Nil(t, sent.Name)
	assert.Nil(t, sent.Recipient)
}

func TestUpdateDeprecatedFlagsStillWork(t *testing.T) {
	sent := updatedRequest(t, "--disabled", "--no-include-headers")

	assert.Equal(t, false, *sent.Enabled)
	assert.Equal(t, false, *sent.Headers)
}

func TestUpdateSendsTheRecipient(t *testing.T) {
	assert.Equal(t, "support-*@example.com", *updatedRequest(t, "--recipient", "support-*@example.com").Recipient)
}

// TestRecipientsAreLocalAtDomain checks that create, update and listen take
// a --recipient of the form local@domain only, as the API does, and refuse
// any other value before authenticating.
func TestRecipientsAreLocalAtDomain(t *testing.T) {
	commands := map[string]func(recipient string) (func() *cobra.Command, []string){
		"create": func(recipient string) (func() *cobra.Command, []string) {
			return NewCreateCommand, []string{"--name", "Inbound", "--url", "https://example.com/hook", "--recipient", recipient}
		},
		"update": func(recipient string) (func() *cobra.Command, []string) {
			return NewUpdateCommand, []string{testRouteID, "--recipient", recipient}
		},
		"listen": func(recipient string) (func() *cobra.Command, []string) {
			return NewListenCommand, []string{"--recipient", recipient}
		},
	}
	const reason = "must be local@domain with one of your domains, such as support@example.com or *@example.com"
	for _, recipient := range []string{"support@*", "*sales*", "@example.com", "support@", "a@b@example.com", "support@exa mple.com"} {
		for name, command := range commands {
			t.Run(name+" "+recipient, func(t *testing.T) {
				called := testutil.RefuseAuth(t)
				newCommand, args := command(recipient)

				_, _, err := testutil.RunCommand(t, newCommand(), "plain", "", args...)

				require.Error(t, err)
				assert.Equal(t, `invalid --recipient value "`+recipient+`": `+reason, err.Error())
				assert.Equal(t, 4, clierrors.GetExitCode(err))
				assert.False(t, *called, "the command must not authenticate")
			})
		}
	}
	t.Run("update with an empty recipient", func(t *testing.T) {
		called := testutil.RefuseAuth(t)

		_, _, err := testutil.RunCommand(t, NewUpdateCommand(), "plain", "", testRouteID, "--recipient", "")

		require.Error(t, err)
		assert.Equal(t, "--recipient cannot be empty: a route's recipient cannot be cleared", err.Error())
		assert.Equal(t, 4, clierrors.GetExitCode(err))
		assert.False(t, *called)
	})
}

func TestDeprecatedFlagsAreHidden(t *testing.T) {
	flags := NewUpdateCommand().Flags()
	for _, name := range []string{"disabled", "no-include-attachments", "no-include-headers", "no-group-by-message-id", "no-strip-replies"} {
		flag := flags.Lookup(name)
		require.NotNil(t, flag, name)
		assert.True(t, flag.Hidden, name)
		assert.True(t, strings.HasPrefix(flag.Deprecated, "use --"), name)
	}
}

func TestGetCSVWritesEveryRouteColumn(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("GetRoute", uuid.MustParse(testRouteID)).Return(mocks.APIResponse[responses.Route](t,
		`{"object":"route","id":"`+testRouteID+`","name":"Inbound","url":"https://example.com/hook","recipient":"*@example.com","enabled":true,`+
			`"created_at":"2026-09-10T08:30:00Z","updated_at":"2026-09-10T09:30:00Z"}`), nil)

	out, err := testutil.RunWithClient(t, mockClient, NewGetCommand(), "csv", testRouteID)

	require.NoError(t, err)
	assert.Equal(t, "name,id,url,enabled,recipient,attachments,headers,group_by_message_id,strip_replies,created_at,updated_at,"+
		"success_count,error_count,errors_since_last_success,last_request_at\n"+
		"Inbound,"+testRouteID+",https://example.com/hook,true,*@example.com,false,false,false,false,2026-09-10T08:30:00Z,2026-09-10T09:30:00Z,"+
		"0,0,0,\n", out)
}

// TestListenWarnsAboutAnHTTPForwardURLBeforeAuthenticating checks that
// --forward-to gets the HTTP warning create --url gets, except for localhost.
func TestListenWarnsAboutAnHTTPForwardURLBeforeAuthenticating(t *testing.T) {
	tests := map[string]bool{"http://example.com/hook": true, "http://localhost:3000/hook": false, "https://example.com/hook": false}
	for forwardTo, warns := range tests {
		t.Run(forwardTo, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("InitiateRouteStream", mock.Anything, mock.Anything).Return(nil, clierrors.NewAPIError("stop", nil))
			testutil.InjectClient(t, mockClient)

			_, stderr, _ := testutil.RunCommand(t, NewListenCommand(), "plain", "", "--recipient", "*@example.com", "--forward-to", forwardTo)

			assert.Equal(t, warns, strings.Contains(stderr, "uses plain HTTP, so each email is sent unencrypted. Use HTTPS."), stderr)
		})
	}
}
