package printer

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func campaignListItemFixture() responses.CampaignListItem {
	createdAt := time.Date(2026, time.October, 1, 8, 0, 0, 0, time.UTC)
	sendAt := time.Date(2026, time.November, 1, 9, 0, 0, 0, time.UTC)
	subject, preheader, replyTo, empty := "October news", "Read me", "support@example.com", ""
	return responses.CampaignListItem{
		Object:    "campaign",
		ID:        uuid.MustParse("44444444-4444-4444-8444-444444444444"),
		CreatedAt: createdAt,
		UpdatedAt: createdAt.Add(time.Hour),
		Name:      "October",
		Subject:   &subject,
		Preheader: &preheader,
		From:      &common.SenderAddress{Email: "news@example.com", Name: &empty},
		ReplyTo:   &replyTo,
		Status:    common.CampaignStatusScheduled,
		SendAt:    &sendAt,
	}
}

func campaignFixture() responses.Campaign {
	editor := "html"
	return responses.Campaign{
		CampaignListItem: campaignListItemFixture(),
		ListIDs: []uuid.UUID{
			uuid.MustParse("55555555-5555-4555-8555-555555555555"),
			uuid.MustParse("66666666-6666-4666-8666-666666666666"),
		},
		Editor:  &editor,
		Content: &responses.TemplateContent{HTML: "<p>News</p>", Text: "News"},
	}
}

// draftFixture returns a draft that is not ready, with no design.
func draftFixture() responses.Campaign {
	campaign := campaignFixture()
	campaign.Status = common.CampaignStatusDraft
	campaign.SendAt = nil
	campaign.Editor = nil
	campaign.Content = nil
	ready := false
	campaign.Ready = &ready
	campaign.Issues = []responses.CampaignIssue{
		{Code: responses.CampaignIssueSenderMissing, Message: "Add a sender"},
		{Code: responses.CampaignIssueDesignMissing, Message: "Add a design"},
	}
	return campaign
}

func campaignsPageFixture() *responses.PaginatedCampaignsResponse {
	next, previous := "next-campaign", "previous-campaign"
	return &responses.PaginatedCampaignsResponse{
		Object:     "list",
		Data:       []responses.CampaignListItem{campaignListItemFixture()},
		Pagination: common.PaginationInfo{HasMore: true, NextCursor: &next, PreviousCursor: &previous},
	}
}

func campaignSendFixture() *responses.CampaignSend {
	campaign := campaignFixture()
	campaign.Status = common.CampaignStatusValidating
	return &responses.CampaignSend{
		Campaign: campaign,
		Warnings: []string{"The account cannot send right now", "The view-in-browser page cannot show the design"},
	}
}

func campaignTestFixture() *responses.CampaignTest {
	return &responses.CampaignTest{
		Object: "campaign_test",
		SentTo: []string{"jane@example.com", "team@example.com"},
		Failed: []responses.CampaignTestFailure{{Email: "bad@example.com", Message: "mailbox unavailable"}},
	}
}

func TestCampaignJSONDecodesIntoSDKTypes(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleCampaignList(campaignsPageFixture(), ListConfig{}))
		var decoded responses.PaginatedCampaignsResponse
		require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
		assert.Equal(t, *campaignsPageFixture(), decoded)
	})
	t.Run("single", func(t *testing.T) {
		campaign := draftFixture()
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleSingleCampaign(&campaign, SingleConfig{}))
		var decoded responses.Campaign
		require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
		assert.Equal(t, campaign, decoded)
		assert.Contains(t, output.String(), `"content": null`)
	})
	t.Run("send", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleCampaignSend(campaignSendFixture(), SingleConfig{}))
		var decoded responses.CampaignSend
		require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
		assert.Equal(t, *campaignSendFixture(), decoded)
	})
	t.Run("test", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleCampaignTest(campaignTestFixture(), SingleConfig{}))
		var decoded responses.CampaignTest
		require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
		assert.Equal(t, *campaignTestFixture(), decoded)
	})
}

func TestCampaignTableListShowsColumnsAndCursorLines(t *testing.T) {
	useLocalTimeZone(t)
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleCampaignList(campaignsPageFixture(),
		ListConfig{SuccessMessage: "Campaigns"}))
	text := output.String()
	for _, expected := range []string{"NAME", "ID", "STATUS", "SUBJECT", "SEND AT", "UPDATED", "October",
		"44444444-4444-4444-8444-444444444444", "scheduled", "October news", "2026-11-01 11:00:00 UTC+2",
		"2026-10-01 11:00:00 UTC+2", "Showing 1 campaign (more available)"} {
		assert.Contains(t, text, expected)
	}
	assert.True(t, strings.HasSuffix(text, "--after next-campaign\n--before previous-campaign\n"), text)
}

