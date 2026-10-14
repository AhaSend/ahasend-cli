package integration

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

const intListID = "77777777-7777-4777-8777-777777777777"

// ListsIntegrationTestSuite drives the lists and lists members commands end to
// end through cmd.NewRootCmdForTesting against a MockClient wired in via the
// auth resolver.
type ListsIntegrationTestSuite struct {
	commandSuite
}

func fixedList() *responses.ContactList {
	return &responses.ContactList{
		Object:       "list",
		ID:           uuid.MustParse(intListID),
		CreatedAt:    time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		Name:         "Newsletter",
		Description:  "Monthly news",
		Tags:         []string{"news"},
		ContactCount: 2,
	}
}

func fixedListsPage() *responses.PaginatedContactListsResponse {
	previous := "previous-lists"
	return &responses.PaginatedContactListsResponse{
		Object:     "list",
		Data:       []responses.ContactList{*fixedList()},
		Pagination: responses.ContactPagination{PreviousCursor: &previous},
	}
}

func fixedListMember() *responses.ListContact {
	subscribedAt := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	return &responses.ListContact{
		Object:             "list_contact",
		ListID:             uuid.MustParse(intListID),
		ContactID:          uuid.MustParse(intContactID),
		Email:              intContactEmail,
		SubscriptionStatus: "confirmed",
		SubscribedAt:       &subscribedAt,
		CreatedAt:          subscribedAt,
		UpdatedAt:          subscribedAt,
	}
}

func fixedListMembersPage() *responses.PaginatedListContactsResponse {
	member := fixedListMember()
	member.Contact = fixedContact()
	return &responses.PaginatedListContactsResponse{
		Object: "list",
		Data:   []responses.ListContact{*member},
	}
}

// TestListWorkflow creates, gets, lists and deletes a list.
func (suite *ListsIntegrationTestSuite) TestListWorkflow() {
	description := "Monthly news"
	suite.mockClient.On("CreateList", requests.CreateListRequest{
		Name:        "Newsletter",
		Description: &description,
		Tags:        []string{"news"},
	}).Return(fixedList(), nil)
	suite.mockClient.On("GetList", uuid.MustParse(intListID)).Return(fixedList(), nil)
	suite.mockClient.On("GetLists", requests.GetListsParams{}).Return(fixedListsPage(), nil)
	suite.mockClient.On("DeleteList", uuid.MustParse(intListID)).Return(&common.SuccessResponse{}, nil)
	called := testutil.InjectClient(suite.T(), suite.mockClient)

	out, _, code := execRoot("plain", "", "lists", "create", "--name", "Newsletter",
		"--description", description, "--tag", "news")
	suite.Zero(code)
	suite.Contains(out, "List created successfully")
	suite.Contains(out, intListID)

	out, _, code = execRoot("plain", "", "lists", "get", intListID)
	suite.Zero(code)
	suite.Contains(out, "Contacts: 2")

	out, _, code = execRoot("table", "", "lists", "list")
	suite.Zero(code)
	suite.Contains(out, "Newsletter")
	suite.True(strings.HasSuffix(out, "--before previous-lists\n"), out)

	out, stderr, code := execRoot("plain", "", "lists", "delete", intListID, "--force")
	suite.Zero(code)
	suite.Equal("List '"+intListID+"' deleted successfully\n", out)
	suite.Empty(stderr)

	suite.True(*called)
	suite.mockClient.AssertExpectations(suite.T())
}

