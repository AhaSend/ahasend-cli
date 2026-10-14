package contacts

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// list ------------------------------------------------------------------------

func TestListRejectsBothCursorsBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewListCommand(), "--after", "next", "--before", "previous")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--after and --before")
	assert.False(t, *authCalled)
}

func TestListForwardsPaginationAndAllFilters(t *testing.T) {
	from := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	limit := int32(25)
	after := "next"
	email := "person@example.com"
	status := "disabled"
	subscribed := true
	expected := requests.GetContactsParams{
		Limit:      &limit,
		After:      &after,
		Email:      &email,
		Status:     &status,
		Subscribed: &subscribed,
		FromTime:   &from,
		ToTime:     &to,
	}

	mockClient := &mocks.MockClient{}
	mockClient.On("GetContacts", expected).Return(&responses.PaginatedContactsResponse{
		Object: "list",
		Data:   []responses.Contact{{Email: email, Status: status}},
	}, nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewListCommand(),
		"--limit", "25", "--after", after, "--email", email, "--status", status,
		"--subscribed", "--from-time", from.Format(time.RFC3339), "--to-time", to.Format(time.RFC3339))
	require.NoError(t, err)
	assert.Contains(t, output, email)
	mockClient.AssertExpectations(t)
}

func TestListSubscribedFalseSendsFalse(t *testing.T) {
	subscribed := false
	mockClient := &mocks.MockClient{}
	mockClient.On("GetContacts", requests.GetContactsParams{Subscribed: &subscribed}).
		Return(&responses.PaginatedContactsResponse{Object: "list"}, nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewListCommand(), "--subscribed=false")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestListWithoutSubscribedSendsNoFilter(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("GetContacts", requests.GetContactsParams{}).
		Return(&responses.PaginatedContactsResponse{Object: "list"}, nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewListCommand())
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestListForwardsBeforeCursor(t *testing.T) {
	before := "previous"
	mockClient := &mocks.MockClient{}
	mockClient.On("GetContacts", requests.GetContactsParams{Before: &before}).
		Return(&responses.PaginatedContactsResponse{Object: "list"}, nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewListCommand(), "--before", before)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestListSendsStatusAndEmail(t *testing.T) {
	email := "Person@Example.com"
	status := "blocked"
	mockClient := &mocks.MockClient{}
	mockClient.On("GetContacts", requests.GetContactsParams{Email: &email, Status: &status}).
		Return(&responses.PaginatedContactsResponse{Object: "list"}, nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewListCommand(), "--email", email, "--status", status)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestListValidationHappensBeforeAuthentication(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"--from-time", "yesterday"}, `invalid --from-time value "yesterday": use RFC 3339 or a relative time such as 24h or 7d`},
		{[]string{"--to-time", "tomorrow"}, `invalid --to-time value "tomorrow": use RFC 3339 or a relative time such as 24h or 7d`},
		{[]string{"--limit", "101"}, "--limit must be between 0 and 100"},
		{[]string{"--status", "active"}, "--status must be one of: enabled, disabled, blocked"},
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

func TestListAcceptsRelativeTimes(t *testing.T) {
	mockClient := &mocks.MockClient{}
	var params requests.GetContactsParams
	mockClient.On("GetContacts", mock.AnythingOfType("requests.GetContactsParams")).
		Run(func(args mock.Arguments) { params = args.Get(0).(requests.GetContactsParams) }).
		Return(&responses.PaginatedContactsResponse{Object: "list"}, nil)
	testutil.InjectClient(t, mockClient)

	before := time.Now()
	_, err := testutil.RunPlain(t, NewListCommand(), "--from-time", "7d", "--to-time", "24h")
	require.NoError(t, err)
	require.NotNil(t, params.FromTime)
	require.NotNil(t, params.ToTime)
	assert.WithinDuration(t, before.AddDate(0, 0, -7), *params.FromTime, time.Minute)
	assert.WithinDuration(t, before.Add(-24*time.Hour), *params.ToTime, time.Minute)
}

func TestListInvalidTimeUsesMessagesListWording(t *testing.T) {
	mockClient := &mocks.MockClient{}
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewListCommand(), "--from-time", "yesterday")
	require.Error(t, err)
	assert.Equal(t, `invalid --from-time value "yesterday": use RFC 3339 or a relative time such as 24h or 7d`, err.Error())
}

func TestListPrintsCursorLines(t *testing.T) {
	next := "next-page"
	previous := "previous-page"
	mockClient := &mocks.MockClient{}
	mockClient.On("GetContacts", requests.GetContactsParams{}).Return(&responses.PaginatedContactsResponse{
		Object:     "list",
		Data:       []responses.Contact{{Email: "person@example.com"}},
		Pagination: responses.ContactPagination{HasMore: true, NextCursor: &next, PreviousCursor: &previous},
	}, nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewListCommand())
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(output, "--after next-page\n--before previous-page\n"), output)
}

// get -------------------------------------------------------------------------

func TestGetForwardsRawEmail(t *testing.T) {
	rawEmail := "User+Tag/Segment@Example.COM"
	mockClient := &mocks.MockClient{}
	mockClient.On("GetContact", rawEmail).Return(&responses.Contact{Email: rawEmail}, nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewGetCommand(), rawEmail)
	require.NoError(t, err)
	assert.Contains(t, output, rawEmail)
	mockClient.AssertExpectations(t)
}

func TestGetRejectsEmptyIdentifierBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewGetCommand(), "")
	require.Error(t, err)
	assert.Equal(t, "contact ID or email cannot be empty", err.Error())
	assert.False(t, *authCalled)
}