func TestCampaignPlainListEndsWithCursorLines(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleCampaignList(campaignsPageFixture(), ListConfig{SuccessMessage: "Campaigns"}))
	text := output.String()
	assert.Contains(t, text, "Name: October\n")
	assert.Contains(t, text, "From: news@example.com\n")
	assert.True(t, strings.HasSuffix(text, "--after next-campaign\n--before previous-campaign\n"), text)
}

func TestCampaignEmptyPageShowsMessageAndCursor(t *testing.T) {
	previous := "previous-campaign"
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			response := &responses.PaginatedCampaignsResponse{Object: "list", Pagination: common.PaginationInfo{PreviousCursor: &previous}}
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleCampaignList(response, ListConfig{EmptyMessage: "No campaigns"}))
			assert.Equal(t, "No campaigns\n--before previous-campaign\n", output.String())
		})
	}
}

func TestCampaignPlainDraftShowsReadyAndIssues(t *testing.T) {
	useLocalTimeZone(t)
	campaign := draftFixture()
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleSingleCampaign(&campaign, SingleConfig{SuccessMessage: "Campaign"}))
	assert.Equal(t, "Campaign\n"+
		"Name: October\nID: 44444444-4444-4444-8444-444444444444\nStatus: draft\n"+
		"Subject: October news\nPreheader: Read me\nFrom: news@example.com\nReply To: support@example.com\n"+
		"Created: 2026-10-01 10:00:00 UTC+2\nUpdated: 2026-10-01 11:00:00 UTC+2\n"+
		"Lists: 55555555-5555-4555-8555-555555555555, 66666666-6666-4666-8666-666666666666\n"+
		"Ready: No\nIssues: sender_missing: Add a sender, design_missing: Add a design\n", output.String())
}

func TestCampaignTableDraftShowsReadyAndIssues(t *testing.T) {
	campaign := draftFixture()
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleSingleCampaign(&campaign, SingleConfig{SuccessMessage: "Campaign"}))
	text := output.String()
	assert.Contains(t, text, "Ready")
	assert.Contains(t, text, "│ Issues          │ sender_missing: Add a sender, design_missing: Add a design ")
}

func TestCampaignSingleShowsReadyAsMissingWhenNotDraft(t *testing.T) {
	for format, check := range map[string]func(t *testing.T, text string){
		"table": func(t *testing.T, text string) {
			assert.Regexp(t, `│ Ready +│ - +│`, text)
			assert.Regexp(t, `│ Issues +│ - +│`, text)
		},
		"plain": func(t *testing.T, text string) {
			assert.NotContains(t, text, "Ready")
			assert.NotContains(t, text, "Issues")
		},
	} {
		t.Run(format, func(t *testing.T) {
			campaign := campaignFixture()
			var output bytes.Buffer
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleSingleCampaign(&campaign, SingleConfig{SuccessMessage: "Campaign"}))
			check(t, output.String())
			assert.Contains(t, output.String(), "<p>News</p>")
		})
	}
}

func TestCampaignPlainReadyDraftShowsNoIssues(t *testing.T) {
	campaign := draftFixture()
	ready := true
	campaign.Ready = &ready
	campaign.Issues = []responses.CampaignIssue{}
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleSingleCampaign(&campaign, SingleConfig{SuccessMessage: "Campaign"}))
	assert.Contains(t, output.String(), "Ready: Yes\n")
	assert.NotContains(t, output.String(), "Issues:")
}

func TestCampaignSendPrintsWarnings(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleCampaignSend(campaignSendFixture(), SingleConfig{SuccessMessage: "Sent"}))
		text := output.String()
		assert.Contains(t, text, "Status: validating\n")
		assert.True(t, strings.HasSuffix(text, "Text Is Custom: No\n"+
			"Warnings: The account cannot send right now, The view-in-browser page cannot show the design\n"), text)
	})
	t.Run("table", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleCampaignSend(campaignSendFixture(), SingleConfig{SuccessMessage: "Sent"}))
		assert.Contains(t, output.String(), "│ Warnings        │ The account cannot send right now, The view-in-browser page cannot show the design │")
	})
	t.Run("csv", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleCampaignSend(campaignSendFixture(), SingleConfig{}))
		_, rows := readCSV(t, output.String())
		require.Len(t, rows, 1)
		row := rows[0]
		assert.Equal(t, "The account cannot send right now, The view-in-browser page cannot show the design", row["warnings"])
		assert.Equal(t, "validating", row["status"])
	})
}

func TestCampaignSendWithoutWarningsOmitsThem(t *testing.T) {
	send := campaignSendFixture()
	send.Warnings = []string{}
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleCampaignSend(send, SingleConfig{SuccessMessage: "Sent"}))
			assert.NotContains(t, output.String(), "Warnings")
		})
	}
}