// TestMembersWorkflow adds, lists, updates and removes a list member.
func (suite *ListsIntegrationTestSuite) TestMembersWorkflow() {
	listID := uuid.MustParse(intListID)
	email := intContactEmail
	unsubscribed := "unsubscribed"
	include := true
	suite.mockClient.On("BatchAddListContacts", listID, requests.BatchAddListContactsRequest{
		Data: []requests.BatchAddListContactInput{{Email: &email}},
	}).Return(&responses.BatchAddListContactsResponse{
		Object: "list",
		Added:  1,
		Data: []responses.BatchListContactResult{
			{Position: 0, Email: email, Outcome: "added", Membership: fixedListMember()},
		},
	}, nil)
	suite.mockClient.On("GetListContacts", listID, requests.GetListContactsParams{IncludeContacts: &include}).
		Return(fixedListMembersPage(), nil)
	suite.mockClient.On("UpsertListContact", listID, email, requests.UpsertListContactRequest{SubscriptionStatus: &unsubscribed}).
		Return(fixedListMember(), nil)
	suite.mockClient.On("DeleteListContact", listID, email).
		Return(&common.SuccessResponse{Message: "Contact removed from list"}, nil)
	testutil.InjectClient(suite.T(), suite.mockClient)

	out, _, code := execRoot("plain", "", "lists", "members", "add", intListID, "--email", email)
	suite.Zero(code)
	suite.Contains(out, "Added: 1")

	out, _, code = execRoot("table", "", "lists", "members", "list", intListID, "--include-contacts")
	suite.Zero(code)
	suite.Contains(out, "Pat")

	out, _, code = execRoot("plain", "", "lists", "members", "set", intListID, email,
		"--subscription-status", unsubscribed)
	suite.Zero(code)
	suite.Contains(out, "List member saved successfully")

	out, stderr, code := execRoot("plain", "y\n", "lists", "members", "remove", intListID, email)
	suite.Zero(code)
	suite.Equal("Contact removed from list\n", out)
	suite.Contains(stderr, "Are you sure")

	suite.mockClient.AssertExpectations(suite.T())
}

// TestValidationFailsBeforeAuth checks that local validation errors never
// reach the auth resolver.
func (suite *ListsIntegrationTestSuite) TestValidationFailsBeforeAuth() {
	cases := []struct {
		name    string
		args    []string
		message string
	}{
		{"get bad list ID", []string{"lists", "get", "nope"}, `invalid list ID value "nope": not a UUID`},
		{"update tag and clear tags", []string{"lists", "update", intListID, "--tag", "a", "--clear-tags"}, "--tag and --clear-tags cannot be used together"},
		{"members add no input", []string{"lists", "members", "add", intListID}, "one of --email, --contact-id or --file must be provided"},
		{"members remove empty contact", []string{"lists", "members", "remove", intListID, "", "--force"}, "contact ID or email cannot be empty"},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			called := testutil.InjectClient(suite.T(), &mocks.MockClient{})
			out, stderr, code := execRoot("plain", "", tc.args...)
			suite.Empty(out)
			suite.Contains(stderr, tc.message)
			suite.NotContains(stderr, "Usage:")
			suite.NotZero(code)
			suite.False(*called)
		})
	}
}

// TestJSONRoundTrip checks that JSON output decodes into the SDK types
// unchanged.
func (suite *ListsIntegrationTestSuite) TestJSONRoundTrip() {
	suite.mockClient.On("GetList", uuid.MustParse(intListID)).Return(fixedList(), nil)
	suite.mockClient.On("GetListContacts", uuid.MustParse(intListID), requests.GetListContactsParams{}).
		Return(fixedListMembersPage(), nil)
	testutil.InjectClient(suite.T(), suite.mockClient)

	out, _, code := execRoot("json", "", "lists", "get", intListID)
	suite.Zero(code)
	var list responses.ContactList
	suite.Require().NoError(json.Unmarshal([]byte(out), &list))
	suite.Equal(*fixedList(), list)

	out, _, code = execRoot("json", "", "lists", "members", "list", intListID)
	suite.Zero(code)
	var page responses.PaginatedListContactsResponse
	suite.Require().NoError(json.Unmarshal([]byte(out), &page))
	suite.Equal(*fixedListMembersPage(), page)

	suite.mockClient.AssertExpectations(suite.T())
}

// TestAPIErrorPassThrough checks that JSON mode prints the API's error body
// unchanged and that every mode exits nonzero.
func (suite *ListsIntegrationTestSuite) TestAPIErrorPassThrough() {
	raw := `{"message":"list is used by a campaign"}`
	for _, format := range []string{"json", "table", "plain"} {
		suite.Run(format, func() {
			mockClient := &mocks.MockClient{}
			mockClient.On("DeleteList", uuid.MustParse(intListID)).Return(nil, &api.APIError{
				StatusCode: 409,
				Message:    "list is used by a campaign",
				Raw:        []byte(raw),
			})
			testutil.InjectClient(suite.T(), mockClient)

			out, stderr, code := execRoot(format, "", "lists", "delete", intListID, "--force")
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

func TestListsIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(ListsIntegrationTestSuite))
}
