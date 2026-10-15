package webhooks

import (
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

const testWebhookID = "11111111-1111-4111-8111-111111111111"

func TestCommandsRejectBadInputBeforeAuthenticating(t *testing.T) {
	tests := []struct {
		name    string
		command func() *cobra.Command
		args    []string
		message string
	}{
		{"create without a name", NewCreateCommand, []string{"--url", "https://example.com/hook"}, "--name is required"},
		{"create without a URL", NewCreateCommand, []string{"--name", "Events"}, "--url is required"},
		{"create with an empty scope", NewCreateCommand, []string{"--name", "Events", "--url", "https://example.com/hook", "--scope", ""},
			"--scope must be one of: global, scoped"},
		{"create with an unknown scope", NewCreateCommand, []string{"--name", "Events", "--url", "https://example.com/hook", "--scope", "all"},
			"--scope must be one of: global, scoped"},
		{"create scoped without domains", NewCreateCommand, []string{"--name", "Events", "--url", "https://example.com/hook", "--scope", "scoped"},
			"--domains is required when --scope is scoped"},
		{"create with a bad URL", NewCreateCommand, []string{"--name", "Events", "--url", "example.com"},
			"--url must be an HTTP or HTTPS URL with a host, such as https://example.com/webhook"},
		{"create with an empty event", NewCreateCommand, []string{"--name", "Events", "--url", "https://example.com/hook", "--scope", "global", "--events", ""},
			"--events cannot be empty"},
		{"create with an unknown event", NewCreateCommand, []string{"--name", "Events", "--url", "https://example.com/hook", "--scope", "global", "--events", "sent"},
			`"sent" is not an event; --events must be one of: reception, delivered,`},
		{"create with events and all events", NewCreateCommand,
			[]string{"--name", "Events", "--url", "https://example.com/hook", "--scope", "global", "--events", "delivered", "--all-events"},
			"--events and --all-events cannot be used together"},
		{"get with a bad ID", NewGetCommand, []string{"abc"}, `invalid webhook ID value "abc": not a UUID`},
		{"update with a bad ID", NewUpdateCommand, []string{"abc", "--name", "x"}, `invalid webhook ID value "abc": not a UUID`},
		{"update with nothing to update", NewUpdateCommand, []string{testWebhookID},
			"at least one of --name, --url, --enabled, --events, --all-events, --clear-events, --scope or --domains must be provided"},
		{"update with an empty name", NewUpdateCommand, []string{testWebhookID, "--name", ""}, "--name cannot be empty"},
		{"update with an empty event", NewUpdateCommand, []string{testWebhookID, "--events", ""},
			"--events cannot be empty; use --clear-events to clear it"},
		{"update with events and clear events", NewUpdateCommand, []string{testWebhookID, "--events", "delivered", "--clear-events"},
			"--events and --clear-events cannot be used together"},
		{"update with a bad URL", NewUpdateCommand, []string{testWebhookID, "--url", "ftp://example.com"},
			"--url must be an HTTP or HTTPS URL with a host, such as https://example.com/webhook"},
		{"listen with a bad forward URL", NewListenCommand, []string{"--forward-to", "localhost:3000"},
			"--forward-to must be an HTTP or HTTPS URL with a host, such as https://example.com/webhook"},
		{"listen with a forward URL with a user name", NewListenCommand, []string{"--forward-to", "http://user@localhost:3000"},
			"--forward-to must have no user name or password, such as https://example.com/webhook"},
		{"listen with skip-verify and no forward URL", NewListenCommand, []string{"--skip-verify"},
			"--skip-verify needs --forward-to"},
		{"trigger with a bad ID", NewTriggerCommand, []string{"abc", "--all-events"}, `invalid webhook ID value "abc": not a UUID`},
		{"trigger without events", NewTriggerCommand, []string{testWebhookID},
			"at least one of --events or --all-events must be provided"},
		{"trigger with an unknown event", NewTriggerCommand, []string{testWebhookID, "--events", "message.sent"},
			`"message.sent" is not an event; --events must be one of: reception, delivered,`},
		{"listen with a bad webhook ID", NewListenCommand, []string{"--webhook-id", "abc"}, `invalid webhook ID value "abc": not a UUID`},
		{"listen with an unknown event", NewListenCommand, []string{"--events", "sent"}, `"sent" is not an event; --events must be one of: reception, delivered,`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			_, stderr, err := testutil.RunCommand(t, tt.command(), "plain", "", tt.args...)

			require.Error(t, err)
			assert.True(t, strings.HasPrefix(err.Error(), tt.message), "error: %s", err)
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
	assert.Equal(t, `invalid webhook ID value "abc": not a UUID`, err.Error())
	assert.NotContains(t, stderr, "(y/N)")
	assert.False(t, *called)
}

func TestCreateNonInteractiveDoesNotPromptOnATerminal(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
	testutil.RefuseAuth(t)

	for _, flag := range []string{"--non-interactive"} {
		_, stderr, err := testutil.RunCommand(t, NewCreateCommand(), "plain", "Events\n", flag)

		require.Error(t, err, flag)
		assert.Equal(t, "--name is required", err.Error(), flag)
		assert.NotContains(t, stderr, "Webhook name:", flag)
	}
}

// createdRequest runs webhooks create with args and returns the request it
// sent.
func createdRequest(t *testing.T, args ...string) requests.CreateWebhookRequest {
	t.Helper()
	var sent requests.CreateWebhookRequest
	mockClient := &mocks.MockClient{}
	testutil.CaptureRequest(mockClient, "CreateWebhook", &sent, mocks.APIResponse[responses.Webhook](t, webhookBody))
	testutil.InjectClient(t, mockClient)

	_, _, err := testutil.RunCommand(t, NewCreateCommand(), "json", "",
		append([]string{"--name", "Events", "--url", "https://example.com/hook"}, args...)...)
	require.NoError(t, err)
	return sent
}

func TestCreateReadsEventsWithSpacesAfterTheCommas(t *testing.T) {
	sent := createdRequest(t, "--events", "delivered, opened")

	assert.True(t, sent.OnDelivered)
	assert.True(t, sent.OnOpened)
}

func TestCreateLeavesEnabledToTheAPIUnlessSet(t *testing.T) {
	assert.Nil(t, createdRequest(t).Enabled)
	assert.Equal(t, false, *createdRequest(t, "--enabled=false").Enabled)
	assert.Equal(t, false, *createdRequest(t, "--disabled").Enabled)
	assert.Equal(t, true, *createdRequest(t, "--enabled").Enabled)
}

func TestCreateScopeIsGlobalUnlessSet(t *testing.T) {
	assert.Equal(t, "global", createdRequest(t).Scope)
	assert.Equal(t, "scoped", createdRequest(t, "--scope", "scoped", "--domains", "example.com").Scope)
}

func TestCreateSendsRepeatedEventsAndDomains(t *testing.T) {
	sent := createdRequest(t, "--scope", "scoped", "--events", "delivered", "--events", "bounced", "--domains", "a.example.com", "--domains", "b.example.com")

	assert.True(t, sent.OnDelivered)
	assert.True(t, sent.OnBounced)
	assert.False(t, sent.OnOpened)
	assert.Equal(t, []string{"a.example.com", "b.example.com"}, *sent.Domains)
}

// updatedRequest runs webhooks update with args and returns the request it
// sent.
func updatedRequest(t *testing.T, args ...string) requests.UpdateWebhookRequest {
	t.Helper()
	var sent requests.UpdateWebhookRequest
	mockClient := &mocks.MockClient{}
	testutil.CaptureRequest(mockClient, "UpdateWebhook", &sent, mocks.APIResponse[responses.Webhook](t, webhookBody), uuid.MustParse(testWebhookID))
	testutil.InjectClient(t, mockClient)

	_, _, err := testutil.RunCommand(t, NewUpdateCommand(), "json", "", append([]string{testWebhookID}, args...)...)
	require.NoError(t, err)
	return sent
}

func TestUpdateSendsEnabledOnlyWhenSet(t *testing.T) {
	assert.Nil(t, updatedRequest(t, "--name", "x").Enabled)
	assert.Equal(t, false, *updatedRequest(t, "--enabled=false").Enabled)
	assert.Equal(t, true, *updatedRequest(t, "--enabled").Enabled)
	assert.Equal(t, true, *updatedRequest(t, "--enable").Enabled)
	assert.Equal(t, false, *updatedRequest(t, "--disable").Enabled)
}

func TestUpdateClearEventsTurnsEveryEventOff(t *testing.T) {
	for _, flag := range []string{"--clear-events", "--no-events"} {
		sent := updatedRequest(t, flag)

		require.NotNil(t, sent.OnDelivered, flag)
		assert.False(t, *sent.OnDelivered, flag)
		assert.False(t, *sent.OnDnsError, flag)
	}
}

// TestUpdateClearDomainsIsADeprecatedScopeGlobal checks that --clear-domains
// makes the webhook global, which removes its domains, and sends no empty
// domain list, which would leave a scoped webhook firing for nothing.
func TestUpdateClearDomainsIsADeprecatedScopeGlobal(t *testing.T) {
	var sent requests.UpdateWebhookRequest
	mockClient := &mocks.MockClient{}
	testutil.CaptureRequest(mockClient, "UpdateWebhook", &sent, mocks.APIResponse[responses.Webhook](t, webhookBody), uuid.MustParse(testWebhookID))
	testutil.InjectClient(t, mockClient)

	_, _, err := testutil.RunCommand(t, NewUpdateCommand(), "json", "", testWebhookID, "--clear-domains")

	require.NoError(t, err)
	assert.Equal(t, "global", *sent.Scope)
	assert.Nil(t, sent.Domains)
	assert.True(t, NewUpdateCommand().Flags().Lookup("clear-domains").Hidden)
}

func TestUpdateClearDomainsCannotBeUsedWithDomains(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, _, err := testutil.RunCommand(t, NewUpdateCommand(), "plain", "", testWebhookID, "--domains", "example.com", "--clear-domains")

	require.Error(t, err)
	assert.Equal(t, "--domains and --clear-domains cannot be used together", err.Error())
	assert.Equal(t, 4, clierrors.GetExitCode(err))
	assert.False(t, *called)
}

func TestListCSVShowsTheEventsOfEachWebhook(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("ListWebhooks", common.PaginationParams{}, (*bool)(nil)).Return(mocks.APIResponse[responses.PaginatedWebhooksResponse](t,
		`{"object":"list","data":[{"object":"webhook","id":"`+testWebhookID+`","name":"Events","url":"https://example.com/hook","enabled":true,"on_delivered":true,"on_bounced":true}],"pagination":{"has_more":false}}`), nil)
	testutil.InjectClient(t, mockClient)

	stdout, _, err := testutil.RunCommand(t, NewListCommand(), "csv", "")

	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	require.Len(t, lines, 2)
	assert.Equal(t, "name,id,url,enabled,events,scope,domains,created_at,updated_at,"+
		"success_count,error_count,errors_since_last_success,last_request_at", lines[0])
	assert.Contains(t, lines[1], "delivered")
	assert.Contains(t, lines[1], "bounced")
}

func TestListEnabledFalseAsksForDisabledWebhooks(t *testing.T) {
	disabled := false
	mockClient := &mocks.MockClient{}
	mockClient.On("ListWebhooks", common.PaginationParams{}, &disabled).Return(mocks.APIResponse[responses.PaginatedWebhooksResponse](t,
		`{"object":"list","data":[],"pagination":{"has_more":false}}`), nil)
	testutil.InjectClient(t, mockClient)

	_, _, err := testutil.RunCommand(t, NewListCommand(), "plain", "", "--enabled=false")

	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestListTablePutsTheNameThenTheID(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("ListWebhooks", common.PaginationParams{}, (*bool)(nil)).Return(mocks.APIResponse[responses.PaginatedWebhooksResponse](t,
		`{"object":"list","data":[{"object":"webhook","id":"`+testWebhookID+`","name":"Events","url":"https://example.com/hook","enabled":true}],"pagination":{"has_more":false}}`), nil)
	testutil.InjectClient(t, mockClient)

	stdout, _, err := testutil.RunCommand(t, NewListCommand(), "table", "")

	require.NoError(t, err)
	assert.Regexp(t, `(?m)^│\s+NAME\s+│\s+ID\s+│`, stdout)
	assert.Contains(t, stdout, testWebhookID)
}