func TestCampaignSendCSVAlwaysHasWarningsColumn(t *testing.T) {
	for name, warnings := range map[string][]string{"empty": {}, "null": nil} {
		t.Run(name, func(t *testing.T) {
			send := campaignSendFixture()
			send.Warnings = warnings
			var output bytes.Buffer
			require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleCampaignSend(send, SingleConfig{}))
			records, err := csv.NewReader(bytes.NewReader(output.Bytes())).ReadAll()
			require.NoError(t, err)
			require.Len(t, records, 2)
			assert.Equal(t, "warnings", records[0][len(records[0])-1])
		})
	}
}

func TestCampaignCSVWritesOneRowPerCampaign(t *testing.T) {
	campaign := draftFixture()
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleSingleCampaign(&campaign, SingleConfig{}))
	headers, rows := readCSV(t, output.String())
	require.Len(t, rows, 1)
	assert.Equal(t, slices.Concat(campaignDetailFields, templateContentColumns), headers)
	row := rows[0]
	assert.Equal(t, "October", row["name"])
	assert.Equal(t, "55555555-5555-4555-8555-555555555555, 66666666-6666-4666-8666-666666666666", row["list_ids"])
	assert.Equal(t, "false", row["ready"])
	assert.Equal(t, "sender_missing: Add a sender, design_missing: Add a design", row["issues"])
	assert.Equal(t, "", row["html"])
}

func TestCampaignCSVListLeavesDetailCellsEmpty(t *testing.T) {
	useLocalTimeZone(t)
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleCampaignList(campaignsPageFixture(), ListConfig{}))
	_, rows := readCSV(t, output.String())
	require.Len(t, rows, 1)
	row := rows[0]
	assert.Equal(t, "October news", row["subject"])
	assert.Equal(t, "2026-11-01T09:00:00Z", row["send_at"])
	assert.Equal(t, "", row["list_ids"])
	assert.Equal(t, "", row["ready"])
}

func TestCampaignTestTableShowsCountsAndFailures(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleCampaignTest(campaignTestFixture(), SingleConfig{SuccessMessage: "Test sent"}))
	text := output.String()
	assert.Contains(t, text, "Sent: 2  Failed: 1\n")
	for _, expected := range []string{"EMAIL", "OUTCOME", "REASON", "jane@example.com", "sent", "bad@example.com",
		"failed", "mailbox unavailable"} {
		assert.Contains(t, text, expected)
	}
}

func TestCampaignTestPlainShowsSentAndFailures(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleCampaignTest(campaignTestFixture(), SingleConfig{SuccessMessage: "Test sent"}))
	assert.Equal(t, "Test sent\nSent: 2\nFailed: 1\nSent To: jane@example.com, team@example.com\n"+
		"Failures: bad@example.com: mailbox unavailable\n", output.String())
}

func TestCampaignTestCSVWritesOneRowPerAddress(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleCampaignTest(campaignTestFixture(), SingleConfig{}))
	records, err := csv.NewReader(bytes.NewReader(output.Bytes())).ReadAll()
	require.NoError(t, err)
	assert.Equal(t, [][]string{
		{"email", "outcome", "reason"},
		{"jane@example.com", "sent", ""},
		{"team@example.com", "sent", ""},
		{"bad@example.com", "failed", "mailbox unavailable"},
	}, records)
}

func TestCampaignTestResultUnknown(t *testing.T) {
	unknown := &responses.CampaignTest{Object: "campaign_test", ResultUnknown: true}
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleCampaignTest(unknown, SingleConfig{SuccessMessage: "Test sent"}))
			assert.Equal(t, campaignTestUnknownNotice+"\n", output.String())
		})
	}
	t.Run("csv", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleCampaignTest(unknown, SingleConfig{}))
		records, err := csv.NewReader(bytes.NewReader(output.Bytes())).ReadAll()
		require.NoError(t, err)
		assert.Equal(t, [][]string{{"email", "outcome", "reason"}, {"", "unknown", ""}}, records)
	})
}

func TestCampaignRendersInEveryFormat(t *testing.T) {
	for _, format := range GetSupportedFormats() {
		t.Run(format, func(t *testing.T) {
			campaign := campaignFixture()
			var single, list, send, test bytes.Buffer
			handler := func(buffer *bytes.Buffer) ResponseHandler { return testHandler(t, format, buffer, os.Stderr) }
			require.NoError(t, handler(&single).HandleSingleCampaign(&campaign, SingleConfig{SuccessMessage: "Campaign"}))
			require.NoError(t, handler(&list).HandleCampaignList(campaignsPageFixture(), ListConfig{SuccessMessage: "Campaigns"}))
			require.NoError(t, handler(&send).HandleCampaignSend(campaignSendFixture(), SingleConfig{SuccessMessage: "Sent"}))
			require.NoError(t, handler(&test).HandleCampaignTest(campaignTestFixture(), SingleConfig{SuccessMessage: "Test"}))
			assert.Contains(t, single.String(), "October")
			assert.Contains(t, list.String(), "October")
			assert.Contains(t, send.String(), "The account cannot send right now")
			assert.Contains(t, test.String(), "bad@example.com")
		})
	}
}