// create ----------------------------------------------------------------------

func TestCreateParsesAttributesPreservesFalseAndForwardsIdempotency(t *testing.T) {
	firstName := "Pat"
	status := "enabled"
	unsubscribed := false
	expected := requests.CreateContactRequest{
		Email:        "person@example.com",
		FirstName:    &firstName,
		Status:       &status,
		Unsubscribed: &unsubscribed,
		Attributes:   map[string]any{"score": json.Number("7"), "vip": true},
	}
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateContact", expected, testutil.IdempotencyKeyArg("create-key")).
		Return(&responses.Contact{Email: expected.Email}, nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewCreateCommand(), expected.Email,
		"--first-name", firstName, "--status", status, "--unsubscribed=false",
		"--attributes", `{"score":7,"vip":true}`, "--idempotency-key", "create-key")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

// TestEmailsAreCheckedBeforeAuthentication checks that each contact command
// that takes an email address refuses one that is not an address, or is
// blank, before authenticating. The filters match the address exactly, so
// they take only an address too.
func TestEmailsAreCheckedBeforeAuthentication(t *testing.T) {
	tests := []struct {
		name    string
		command func() *cobra.Command
		args    []string
		message string
	}{
		{"create", NewCreateCommand, []string{"not-an-email"}, `invalid email value "not-an-email": not an email address`},
		{"create blank", NewCreateCommand, []string{"  "}, "email cannot be empty"},
		{"update", NewUpdateCommand, []string{"person@example.com", "--email", "Jane <jane@example.com>"},
			`invalid --email value "Jane <jane@example.com>": not an email address`},
		{"list", NewListCommand, []string{"--email", "@example.com"}, `invalid --email value "@example.com": not an email address`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)

			_, err := testutil.RunPlain(t, test.command(), test.args...)

			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.False(t, *authCalled)
		})
	}
}

func TestCreateRejectsNullAttributeBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewCreateCommand(), "person@example.com", "--attributes", `{"obsolete":null}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be null")
	assert.False(t, *authCalled)
}

func TestCreateRejectsInvalidAttributesBeforeAuthentication(t *testing.T) {
	notAnObject := "--attributes must be a JSON object"
	notOneObject := "--attributes must contain exactly one JSON object"
	for value, message := range map[string]string{
		`[]`:              notAnObject,
		`null`:            notAnObject,
		`not json`:        notAnObject,
		`{"a":1} {"b":2}`: notOneObject,
		`{"a":1} x`:       notOneObject,
		`{"a":1} [`:       notOneObject,
		``:                "--attributes is empty",
	} {
		t.Run(value, func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)

			_, err := testutil.RunPlain(t, NewCreateCommand(), "person@example.com", "--attributes", value)
			require.Error(t, err)
			assert.Equal(t, 4, errors.GetExitCode(err))
			assert.Equal(t, message, err.Error())
			assert.False(t, *authCalled)
		})
	}
}

func TestCreateSendsEveryFieldFlag(t *testing.T) {
	lastName := "Doe"
	statusReason := "imported"
	unsubscribed := true
	expected := requests.CreateContactRequest{
		Email:        "person@example.com",
		LastName:     &lastName,
		StatusReason: &statusReason,
		Unsubscribed: &unsubscribed,
	}
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateContact", expected).Return(&responses.Contact{Email: expected.Email}, nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewCreateCommand(), expected.Email,
		"--last-name", lastName, "--status-reason", statusReason, "--unsubscribed")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCreateRejectsEmptyEmailBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewCreateCommand(), "")
	require.Error(t, err)
	assert.Equal(t, "email cannot be empty", err.Error())
	assert.False(t, *authCalled)
}

// update ----------------------------------------------------------------------

func TestUpdateSendsNameAndStatusFlags(t *testing.T) {
	firstName := "Pat"
	status := "disabled"
	statusReason := "requested"
	expected := requests.UpdateContactRequest{
		FirstName:    &firstName,
		Status:       &status,
		StatusReason: &statusReason,
	}
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateContact", "person@example.com", expected).
		Return(&responses.Contact{Email: "person@example.com"}, nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), "person@example.com",
		"--first-name", firstName, "--status", status, "--status-reason", statusReason)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestUpdateValidationHappensBeforeAuthentication(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"", "--first-name", "Pat"}, "contact ID or email cannot be empty"},
		{[]string{"person@example.com", "--attributes", `[]`}, "--attributes must be a JSON object"},
		{[]string{"person@example.com", "--email", ""}, "--email cannot be empty"},
		{[]string{"person@example.com", "--status", ""}, "--status cannot be empty"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)
			_, err := testutil.RunPlain(t, NewUpdateCommand(), test.args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.message)
			assert.False(t, *authCalled)
		})
	}
}

func TestUpdateSendsOnlyChangedFieldsAndPreservesFalseAndNull(t *testing.T) {
	emptyLastName := ""
	unsubscribed := false
	expected := requests.UpdateContactRequest{
		LastName:     &emptyLastName,
		Unsubscribed: &unsubscribed,
		Attributes:   map[string]any{"obsolete": nil, "score": json.Number("9")},
	}
	rawEmail := "User+Tag@example.com"
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateContact", rawEmail, expected).Return(&responses.Contact{Email: rawEmail}, nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), rawEmail,
		"--last-name=", "--unsubscribed=false", "--attributes", `{"obsolete":null,"score":9}`)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestUpdateSendsTheNewEmail(t *testing.T) {
	email := "new@example.com"
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateContact", "person@example.com", requests.UpdateContactRequest{Email: &email}).
		Return(&responses.Contact{Email: email}, nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), "person@example.com", "--email", email)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestUpdateRequiresAChangedFlagBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), "person@example.com")
	require.Error(t, err)
	assert.Equal(t, "at least one of --email, --first-name, --last-name, --status, --status-reason, --unsubscribed or --attributes must be provided", err.Error())
	assert.False(t, *authCalled)
}

// delete ----------------------------------------------------------------------

func TestDeleteRejectsEmptyIdentifierBeforePrompt(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "y\n", "")
	require.Error(t, err)
	assert.Equal(t, "contact ID or email cannot be empty", err.Error())
	assert.NotContains(t, stderr, "Are you sure")
	assert.False(t, *authCalled)
}

func TestDeleteDeclinedMakesNoCall(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "no\n", "person@example.com")
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	for _, warning := range []string{
		"cancels all active workflows", "cancelled workflow audit rows are retained",
		"unsubscribe history", "membership history", "campaign history", "open history", "click history",
		"--unsubscribed=true",
	} {
		assert.Contains(t, stderr, warning)
	}
	assert.Contains(t, stderr, "\n\nAre you sure you want to delete contact person@example.com? (y/N): ")
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
}

func TestDeleteAcceptedPromptsOnStderrOnly(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteContact", "person@example.com").Return(&common.SuccessResponse{Message: "contact deleted"}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "yes\n", "person@example.com")
	require.NoError(t, err)
	assert.Contains(t, stderr, "Are you sure")
	assert.Equal(t, "contact deleted\n", stdout)
	mockClient.AssertExpectations(t)
}

func TestDeleteForcedSkipsPrompt(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteContact", "person@example.com").Return(&common.SuccessResponse{}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "", "person@example.com", "--force")
	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Equal(t, "Contact 'person@example.com' deleted successfully\n", stdout)
	mockClient.AssertExpectations(t)
}

func TestDeleteJSONStdoutIsTheResponse(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteContact", "person@example.com").Return(&common.SuccessResponse{Message: "contact deleted"}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "json", "y\n", "person@example.com")
	require.NoError(t, err)
	assert.Contains(t, stderr, "Are you sure")
	assert.JSONEq(t, `{"message":"contact deleted"}`, stdout)
	mockClient.AssertExpectations(t)
}

// batch -----------------------------------------------------------------------

func TestBatchStrictEnvelopeNullAttributeClearsAndIdempotency(t *testing.T) {
	path := testutil.WriteTempFile(t, "contacts.json", `{"data":[{"email":"one@example.com","unsubscribed":false,"attributes":{"obsolete":null}},{"email":"two@example.com"}]}`)
	unsubscribed := false
	expected := requests.BatchUpsertContactsRequest{Data: []requests.BatchUpsertContactInput{
		{Email: "one@example.com", Unsubscribed: &unsubscribed, Attributes: map[string]any{"obsolete": nil}},
		{Email: "two@example.com"},
	}}
	mockClient := &mocks.MockClient{}
	mockClient.On("BatchUpsertContacts", expected, testutil.IdempotencyKeyArg("batch-key")).
		Return(&responses.BatchUpsertContactsResponse{Object: "list", Data: []responses.BatchContactResult{
			{Position: 0, Email: "one@example.com", Outcome: "updated", Contact: &responses.Contact{Email: "one@example.com"}},
			{Position: 1, Email: "two@example.com", Outcome: "created", Contact: &responses.Contact{Email: "two@example.com"}},
		}}, nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewBatchCommand(), "--file", path, "--idempotency-key", "batch-key")
	require.NoError(t, err)
	assert.Contains(t, output, "one@example.com")
	assert.Contains(t, output, "two@example.com")
	mockClient.AssertExpectations(t)
}

func TestBatchNullContactFieldRejectedBeforeAuthentication(t *testing.T) {
	for _, field := range []string{"first_name", "last_name", "status", "status_reason", "unsubscribed", "attributes"} {
		t.Run(field, func(t *testing.T) {
			path := testutil.WriteTempFile(t, "contacts.json", `{"data":[{"email":"one@example.com"},{"email":"two@example.com","`+field+`":null}]}`)
			authCalled := testutil.RefuseAuth(t)

			_, err := testutil.RunPlain(t, NewBatchCommand(), "--file", path)
			require.Error(t, err)
			assert.Equal(t, "batch contact at position 1 sets "+field+" to null; leave the field out instead", err.Error())
			assert.False(t, *authCalled)
		})
	}
}

func TestBatchSendsDuplicatesUnknownStatusAndMalformedEmailUnchanged(t *testing.T) {
	path := testutil.WriteTempFile(t, "contacts.json", `{"data":[{"email":"One@example.com"},{"email":"one@example.com","status":"active"},{"email":"not-an-email"}]}`)
	status := "active"
	expected := requests.BatchUpsertContactsRequest{Data: []requests.BatchUpsertContactInput{
		{Email: "One@example.com"},
		{Email: "one@example.com", Status: &status},
		{Email: "not-an-email"},
	}}
	mockClient := &mocks.MockClient{}
	mockClient.On("BatchUpsertContacts", expected).
		Return(&responses.BatchUpsertContactsResponse{Object: "list", Data: []responses.BatchContactResult{
			{Position: 0, Email: "one@example.com", Outcome: "created"},
		}}, nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewBatchCommand(), "--file", path)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

// partialBatchResponse is a batch result in which one of two contacts failed.
const partialBatchResponse = `{"object":"list","created":1,"updated":0,"failed":1,"data":[` +
	`{"position":0,"email":"one@example.com","outcome":"created","contact":{"object":"contact","id":"11111111-1111-4111-8111-111111111111","email":"one@example.com",` +
	`"first_name":"","last_name":"","status":"enabled","status_reason":"","unsubscribed":false,"unsubscribed_at":null,` +
	`"validation_status":"","last_validated_at":null,"attributes":{},"created_at":"2026-09-01T10:00:00Z","updated_at":"2026-09-01T10:00:00Z"}},` +
	`{"position":1,"email":"bad@example.com","outcome":"failed","reason":"email is suppressed"}]}`

func partialBatchMock(t *testing.T) {
	t.Helper()
	var response responses.BatchUpsertContactsResponse
	require.NoError(t, json.Unmarshal([]byte(partialBatchResponse), &response))
	mockClient := &mocks.MockClient{}
	mockClient.On("BatchUpsertContacts", mock.AnythingOfType("requests.BatchUpsertContactsRequest")).Return(&response, nil)
	testutil.InjectClient(t, mockClient)
}

func TestBatchPlainPrintsResultAndFailsWhenAnItemFailed(t *testing.T) {
	path := testutil.WriteTempFile(t, "contacts.json", `{"data":[{"email":"one@example.com"},{"email":"bad@example.com"}]}`)
	partialBatchMock(t)

	output, err := testutil.RunPlain(t, NewBatchCommand(), "--file", path)
	var itemsFailed *clierrors.ItemsFailedError
	require.ErrorAs(t, err, &itemsFailed)
	assert.Equal(t, "1 of 2 contacts failed", err.Error())
	assert.Contains(t, output, "Created: 1")
	assert.Contains(t, output, "Failed: 1")
	assert.Contains(t, output, "  Email: bad@example.com\n  Outcome: failed\n  Reason: email is suppressed\n")
}

func TestBatchJSONPrintsOnlyTheResponseWhenAnItemFailed(t *testing.T) {
	path := testutil.WriteTempFile(t, "contacts.json", `{"data":[{"email":"one@example.com"},{"email":"bad@example.com"}]}`)
	partialBatchMock(t)

	stdout, _, err := testutil.RunCommand(t, NewBatchCommand(), "json", "", "--file", path)
	var itemsFailed *clierrors.ItemsFailedError
	require.ErrorAs(t, err, &itemsFailed)
	assert.JSONEq(t, partialBatchResponse, stdout)
}

func TestBatchRejectsInvalidShapesBeforeAuthentication(t *testing.T) {
	tests := map[string]struct {
		contents string
		message  string
	}{
		"array envelope":     {`[]`, `batch file must be a JSON object, {"data": [...]}; CSV is not read`},
		"CSV file":           {"email\nperson@example.com\n", `batch file must be a JSON object, {"data": [...]}; CSV is not read`},
		"empty file":         {``, "batch file is empty"},
		"missing data":       {`{"contacts":[]}`, "batch file data must be a JSON array"},
		"extra envelope key": {`{"data":[{"email":"one@example.com"}],"extra":true}`, "batch file must contain only a data field"},
		"unknown item field": {`{"data":[{"email":"one@example.com","list_id":"x"}]}`,
			`batch contact at position 0 is invalid: unknown field "list_id"`},
		"wrong field type": {`{"data":[{"email":5}]}`, "batch contact at position 0 is invalid: email must be a string"},
		"no contacts":      {`{"data":[]}`, "batch file data cannot be empty"},
		"null data":        {`{"data":null}`, "batch file data must be a JSON array"},
		"null contact":     {`{"data":[null]}`, "batch contact at position 0 is null"},
		"too many contacts": {
			`{"data":[` + strings.Repeat(`{"email":"one@example.com"},`, 1000) + `{"email":"one@example.com"}]}`,
			"batch file data takes at most 1000 contacts, got 1001",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			path := testutil.WriteTempFile(t, "contacts.json", test.contents)
			authCalled := testutil.RefuseAuth(t)
			_, err := testutil.RunPlain(t, NewBatchCommand(), "--file", path)
			var cliErr *clierrors.CLIError
			require.ErrorAs(t, err, &cliErr)
			assert.Equal(t, test.message, cliErr.Message)
			assert.False(t, *authCalled)
		})
	}
}

func TestBatchWithoutFileRejectedBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewBatchCommand())
	require.Error(t, err)
	assert.Equal(t, "--file is required", err.Error())
	assert.False(t, *authCalled)
}

func TestBatchMissingFileRejectedBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewBatchCommand(), "--file", filepath.Join(t.TempDir(), "missing.json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read --file")
	assert.False(t, *authCalled)
}

// memberships -----------------------------------------------------------------

// TestMembershipsHasNoListsAlias checks that only memberships names the
// command: the contacts group is new, so no one used the lists alias.
func TestMembershipsHasNoListsAlias(t *testing.T) {
	command, _, err := NewCommand().Find([]string{"memberships"})
	require.NoError(t, err)
	assert.Equal(t, "memberships", command.Name())

	command, _, _ = NewCommand().Find([]string{"lists"})
	assert.NotEqual(t, "memberships", command.Name())
}

func TestMembershipsPassesStatusFilterAndPagination(t *testing.T) {
	status := "unsubscribed"
	limit := int32(10)
	mockClient := &mocks.MockClient{}
	mockClient.On("GetContactLists", "person@example.com",
		requests.GetContactListsParams{Limit: &limit, SubscriptionStatus: &status}).
		Return(&responses.PaginatedListContactsResponse{Object: "list"}, nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewMembershipsCommand(), "person@example.com", "--subscription-status", status, "--limit", "10")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestMembershipsShowsListNames(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("GetContactLists", "person@example.com", requests.GetContactListsParams{}).
		Return(&responses.PaginatedListContactsResponse{
			Object: "list",
			Data: []responses.ListContact{{
				ListID:             uuid.MustParse("22222222-2222-4222-8222-222222222222"),
				Email:              "person@example.com",
				SubscriptionStatus: "confirmed",
				List:               &responses.EmbeddedContactList{Name: "Newsletter"},
			}},
		}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, _, err := testutil.RunCommand(t, NewMembershipsCommand(), "table", "", "person@example.com")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Newsletter")
	assert.Contains(t, stdout, "confirmed")
	assert.True(t, strings.HasSuffix(stdout, "\nShowing 1 list\n"), stdout)
	mockClient.AssertExpectations(t)
}

func TestMembershipsValidationHappensBeforeAuthentication(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{""}, "contact ID or email cannot be empty"},
		{[]string{"person@example.com", "--after", "a", "--before", "b"}, "--after and --before cannot be used together"},
		{[]string{"person@example.com", "--limit", "-1"}, "--limit must be between 0 and 100"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)
			_, err := testutil.RunPlain(t, NewMembershipsCommand(), test.args...)
			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.False(t, *authCalled)
		})
	}
}
