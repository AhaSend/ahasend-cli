package printer

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func listMemberFixture() responses.ListContact {
	subscribedAt := time.Date(2026, time.September, 10, 8, 30, 0, 0, time.UTC)
	return responses.ListContact{
		Object:             "list_contact",
		ListID:             uuid.MustParse("22222222-2222-4222-8222-222222222222"),
		ContactID:          uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		Email:              "person@example.com",
		SubscriptionStatus: "confirmed",
		SubscribedAt:       &subscribedAt,
		CreatedAt:          subscribedAt,
		UpdatedAt:          subscribedAt,
	}
}

// embeddedMembersFixture returns a page whose first member embeds its list
// and contact, and whose second embeds nothing.
func embeddedMembersFixture() *responses.PaginatedListContactsResponse {
	embedded := listMemberFixture()
	embedded.List = &responses.EmbeddedContactList{Object: "list", ID: embedded.ListID, Name: "Newsletter", Tags: []string{}}
	embedded.Contact = &responses.Contact{Object: "contact", ID: embedded.ContactID, Email: embedded.Email, FirstName: "Pat", LastName: "Lee"}
	plain := listMemberFixture()
	plain.Email = "other@example.com"
	next := "next-member"
	return &responses.PaginatedListContactsResponse{
		Object:     "list",
		Data:       []responses.ListContact{embedded, plain},
		Pagination: responses.ContactPagination{HasMore: true, NextCursor: &next},
	}
}

// batchMembersFixture returns a batch add result in the API's shape: a
// member given by email holds the resolved contact ID in its membership
// alone, and one given by ID holds the ID.
func batchMembersFixture() *responses.BatchAddListContactsResponse {
	member := listMemberFixture()
	id := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	return &responses.BatchAddListContactsResponse{
		Object:  "list",
		Added:   1,
		Skipped: 0,
		Failed:  2,
		Data: []responses.BatchListContactResult{
			{Position: 0, Email: "person@example.com", Outcome: "added", Membership: &member},
			{Position: 1, ID: &id, Outcome: "not_found", Reason: "contact not found"},
			{Position: 2, Outcome: "invalid", Reason: "name exactly one of email or id"},
		},
	}
}

func TestListMemberJSONDecodesIntoSDKTypes(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleListMemberList(embeddedMembersFixture(), ListConfig{}))
		var decoded responses.PaginatedListContactsResponse
		require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
		assert.Equal(t, *embeddedMembersFixture(), decoded)
	})
	t.Run("single", func(t *testing.T) {
		member := listMemberFixture()
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleSingleListMember(&member, SingleConfig{}))
		var decoded responses.ListContact
		require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
		assert.Equal(t, member, decoded)
		assert.NotContains(t, output.String(), `"list"`, "an unset embedded list is left out")
	})
	t.Run("batch", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleBatchListMembers(batchMembersFixture(), SingleConfig{}))
		var decoded responses.BatchAddListContactsResponse
		require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
		assert.Equal(t, *batchMembersFixture(), decoded)
	})
}

func TestListMemberTableShowsTheSummaryColumnsInPlainOrder(t *testing.T) {
	useLocalTimeZone(t)
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleListMemberList(embeddedMembersFixture(), ListConfig{SuccessMessage: "Members"}))
	text := output.String()
	assert.Equal(t, [][]string{
		{"EMAIL", "CONTACT ID", "FIRST NAME", "LAST NAME", "SUBSCRIPTION STATUS", "SUBSCRIBED AT"},
		{"person@example.com", "11111111-1111-4111-8111-111111111111", "Pat", "Lee", "confirmed", "2026-09-10 10:30:00 UTC+2"},
		{"other@example.com", "11111111-1111-4111-8111-111111111111", "-", "-", "confirmed", "2026-09-10 10:30:00 UTC+2"},
	}, tableCells(text))
	assert.True(t, strings.HasSuffix(text, "--after next-member\n"), text)
}

