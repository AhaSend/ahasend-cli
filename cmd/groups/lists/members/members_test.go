package members

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-go"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testListID    = "22222222-2222-4222-8222-222222222222"
	testContactID = "11111111-1111-4111-8111-111111111111"
)

func testMember() *responses.ListContact {
	return &responses.ListContact{
		Object:             "list_contact",
		ListID:             uuid.MustParse(testListID),
		ContactID:          uuid.MustParse(testContactID),
		Email:              "person@example.com",
		SubscriptionStatus: "confirmed",
	}
}

// list ------------------------------------------------------------------------

func TestListSendsFiltersAndIncludeContacts(t *testing.T) {
	status := "confirmed"
	email := "person@example.com"
	include := true
	before := "previous"
	mockClient := &mocks.MockClient{}
	mockClient.On("GetListContacts", uuid.MustParse(testListID), requests.GetListContactsParams{
		Before:             &before,
		SubscriptionStatus: &status,
		Email:              &email,
		IncludeContacts:    &include,
	}).Return(&responses.PaginatedListContactsResponse{Object: "list"}, nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewListCommand(), testListID, "--before", before,
		"--subscription-status", status, "--email", email, "--include-contacts")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestListWithoutIncludeContactsSendsNoFlag(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("GetListContacts", uuid.MustParse(testListID), requests.GetListContactsParams{}).
		Return(&responses.PaginatedListContactsResponse{Object: "list"}, nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewListCommand(), testListID)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestListShowsEmbeddedContactNames(t *testing.T) {
	member := testMember()
	member.Contact = &responses.Contact{FirstName: "Pat", LastName: "Lee"}
	mockClient := &mocks.MockClient{}
	mockClient.On("GetListContacts", uuid.MustParse(testListID), mock.Anything).
		Return(&responses.PaginatedListContactsResponse{Object: "list", Data: []responses.ListContact{*member}}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, _, err := testutil.RunCommand(t, NewListCommand(), "table", "", testListID, "--include-contacts")
	require.NoError(t, err)
	assert.Contains(t, stdout, "FIRST NAME")
	assert.Contains(t, stdout, "Pat")
	assert.Contains(t, stdout, "Lee")
	assert.True(t, strings.HasSuffix(stdout, "\nShowing 1 member\n"), stdout)
	mockClient.AssertExpectations(t)
}

func TestListValidationHappensBeforeAuthentication(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"not-a-uuid"}, `invalid list ID value "not-a-uuid": not a UUID`},
		{[]string{testListID, "--after", "a", "--before", "b"}, "--after and --before cannot be used together"},
		{[]string{testListID, "--limit", "101"}, "--limit must be between 0 and 100"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)
			_, err := testutil.RunPlain(t, NewListCommand(), test.args...)
			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.False(t, *authCalled)
		})
	}
}

// add -------------------------------------------------------------------------

