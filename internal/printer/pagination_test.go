package printer

import (
	"bytes"
	"os"
	"testing"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderCursorLines(t *testing.T, pagination common.PaginationInfo) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, writeCursorLines(&buf, pagination))
	return buf.String()
}

func TestWriteCursorLines_NextCursor(t *testing.T) {
	next := "next-page"
	out := renderCursorLines(t, common.PaginationInfo{HasMore: true, NextCursor: &next})
	assert.Equal(t, "--after next-page\n", out)
}

func TestWriteCursorLines_PreviousCursor(t *testing.T) {
	previous := "previous-page"
	out := renderCursorLines(t, common.PaginationInfo{PreviousCursor: &previous})
	assert.Equal(t, "--before previous-page\n", out)
}

func TestWriteCursorLines_BothCursors(t *testing.T) {
	next, previous := "next-page", "previous-page"
	out := renderCursorLines(t, common.PaginationInfo{HasMore: true, NextCursor: &next, PreviousCursor: &previous})
	assert.Equal(t, "--after next-page\n--before previous-page\n", out)
}

func TestWriteCursorLines_NoCursor(t *testing.T) {
	assert.Empty(t, renderCursorLines(t, common.PaginationInfo{}))
}

func renderShowingLine(t *testing.T, count int) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, writeShowingLine(&buf, count, "campaigns", false))
	return buf.String()
}

func TestWriteShowingLine_OneItemIsSingular(t *testing.T) {
	assert.Equal(t, "\nShowing 1 campaign\n", renderShowingLine(t, 1))
}

func TestWriteShowingLine_OtherCountsArePlural(t *testing.T) {
	assert.Equal(t, "\nShowing 2 campaigns\n", renderShowingLine(t, 2))
}

func TestCSVListsHaveNoCursorLine(t *testing.T) {
	for name, render := range everyList(1) {
		t.Run(name, func(t *testing.T) {
			assert.NotContains(t, renderWith(t, "csv", render), "next-page")
		})
	}
}

func TestEmptyListPageKeepsTheCursorLine(t *testing.T) {
	next := "next-page"
	var buf bytes.Buffer
	handler := testHandler(t, "table", &buf, os.Stderr)
	require.NoError(t, handler.HandleWebhookList(&responses.PaginatedWebhooksResponse{
		Pagination: common.PaginationInfo{HasMore: true, NextCursor: &next},
	}, ListConfig{EmptyMessage: "No enabled webhooks found"}))
	assert.Equal(t, "No enabled webhooks found\n--after next-page\n", buf.String())
}