func TestListMemberShowingLineNamesTheItemsTheCommandGives(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			handler := testHandler(t, format, &output, os.Stderr)
			require.NoError(t, handler.HandleListMemberList(embeddedMembersFixture(), ListConfig{ItemName: "members"}))
			assert.Contains(t, output.String(), "\nShowing 2 members (more available)\n")

			output.Reset()
			member := listMemberFixture()
			members := &responses.PaginatedListContactsResponse{Object: "list", Data: []responses.ListContact{member}}
			require.NoError(t, handler.HandleListMemberList(members, ListConfig{ItemName: "lists"}))
			assert.Contains(t, output.String(), "\nShowing 1 list\n")
		})
	}
}

// TestListMemberDetailShowsEveryRowInTableAndSetFieldsInPlain checks the
// membership lists members set writes, which embeds neither the list nor the
// contact, so neither the list name nor the contact's names are shown.
func TestListMemberDetailShowsEveryRowInTableAndSetFieldsInPlain(t *testing.T) {
	useLocalTimeZone(t)
	member := listMemberFixture()
	var plain, table bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &plain, os.Stderr).HandleSingleListMember(&member, SingleConfig{SuccessMessage: "Member"}))
	require.NoError(t, testHandler(t, "table", &table, os.Stderr).HandleSingleListMember(&member, SingleConfig{SuccessMessage: "Member"}))
	assert.Equal(t, "Member\n"+
		"Email: person@example.com\nContact ID: 11111111-1111-4111-8111-111111111111\n"+
		"List ID: 22222222-2222-4222-8222-222222222222\nSubscription Status: confirmed\n"+
		"Subscribed At: 2026-09-10 10:30:00 UTC+2\nCreated: 2026-09-10 10:30:00 UTC+2\nUpdated: 2026-09-10 10:30:00 UTC+2\n",
		plain.String())
	assert.Equal(t, [][]string{
		{"FIELD", "VALUE"},
		{"Email", "person@example.com"},
		{"Contact ID", "11111111-1111-4111-8111-111111111111"},
		{"List ID", "22222222-2222-4222-8222-222222222222"},
		{"Subscription Status", "confirmed"},
		{"Subscribed At", "2026-09-10 10:30:00 UTC+2"},
		{"Unsubscribed At", "-"},
		{"Created", "2026-09-10 10:30:00 UTC+2"},
		{"Updated", "2026-09-10 10:30:00 UTC+2"},
	}, tableCells(table.String()))
}

func TestListMemberTableKeepsColumnsNothingEmbeds(t *testing.T) {
	member := listMemberFixture()
	response := &responses.PaginatedListContactsResponse{Object: "list", Data: []responses.ListContact{member}}
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleListMemberList(response, ListConfig{SuccessMessage: "Members"}))
	assert.Equal(t, []string{"EMAIL", "CONTACT ID", "FIRST NAME", "LAST NAME", "SUBSCRIPTION STATUS", "SUBSCRIBED AT"},
		tableCells(output.String())[0])
	assert.Equal(t, []string{"-", "-"}, tableCells(output.String())[1][2:4])
}

func TestListMemberPlainShowsEmbeddedNamesAndCursorLines(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleListMemberList(embeddedMembersFixture(), ListConfig{SuccessMessage: "Members"}))
	text := output.String()
	assert.Contains(t, text, "First Name: Pat\n")
	assert.Contains(t, text, "Last Name: Lee\n")
	assert.Contains(t, text, "Email: other@example.com\n")
	assert.NotContains(t, text, "List: ", "the members of one list do not name it")
	assert.True(t, strings.HasSuffix(text, "--after next-member\n"), text)
}

func TestListMemberCSVHasTheSameColumnsForEveryRow(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleListMemberList(embeddedMembersFixture(), ListConfig{}))
	headers, rows := readCSV(t, output.String())
	require.Len(t, rows, 2)
	assert.Equal(t, listMemberFields, headers)

	embedded := rows[0]
	assert.Equal(t, "Pat", embedded["first_name"])
	assert.Equal(t, "Lee", embedded["last_name"])

	plain := rows[1]
	assert.Equal(t, "other@example.com", plain["email"])
	assert.Empty(t, plain["first_name"])
	assert.Empty(t, plain["last_name"])
}