func TestAddBuildsOneItemPerValueEmailsFirst(t *testing.T) {
	contactID := uuid.MustParse(testContactID)
	expected := requests.BatchAddListContactsRequest{Data: []requests.BatchAddListContactInput{
		{Email: ahasend.String("a@example.com")},
		{Email: ahasend.String("b@example.com")},
		{Email: ahasend.String("c@example.com")},
		{ID: &contactID},
	}}
	mockClient := &mocks.MockClient{}
	mockClient.On("BatchAddListContacts", uuid.MustParse(testListID), expected, testutil.IdempotencyKeyArg("add-key")).
		Return(&responses.BatchAddListContactsResponse{Object: "list"}, nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewAddCommand(), testListID, "--contact-id", testContactID,
		"--email", "a@example.com", "--email", "b@example.com,c@example.com", "--idempotency-key", "add-key")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestAddFileSendsItemsAsWritten(t *testing.T) {
	path := testutil.WriteTempFile(t, "members.json", `{"data":[{"email":"a@example.com"},{"id":"`+testContactID+`"},{"email":"b@example.com","id":"`+testContactID+`"},{}]}`)
	contactID := uuid.MustParse(testContactID)
	expected := requests.BatchAddListContactsRequest{Data: []requests.BatchAddListContactInput{
		{Email: ahasend.String("a@example.com")},
		{ID: &contactID},
		{Email: ahasend.String("b@example.com"), ID: &contactID},
		{},
	}}
	mockClient := &mocks.MockClient{}
	mockClient.On("BatchAddListContacts", uuid.MustParse(testListID), expected).
		Return(&responses.BatchAddListContactsResponse{Object: "list"}, nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewAddCommand(), testListID, "--file", path)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

// partialAddResponse is a batch add result in which one of three contacts
// failed.
const partialAddResponse = `{"object":"list","added":1,"skipped":1,"failed":1,"data":[` +
	`{"position":0,"email":"a@example.com","outcome":"added"},` +
	`{"position":1,"email":"b@example.com","outcome":"already_member"},` +
	`{"position":2,"email":"c@example.com","outcome":"not_found","reason":"contact not found"}]}`

func TestAddPrintsResultAndFailsWhenAContactFailed(t *testing.T) {
	var response responses.BatchAddListContactsResponse
	require.NoError(t, json.Unmarshal([]byte(partialAddResponse), &response))
	mockClient := &mocks.MockClient{}
	mockClient.On("BatchAddListContacts", uuid.MustParse(testListID), mock.Anything).Return(&response, nil)
	testutil.InjectClient(t, mockClient)

	for _, format := range []string{"plain", "table"} {
		t.Run(format, func(t *testing.T) {
			stdout, _, err := testutil.RunCommand(t, NewAddCommand(), format, "", testListID,
				"--email", "a@example.com", "--email", "b@example.com", "--email", "c@example.com")
			var itemsFailed *clierrors.ItemsFailedError
			require.ErrorAs(t, err, &itemsFailed)
			assert.Equal(t, "1 of 3 contacts failed", err.Error())
			assert.Contains(t, stdout, "List member batch completed")
			assert.Contains(t, stdout, "Failed")
			assert.Contains(t, stdout, "not_found")
			assert.Contains(t, stdout, "contact not found")
		})
	}

	t.Run("json", func(t *testing.T) {
		stdout, _, err := testutil.RunCommand(t, NewAddCommand(), "json", "", testListID, "--email", "a@example.com")
		var itemsFailed *clierrors.ItemsFailedError
		require.ErrorAs(t, err, &itemsFailed)
		assert.JSONEq(t, partialAddResponse, stdout)
	})
}

func TestAddSkippedContactsDoNotFail(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("BatchAddListContacts", uuid.MustParse(testListID), mock.Anything).
		Return(&responses.BatchAddListContactsResponse{Object: "list", Added: 1, Skipped: 1, Data: []responses.BatchListContactResult{
			{Position: 0, Email: "a@example.com", Outcome: "added"},
			{Position: 1, Email: "b@example.com", Outcome: "already_member"},
		}}, nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewAddCommand(), testListID, "--email", "a@example.com,b@example.com")
	require.NoError(t, err)
}

func TestAddAcceptsMaxMembersFromFlags(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("BatchAddListContacts", uuid.MustParse(testListID),
		mock.MatchedBy(func(request requests.BatchAddListContactsRequest) bool { return len(request.Data) == 1000 })).
		Return(&responses.BatchAddListContactsResponse{Object: "list"}, nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewAddCommand(), testListID,
		"--email", strings.Repeat("a@example.com,", 998)+"a@example.com", "--contact-id", testContactID)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestAddValidationHappensBeforeAuthentication(t *testing.T) {
	tooMany := `{"data":[` + strings.Repeat(`{"email":"a@example.com"},`, 1000) + `{"email":"a@example.com"}]}`
	tests := []struct {
		name    string
		args    func(t *testing.T) []string
		message string
	}{
		{
			name:    "no input",
			args:    func(*testing.T) []string { return []string{testListID} },
			message: "one of --email, --contact-id or --file must be provided",
		},
		{
			name: "file and email",
			args: func(t *testing.T) []string {
				return []string{testListID, "--file", testutil.WriteTempFile(t, "members.json", `{"data":[{}]}`), "--email", "a@example.com"}
			},
			message: "--file and --email cannot be used together",
		},
		{
			name: "file and contact ID",
			args: func(t *testing.T) []string {
				return []string{testListID, "--file", testutil.WriteTempFile(t, "members.json", `{"data":[{}]}`), "--contact-id", testContactID}
			},
			message: "--file and --contact-id cannot be used together",
		},
		{
			name:    "empty email",
			args:    func(*testing.T) []string { return []string{testListID, "--email", ""} },
			message: "--email cannot be empty",
		},
		{
			name:    "empty email between commas",
			args:    func(*testing.T) []string { return []string{testListID, "--email", "a@example.com,,b@example.com"} },
			message: "--email cannot be empty",
		},
		{
			name:    "empty contact ID",
			args:    func(*testing.T) []string { return []string{testListID, "--contact-id", ""} },
			message: "--contact-id cannot be empty",
		},
		{
			name: "null file email",
			args: func(t *testing.T) []string {
				return []string{testListID, "--file", testutil.WriteTempFile(t, "members.json", `{"data":[{"email":"a@example.com"},{"email":null}]}`)}
			},
			message: "batch member at position 1 sets email to null",
		},
		{
			name: "null file id",
			args: func(t *testing.T) []string {
				return []string{testListID, "--file", testutil.WriteTempFile(t, "members.json", `{"data":[{"id":null}]}`)}
			},
			message: "batch member at position 0 sets id to null",
		},
		{
			name:    "bad contact ID",
			args:    func(*testing.T) []string { return []string{testListID, "--contact-id", "nope"} },
			message: `invalid contact ID value "nope": not a UUID`,
		},
		{
			name:    "bad list ID",
			args:    func(*testing.T) []string { return []string{"nope", "--email", "a@example.com"} },
			message: `invalid list ID value "nope": not a UUID`,
		},
		{
			name: "1001 file items",
			args: func(t *testing.T) []string {
				return []string{testListID, "--file", testutil.WriteTempFile(t, "members.json", tooMany)}
			},
			message: "batch file data takes at most 1000 members, got 1001",
		},
		{
			name: "1001 flag values",
			args: func(*testing.T) []string {
				return []string{testListID, "--email", strings.Repeat("a@example.com,", 999) + "a@example.com",
					"--contact-id", testContactID}
			},
			message: "--email and --contact-id take at most 1000 members in total, got 1001",
		},
		{
			name: "unknown file field",
			args: func(t *testing.T) []string {
				return []string{testListID, "--file", testutil.WriteTempFile(t, "members.json", `{"data":[{"email":"a@example.com","status":"x"}]}`)}
			},
			message: "batch member at position 0 is invalid",
		},
		{
			name: "non-UUID file id",
			args: func(t *testing.T) []string {
				return []string{testListID, "--file", testutil.WriteTempFile(t, "members.json", `{"data":[{"email":"a@example.com"},{"id":"nope"}]}`)}
			},
			message: "batch member at position 1 is invalid",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)
			_, err := testutil.RunPlain(t, NewAddCommand(), test.args(t)...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.message)
			assert.False(t, *authCalled)
		})
	}
}

// set -------------------------------------------------------------------------

func TestSetWithoutFlagSendsEmptyRequest(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("UpsertListContact", uuid.MustParse(testListID), "person@example.com", requests.UpsertListContactRequest{}).
		Return(testMember(), nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewSetCommand(), testListID, "person@example.com")
	require.NoError(t, err)
	assert.Contains(t, output, "Subscription Status: confirmed")
	mockClient.AssertExpectations(t)
}

func TestSetSendsSubscriptionStatus(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("UpsertListContact", uuid.MustParse(testListID), "person@example.com",
		requests.UpsertListContactRequest{SubscriptionStatus: ahasend.String("unsubscribed")}).
		Return(testMember(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewSetCommand(), testListID, "person@example.com", "--subscription-status", "unsubscribed")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestSetValidationHappensBeforeAuthentication(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"nope", "person@example.com"}, `invalid list ID value "nope": not a UUID`},
		{[]string{testListID, ""}, "contact ID or email cannot be empty"},
		{[]string{testListID, "person@example.com", "--subscription-status", ""}, "--subscription-status cannot be empty"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)
			_, err := testutil.RunPlain(t, NewSetCommand(), test.args...)
			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.False(t, *authCalled)
		})
	}
}

