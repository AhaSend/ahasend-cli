package printer

import (
	"bytes"
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

func listFixture() responses.ContactList {
	createdAt := time.Date(2026, time.September, 10, 8, 30, 0, 0, time.UTC)
	return responses.ContactList{
		Object:       "list",
		ID:           uuid.MustParse("22222222-2222-4222-8222-222222222222"),
		CreatedAt:    createdAt,
		UpdatedAt:    createdAt.Add(time.Hour),
		Name:         "Newsletter",
		Description:  "Monthly news",
		Tags:         []string{"news", "weekly"},
		ContactCount: 42,
	}
}

func listsPageFixture() *responses.PaginatedContactListsResponse {
	next, previous := "next-list", "previous-list"
	return &responses.PaginatedContactListsResponse{
		Object:     "list",
		Data:       []responses.ContactList{listFixture()},
		Pagination: responses.ContactPagination{HasMore: true, NextCursor: &next, PreviousCursor: &previous},
	}
}

func TestListJSONDecodesIntoSDKTypes(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleListsList(listsPageFixture(), ListConfig{}))
		var decoded responses.PaginatedContactListsResponse
		require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
		assert.Equal(t, *listsPageFixture(), decoded)
	})
	t.Run("single", func(t *testing.T) {
		list := listFixture()
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleSingleList(&list, SingleConfig{}))
		var decoded responses.ContactList
		require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
		assert.Equal(t, list, decoded)
	})
}

func TestListTableShowsColumnsAndCursorLines(t *testing.T) {
	useLocalTimeZone(t)
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleListsList(listsPageFixture(), ListConfig{SuccessMessage: "Lists"}))
	text := output.String()
	assert.Equal(t, [][]string{
		{"NAME", "ID", "TAGS", "CONTACTS", "UPDATED"},
		{"Newsletter", "22222222-2222-4222-8222-222222222222", "news, weekly", "42", "2026-09-10 11:30:00 UTC+2"},
	}, tableCells(text))
	assert.True(t, strings.HasSuffix(text, "--after next-list\n--before previous-list\n"), text)
}

func TestListPlainJoinsTagsAndEndsWithCursorLines(t *testing.T) {
	useLocalTimeZone(t)
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleListsList(listsPageFixture(), ListConfig{SuccessMessage: "Lists"}))
	assert.Equal(t, "Lists\n"+
		"Name: Newsletter\nID: 22222222-2222-4222-8222-222222222222\nDescription: Monthly news\n"+
		"Tags: news, weekly\nContacts: 42\nCreated: 2026-09-10 10:30:00 UTC+2\nUpdated: 2026-09-10 11:30:00 UTC+2\n"+
		"\nShowing 1 list (more available)\n--after next-list\n--before previous-list\n", output.String())
}

func TestListPlainSkipsUnsetFields(t *testing.T) {
	list := listFixture()
	list.Description = ""
	list.Tags = nil
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleSingleList(&list, SingleConfig{SuccessMessage: "List"}))
	assert.NotContains(t, output.String(), "Description")
	assert.NotContains(t, output.String(), "Tags")
}

func TestListCSVJoinsTagsWithCommas(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleListsList(listsPageFixture(), ListConfig{}))
	headers, rows := readCSV(t, output.String())
	require.Len(t, rows, 1)
	assert.Equal(t, listFields, headers)
	row := rows[0]
	assert.Equal(t, "Newsletter", row["name"])
	assert.Equal(t, "news, weekly", row["tags"])
	assert.Equal(t, "42", row["contact_count"])
}

func TestListSingleRendersInEveryFormat(t *testing.T) {
	for _, format := range GetSupportedFormats() {
		t.Run(format, func(t *testing.T) {
			list := listFixture()
			var output bytes.Buffer
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleSingleList(&list, SingleConfig{SuccessMessage: "List"}))
			assert.Contains(t, output.String(), "Newsletter")
			assert.Contains(t, output.String(), "42")
		})
	}
}

func TestListEmptyPageShowsMessage(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			response := &responses.PaginatedContactListsResponse{Object: "list"}
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleListsList(response, ListConfig{EmptyMessage: "No lists"}))
			assert.Equal(t, "No lists\n", output.String())
		})
	}
}
