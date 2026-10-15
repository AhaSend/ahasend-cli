package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

const (
	intContactID    = "66666666-6666-4666-8666-666666666666"
	intContactEmail = "person@example.com"
)

// ContactsIntegrationTestSuite drives the contacts commands end to end through
// cmd.NewRootCmdForTesting against a MockClient wired in via the auth resolver.
type ContactsIntegrationTestSuite struct {
	commandSuite
}

func fixedContact() *responses.Contact {
	return &responses.Contact{
		Object:           "contact",
		ID:               uuid.MustParse(intContactID),
		CreatedAt:        time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:        time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		Email:            intContactEmail,
		FirstName:        "Pat",
		Status:           "enabled",
		Attributes:       map[string]any{"plan": "pro"},
		ValidationStatus: "valid",
	}
}

func fixedContactsPage() *responses.PaginatedContactsResponse {
	next := "next-contacts"
	return &responses.PaginatedContactsResponse{
		Object:     "list",
		Data:       []responses.Contact{*fixedContact()},
		Pagination: responses.ContactPagination{HasMore: true, NextCursor: &next},
	}
}

// TestContactWorkflow creates, gets, lists and deletes a contact, and lists
// its list memberships.
func (suite *ContactsIntegrationTestSuite) TestContactWorkflow() {
	firstName := "Pat"
	suite.mockClient.On("CreateContact", requests.CreateContactRequest{
		Email:      intContactEmail,
		FirstName:  &firstName,
		Attributes: map[string]any{"plan": "pro"},
	}).Return(fixedContact(), nil)
	suite.mockClient.On("GetContact", intContactEmail).Return(fixedContact(), nil)
	suite.mockClient.On("GetContacts", requests.GetContactsParams{}).Return(fixedContactsPage(), nil)
	suite.mockClient.On("GetContactLists", intContactEmail, requests.GetContactListsParams{}).
		Return(&responses.PaginatedListContactsResponse{Object: "list"}, nil)
	suite.mockClient.On("DeleteContact", intContactEmail).Return(&common.SuccessResponse{Message: "Contact deleted"}, nil)
	called := testutil.InjectClient(suite.T(), suite.mockClient)

	out, _, code := execRoot("plain", "", "contacts", "create", intContactEmail,
		"--first-name", firstName, "--attributes", `{"plan":"pro"}`)
	suite.Zero(code)
	suite.Contains(out, "Contact created successfully")
	suite.Contains(out, intContactID)

	out, _, code = execRoot("plain", "", "contacts", "get", intContactEmail)
	suite.Zero(code)
	suite.Contains(out, `Attributes: {"plan":"pro"}`)

	out, _, code = execRoot("table", "", "contacts", "list")
	suite.Zero(code)
	suite.Contains(out, intContactEmail)
	suite.True(strings.HasSuffix(out, "--after next-contacts\n"), out)

	out, _, code = execRoot("plain", "", "contacts", "memberships", intContactEmail)
	suite.Zero(code)
	suite.Contains(out, "No lists found")

	out, stderr, code := execRoot("plain", "y\n", "contacts", "delete", intContactEmail)
	suite.Zero(code)
	suite.Equal("Contact deleted\n", out)
	suite.Contains(stderr, "Are you sure you want to delete contact "+intContactEmail+"?")

	suite.True(*called)
	suite.mockClient.AssertExpectations(suite.T())
}

// TestValidationFailsBeforeAuth checks that local validation errors never
// reach the auth resolver.
func (suite *ContactsIntegrationTestSuite) TestValidationFailsBeforeAuth() {
	cases := []struct {
		name    string
		args    []string
		message string
	}{
		{"update without fields", []string{"contacts", "update", intContactEmail}, "at least one of --email"},
		{"create with null attribute", []string{"contacts", "create", intContactEmail, "--attributes", `{"a":null}`}, `attribute \"a\" must not be null`},
		{"batch without file", []string{"contacts", "batch"}, "--file is required"},
		{"list with both cursors", []string{"contacts", "list", "--after", "a", "--before", "b"}, "--after and --before cannot be used together"},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			called := testutil.InjectClient(suite.T(), &mocks.MockClient{})
			out, stderr, code := execRoot("json", "", tc.args...)
			suite.Contains(out, tc.message)
			suite.NotContains(stderr, "Usage:")
			suite.NotZero(code)
			suite.False(*called)
		})
	}
}

