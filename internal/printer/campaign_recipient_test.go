package printer

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func campaignRecipientsPageFixture() *responses.PaginatedCampaignRecipientsResponse {
	createdAt := time.Date(2026, time.October, 2, 9, 0, 0, 0, time.UTC)
	classification, reason, message := "invalid_recipient", "mailbox does not exist", "550 5.1.1 unknown user"
	listName := "Newsletter"
	next, previous := "next-recipient", "previous-recipient"
	return &responses.PaginatedCampaignRecipientsResponse{
		Object: "list",
		Data: []responses.CampaignRecipient{{
			Object:               "campaign_recipient",
			ContactID:            uuid.MustParse("88888888-8888-4888-8888-888888888888"),
			Email:                "bad@example.com",
			ListName:             &listName,
			Status:               "Bounced",
			BounceClassification: &classification,
			BounceReason:         &reason,
			BounceMessage:        &message,
			Attempts:             2,
			CreatedAt:            createdAt,
			UpdatedAt:            createdAt.Add(time.Minute),
		}},
		Pagination: common.PaginationInfo{HasMore: true, NextCursor: &next, PreviousCursor: &previous},
	}
}

// segmentRecipientsPageFixture returns a page of one recipient a campaign
// reached through a segment: it has a segment and no list name.
func segmentRecipientsPageFixture() *responses.PaginatedCampaignRecipientsResponse {
	page := campaignRecipientsPageFixture()
	segmentID := uuid.MustParse("99999999-9999-4999-8999-999999999999")
	segmentName := "Active buyers"
	page.Data[0].ListName = nil
	page.Data[0].SegmentID = &segmentID
	page.Data[0].SegmentName = &segmentName
	return page
}

func TestCampaignRecipientJSONDecodesIntoSDKType(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleCampaignRecipientList(campaignRecipientsPageFixture(), ListConfig{}))
	var decoded responses.PaginatedCampaignRecipientsResponse
	require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
	assert.Equal(t, *campaignRecipientsPageFixture(), decoded)
}

func TestCampaignRecipientTableShowsColumnsAndCursorLines(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleCampaignRecipientList(campaignRecipientsPageFixture(),
		ListConfig{SuccessMessage: "Recipients"}))
	text := output.String()
	for _, expected := range []string{"EMAIL", "LIST", "STATUS", "BOUNCE CLASSIFICATION", "ATTEMPTS", "bad@example.com",
		"Newsletter", "Bounced", "invalid_recipient", "Showing 1 recipient (more available)"} {
		assert.Contains(t, text, expected)
	}
	assert.True(t, strings.HasSuffix(text, "--after next-recipient\n--before previous-recipient\n"), text)
}

func TestCampaignRecipientPlainShowsDetailsAndCursorLines(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleCampaignRecipientList(campaignRecipientsPageFixture(),
		ListConfig{SuccessMessage: "Recipients"}))
	text := output.String()
	for _, expected := range []string{"Email: bad@example.com\n", "Contact ID: 88888888-8888-4888-8888-888888888888\n",
		"List: Newsletter\n", "Bounce Reason: mailbox does not exist\n", "Attempts: 2\n"} {
		assert.Contains(t, text, expected)
	}
	assert.True(t, strings.HasSuffix(text, "--after next-recipient\n--before previous-recipient\n"), text)
}

func TestCampaignRecipientThroughASegmentShowsTheSegmentAndNoList(t *testing.T) {
	render := func(h ResponseHandler) error {
		return h.HandleCampaignRecipientList(segmentRecipientsPageFixture(), ListConfig{})
	}

	table := tableCells(renderWith(t, "table", render))
	require.Len(t, table, 2)
	assert.Equal(t, []string{"EMAIL", "CONTACT ID", "LIST", "SEGMENT", "STATUS", "BOUNCE CLASSIFICATION", "ATTEMPTS"}, table[0])
	assert.Equal(t, "-", table[1][2], "the List value is empty")
	assert.Equal(t, "Active buyers", table[1][3])

	plain := renderWith(t, "plain", render)
	assert.Contains(t, plain, "\nSegment ID: 99999999-9999-4999-8999-999999999999\nSegment: Active buyers\n")
	assert.NotContains(t, plain, "List:")

	_, rows := readCSV(t, renderWith(t, "csv", render))
	require.Len(t, rows, 1)
	assert.Equal(t, "", rows[0]["list_name"])
	assert.Equal(t, "99999999-9999-4999-8999-999999999999", rows[0]["segment_id"])
	assert.Equal(t, "Active buyers", rows[0]["segment_name"])
}

func TestCampaignRecipientThroughAListShowsNoSegment(t *testing.T) {
	render := func(h ResponseHandler) error {
		return h.HandleCampaignRecipientList(campaignRecipientsPageFixture(), ListConfig{})
	}

	table := tableCells(renderWith(t, "table", render))
	require.Len(t, table, 2)
	assert.Equal(t, "Newsletter", table[1][2])
	assert.Equal(t, "-", table[1][3], "the Segment value is empty")
	assert.NotContains(t, renderWith(t, "plain", render), "Segment")
	_, rows := readCSV(t, renderWith(t, "csv", render))
	assert.Equal(t, "", rows[0]["segment_id"])
	assert.Equal(t, "", rows[0]["segment_name"])
}

func TestCampaignRecipientEmptyPageShowsMessageAndCursor(t *testing.T) {
	previous := "previous-recipient"
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			response := &responses.PaginatedCampaignRecipientsResponse{Object: "list", Pagination: common.PaginationInfo{PreviousCursor: &previous}}
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleCampaignRecipientList(response, ListConfig{EmptyMessage: "No recipients"}))
			assert.Equal(t, "No recipients\n--before previous-recipient\n", output.String())
		})
	}
}

func TestCampaignRecipientCSVWritesOneRowPerRecipient(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleCampaignRecipientList(campaignRecipientsPageFixture(), ListConfig{}))
	headers, rows := readCSV(t, output.String())
	require.Len(t, rows, 1)
	assert.Equal(t, campaignRecipientFields, headers)
	row := rows[0]
	assert.Equal(t, "bad@example.com", row["email"])
	assert.Equal(t, "invalid_recipient", row["bounce_classification"])
	assert.Equal(t, "550 5.1.1 unknown user", row["bounce_message"])
	assert.Equal(t, "2", row["attempts"])
}

func TestCampaignRecipientRendersInEveryFormat(t *testing.T) {
	for _, format := range GetSupportedFormats() {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleCampaignRecipientList(campaignRecipientsPageFixture(),
				ListConfig{SuccessMessage: "Recipients"}))
			assert.Contains(t, output.String(), "bad@example.com")
		})
	}
}
