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
	next, previous := "next-recipient", "previous-recipient"
	return &responses.PaginatedCampaignRecipientsResponse{
		Object: "list",
		Data: []responses.CampaignRecipient{{
			Object:               "campaign_recipient",
			ContactID:            uuid.MustParse("88888888-8888-4888-8888-888888888888"),
			Email:                "bad@example.com",
			ListName:             "Newsletter",
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