// TestJSONRoundTrip checks that JSON output decodes into the SDK types
// unchanged.
func (suite *ContactsIntegrationTestSuite) TestJSONRoundTrip() {
	suite.mockClient.On("GetContact", intContactID).Return(fixedContact(), nil)
	suite.mockClient.On("GetContacts", requests.GetContactsParams{}).Return(fixedContactsPage(), nil)
	testutil.InjectClient(suite.T(), suite.mockClient)

	out, _, code := execRoot("json", "", "contacts", "get", intContactID)
	suite.Zero(code)
	var contact responses.Contact
	suite.Require().NoError(json.Unmarshal([]byte(out), &contact))
	suite.Equal(*fixedContact(), contact)

	out, _, code = execRoot("json", "", "contacts", "list")
	suite.Zero(code)
	var page responses.PaginatedContactsResponse
	suite.Require().NoError(json.Unmarshal([]byte(out), &page))
	suite.Equal(*fixedContactsPage(), page)

	suite.mockClient.AssertExpectations(suite.T())
}

// TestDeleteJSONKeepsPromptOffStdout checks that the delete prompt leaves
// stdout valid JSON.
func (suite *ContactsIntegrationTestSuite) TestDeleteJSONKeepsPromptOffStdout() {
	suite.mockClient.On("DeleteContact", intContactEmail).Return(&common.SuccessResponse{Message: "Contact deleted"}, nil)
	testutil.InjectClient(suite.T(), suite.mockClient)

	out, stderr, code := execRoot("json", "yes\n", "contacts", "delete", intContactEmail)
	suite.Zero(code)
	suite.JSONEq(`{"message":"Contact deleted"}`, out)
	suite.Contains(stderr, "Are you sure")
	suite.mockClient.AssertExpectations(suite.T())
}

// TestAPIErrorPassThrough checks that JSON mode prints the API's error body
// unchanged and that every mode exits nonzero.
func (suite *ContactsIntegrationTestSuite) TestAPIErrorPassThrough() {
	raw := `{"message":"contact not found"}`
	for _, format := range []string{"json", "table", "plain"} {
		suite.Run(format, func() {
			mockClient := &mocks.MockClient{}
			mockClient.On("GetContact", intContactEmail).Return(nil, &api.APIError{
				StatusCode: 404,
				Message:    "contact not found",
				Raw:        []byte(raw),
			})
			testutil.InjectClient(suite.T(), mockClient)

			out, stderr, code := execRoot(format, "", "contacts", "get", intContactEmail)
			if format == "json" {
				suite.JSONEq(raw, out)
			} else {
				suite.Empty(out)
				suite.Contains(stderr, "Error:")
			}
			suite.NotZero(code)
			mockClient.AssertExpectations(suite.T())
		})
	}
}

// TestBatchPartialFailurePrintsResultAndExitsOne checks that a batch the API
// answered with failed items prints the whole result and exits 1, with one
// stderr line outside JSON mode and nothing but the API response in it.
func (suite *ContactsIntegrationTestSuite) TestBatchPartialFailurePrintsResultAndExitsOne() {
	path := filepath.Join(suite.T().TempDir(), "contacts.json")
	suite.Require().NoError(os.WriteFile(path, []byte(`{"data":[{"email":"a@example.com"},{"email":"bad"}]}`), 0o600))
	response := &responses.BatchUpsertContactsResponse{
		Object:  "list",
		Created: 1,
		Failed:  1,
		Data: []responses.BatchContactResult{
			{Position: 0, Email: "a@example.com", Outcome: "created", Contact: fixedContact()},
			{Position: 1, Email: "bad", Outcome: "failed", Reason: "invalid email"},
		},
	}
	suite.mockClient.On("BatchUpsertContacts", mock.AnythingOfType("requests.BatchUpsertContactsRequest")).Return(response, nil)
	testutil.InjectClient(suite.T(), suite.mockClient)

	out, stderr, code := execRoot("plain", "", "contacts", "batch", "--file", path)
	suite.Contains(out, "Reason: invalid email")
	suite.Equal("1 of 2 contacts failed\n", stderr)
	suite.Equal(1, code)

	out, stderr, code = execRoot("json", "", "contacts", "batch", "--file", path)
	expected, marshalErr := json.Marshal(response)
	suite.Require().NoError(marshalErr)
	suite.JSONEq(string(expected), out)
	suite.Empty(stderr)
	suite.Equal(1, code)
	suite.mockClient.AssertExpectations(suite.T())
}

func TestContactsIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(ContactsIntegrationTestSuite))
}