func TestListMemberSingleRendersInEveryFormat(t *testing.T) {
	for _, format := range GetSupportedFormats() {
		t.Run(format, func(t *testing.T) {
			member := listMemberFixture()
			var output bytes.Buffer
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleSingleListMember(&member, SingleConfig{SuccessMessage: "Member"}))
			assert.Contains(t, output.String(), "person@example.com")
			assert.Contains(t, output.String(), "confirmed")
		})
	}
}

func TestBatchListMembersTableShowsOneRowPerResult(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleBatchListMembers(batchMembersFixture(), SingleConfig{SuccessMessage: "Done"}))
	assert.Contains(t, output.String(), "Added: 1  Skipped: 0  Failed: 2")
	assert.Equal(t, [][]string{
		{"POSITION", "EMAIL", "CONTACT ID", "OUTCOME", "REASON"},
		{"0", "person@example.com", "11111111-1111-4111-8111-111111111111", "added", "-"},
		{"1", "-", "33333333-3333-4333-8333-333333333333", "not_found", "contact not found"},
		{"2", "-", "-", "invalid", "name exactly one of email or id"},
	}, tableCells(output.String()))
}

func TestBatchListMembersPlainShowsOneBlockPerResult(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleBatchListMembers(batchMembersFixture(), SingleConfig{SuccessMessage: "Done"}))
	assert.Equal(t, "Done\nAdded: 1\nSkipped: 0\nFailed: 2\n"+
		"\nResult 1:\n  Position: 0\n  Email: person@example.com\n  Contact ID: 11111111-1111-4111-8111-111111111111\n  Outcome: added\n"+
		"\nResult 2:\n  Position: 1\n  Contact ID: 33333333-3333-4333-8333-333333333333\n  Outcome: not_found\n"+
		"  Reason: contact not found\n"+
		"\nResult 3:\n  Position: 2\n  Outcome: invalid\n  Reason: name exactly one of email or id\n",
		output.String())
}

func TestBatchListMembersCSVWritesOneRowPerResult(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleBatchListMembers(batchMembersFixture(), SingleConfig{}))
	records, err := csv.NewReader(bytes.NewReader(output.Bytes())).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 4)
	assert.Equal(t, []string{"position", "email", "contact_id", "outcome", "reason"}, records[0])
	assert.Equal(t, []string{"0", "person@example.com", "11111111-1111-4111-8111-111111111111", "added", ""}, records[1])
	assert.Equal(t, []string{"1", "", "33333333-3333-4333-8333-333333333333", "not_found", "contact not found"}, records[2])
	assert.Equal(t, []string{"2", "", "", "invalid", "name exactly one of email or id"}, records[3])
}

// membershipWithList is a membership that carries its list, as contacts
// memberships prints it.
func membershipWithList() responses.ListContact {
	return responses.ListContact{
		Email:              "member@example.com",
		ContactID:          uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		ListID:             uuid.MustParse("22222222-2222-4222-8222-222222222222"),
		SubscriptionStatus: "confirmed",
		List:               &responses.EmbeddedContactList{Name: "Newsletter"},
	}
}

// TestContactMembershipsShowTheListFirst checks that every view of contacts
// memberships names each list before its ID, like every other view names an
// item before its ID, and shows no contact names, which the API does not
// embed there.
func TestContactMembershipsShowTheListFirst(t *testing.T) {
	useLocalTimeZone(t)
	member := membershipWithList()
	page := func(h ResponseHandler) error {
		return h.HandleContactMembershipList(&responses.PaginatedListContactsResponse{Data: []responses.ListContact{member}}, ListConfig{})
	}

	assert.Equal(t, [][]string{
		{"LIST", "LIST ID", "SUBSCRIPTION STATUS", "SUBSCRIBED AT", "UNSUBSCRIBED AT"},
		{"Newsletter", "22222222-2222-4222-8222-222222222222", "confirmed", "-", "-"},
	}, tableCells(renderWith(t, "table", page)))
	plain := renderWith(t, "plain", page)
	assert.True(t, strings.HasPrefix(plain, "\nList: Newsletter\nList ID: 22222222-2222-4222-8222-222222222222\n"), plain)
	assert.NotContains(t, plain, "First Name")
	headers, rows := readCSV(t, renderWith(t, "csv", page))
	assert.Equal(t, contactMembershipFields, headers)
	assert.Equal(t, "Newsletter", rows[0]["list_name"])
}
