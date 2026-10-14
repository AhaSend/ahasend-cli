package messages

import (
	"testing"

	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// listParams runs messages list with args and returns the params it sent.
func listParams(t *testing.T, args ...string) requests.GetMessagesParams {
	t.Helper()
	var sent requests.GetMessagesParams
	mockClient := &mocks.MockClient{}
	mockClient.On("GetAccountID").Return(testAccountID)
	mockClient.On("GetMessages", mock.Anything).Run(func(arguments mock.Arguments) {
		sent = arguments.Get(0).(requests.GetMessagesParams)
	}).Return(mocks.APIResponse[responses.PaginatedMessagesResponse](t, `{"object":"list","data":[],"pagination":{"has_more":false}}`), nil)
	testutil.InjectClient(t, mockClient)

	_, _, err := testutil.RunCommand(t, NewListCommand(), "plain", "", args...)
	require.NoError(t, err)
	return sent
}

func TestListLeavesUnsetFiltersOut(t *testing.T) {
	sent := listParams(t, "--recipient", "user@example.com")

	assert.Equal(t, "user@example.com", *sent.Recipient)
	assert.Nil(t, sent.Sender)
	assert.Nil(t, sent.Subject)
	assert.Nil(t, sent.MessageIDHeader)
	assert.Nil(t, sent.Status)
	assert.Nil(t, sent.Tags)
}

// TestListLeavesBlankFiltersOut checks that a blank text filter is left out
// of the request, as an empty one is, whichever filter it is.
func TestListLeavesBlankFiltersOut(t *testing.T) {
	sent := listParams(t, "--recipient", "   ", "--sender", "   ", "--subject", " ", "--message-id", "\t")

	assert.Nil(t, sent.Recipient)
	assert.Nil(t, sent.Sender)
	assert.Nil(t, sent.Subject)
	assert.Nil(t, sent.MessageIDHeader)
}

// TestListSendsAPartOfARecipientAsGiven checks that --recipient takes any
// part of an address, which the API matches as a substring.
func TestListSendsAPartOfARecipientAsGiven(t *testing.T) {
	sent := listParams(t, "--recipient", "@example.com")

	assert.Equal(t, "@example.com", *sent.Recipient)
}

func TestListSendsRepeatedTagsAndStatuses(t *testing.T) {
	sent := listParams(t, "--tag", "a", "--tag", "b", "--status", "delivered", "--status", "sandbox bounced",
		"--status", "Opened", "--status", "clicked")

	assert.Equal(t, []string{"a", "b"}, sent.Tags)
	assert.Equal(t, "Delivered,Sandbox Bounced,Opened,Clicked", *sent.Status)
}

func TestListInvalidStatusListsTheValidOnesInOrder(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, _, err := testutil.RunCommand(t, NewListCommand(), "plain", "", "--status", "lost")

	require.Error(t, err)
	assert.Equal(t, "--status must be one of: received, delivered, opened, clicked, deferred, bounced, failed, suppressed, "+
		"sandbox delivered, sandbox deferred, sandbox failed, sandbox bounced, sandbox suppressed", err.Error())
	assert.False(t, *called)
}

// TestListTagErrorsNameTheFlagAsGiven checks that a --tag error names the
// deprecated --tags when that is the name the user gave.
func TestListTagErrorsNameTheFlagAsGiven(t *testing.T) {
	tooMany := "a,b,c,d,e,f,g,h,i,j,k,l,m,n,o,p,q,r,s,t,u"
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"--tag", tooMany}, "--tag takes at most 20 tags, got 21"},
		{[]string{"--tags", tooMany}, "--tags takes at most 20 tags, got 21"},
		{[]string{"--tags", `"a,b"`}, `invalid --tags value "a,b": a filter tag cannot hold a comma, at which the API splits the tags`},
	}
	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			_, _, err := testutil.RunCommand(t, NewListCommand(), "plain", "", tt.args...)

			require.Error(t, err)
			assert.Equal(t, tt.message, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.False(t, *called)
		})
	}
}

func TestMessageIDsAreCheckedBeforeAuthenticating(t *testing.T) {
	for _, command := range []func() *cobra.Command{NewGetCommand, NewCancelCommand} {
		called := testutil.RefuseAuth(t)

		_, stderr, err := testutil.RunCommand(t, command(), "plain", "y\n", "abc")

		require.Error(t, err)
		assert.Equal(t, `invalid message ID value "abc": not a UUID or <uuid@domain>`, err.Error())
		assert.Equal(t, 4, clierrors.GetExitCode(err))
		assert.NotContains(t, stderr, "(y/N)")
		assert.False(t, *called)
	}
}

func TestCancelUsageShowsItTakesSeveralIDs(t *testing.T) {
	assert.Equal(t, "cancel <message-id>... [flags]", NewCancelCommand().UseLine())
}