// remove ----------------------------------------------------------------------

func TestRemoveValidationHappensBeforePrompt(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"nope", "person@example.com"}, `invalid list ID value "nope": not a UUID`},
		{[]string{testListID, ""}, "contact ID or email cannot be empty"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)
			_, stderr, err := testutil.RunCommand(t, NewRemoveCommand(), "plain", "y\n", test.args...)
			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.NotContains(t, stderr, "Are you sure")
			assert.False(t, *authCalled)
		})
	}
}

func TestRemoveDeclinedMakesNoCall(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	stdout, stderr, err := testutil.RunCommand(t, NewRemoveCommand(), "plain", "\n", testListID, "person@example.com")
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Contains(t, stderr, "Are you sure you want to remove contact person@example.com from list "+testListID+"?")
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
}

func TestRemoveAcceptedPromptsOnStderrOnly(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteListContact", uuid.MustParse(testListID), "person@example.com").
		Return(&common.SuccessResponse{Message: "Contact removed from list"}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewRemoveCommand(), "plain", "y\n", testListID, "person@example.com")
	require.NoError(t, err)
	assert.Contains(t, stderr, "Are you sure")
	assert.Equal(t, "Contact removed from list\n", stdout)
	mockClient.AssertExpectations(t)
}

func TestRemoveForcedSkipsPrompt(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteListContact", uuid.MustParse(testListID), "person@example.com").
		Return(&common.SuccessResponse{}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewRemoveCommand(), "plain", "", testListID, "person@example.com", "--force")
	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Equal(t, "Contact 'person@example.com' removed from list '"+testListID+"' successfully\n", stdout)
	mockClient.AssertExpectations(t)
}
