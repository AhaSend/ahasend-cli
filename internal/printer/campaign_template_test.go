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

func campaignTemplateFixture() responses.CampaignTemplate {
	createdAt := time.Date(2026, time.September, 20, 8, 0, 0, 0, time.UTC)
	return responses.CampaignTemplate{
		Object:    "campaign_template",
		ID:        uuid.MustParse("77777777-7777-4777-8777-777777777777"),
		CreatedAt: createdAt,
		UpdatedAt: createdAt.Add(time.Hour),
		Name:      "Newsletter",
		Editor:    "advanced",
		Content: &responses.TemplateContent{
			MJML: "<mjml></mjml>",
			HTML: "<p>News</p>",
			Text: "News",
		},
	}
}

func campaignTemplatesPageFixture() *responses.PaginatedCampaignTemplatesResponse {
	template := campaignTemplateFixture()
	template.Content = nil
	next, previous := "next-campaign-template", "previous-campaign-template"
	return &responses.PaginatedCampaignTemplatesResponse{
		Object:     "list",
		Data:       []responses.CampaignTemplate{template},
		Pagination: common.PaginationInfo{HasMore: true, NextCursor: &next, PreviousCursor: &previous},
	}
}

func TestCampaignTemplateJSONDecodesIntoSDKTypes(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleCampaignTemplateList(campaignTemplatesPageFixture(), ListConfig{}))
		var decoded responses.PaginatedCampaignTemplatesResponse
		require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
		assert.Equal(t, *campaignTemplatesPageFixture(), decoded)
		assert.NotContains(t, output.String(), `"content"`)
	})
	t.Run("single", func(t *testing.T) {
		template := campaignTemplateFixture()
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleSingleCampaignTemplate(&template, SingleConfig{}))
		var decoded responses.CampaignTemplate
		require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
		assert.Equal(t, template, decoded)
	})
}

func TestCampaignTemplateTableListShowsColumnsAndCursorLines(t *testing.T) {
	useLocalTimeZone(t)
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleCampaignTemplateList(campaignTemplatesPageFixture(),
		ListConfig{SuccessMessage: "Campaign templates"}))
	text := output.String()
	for _, expected := range []string{"NAME", "ID", "EDITOR", "UPDATED", "Newsletter", "77777777-7777-4777-8777-777777777777",
		"advanced", "2026-09-20 11:00:00 UTC+2", "Showing 1 campaign template (more available)"} {
		assert.Contains(t, text, expected)
	}
	assert.True(t, strings.HasSuffix(text, "--after next-campaign-template\n--before previous-campaign-template\n"), text)
}

func TestCampaignTemplatePlainListEndsWithCursorLines(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleCampaignTemplateList(campaignTemplatesPageFixture(),
		ListConfig{SuccessMessage: "Campaign templates"}))
	text := output.String()
	assert.Contains(t, text, "Name: Newsletter\n")
	assert.NotContains(t, text, "MJML")
	assert.True(t, strings.HasSuffix(text, "--after next-campaign-template\n--before previous-campaign-template\n"), text)
}

func TestCampaignTemplateEmptyPageShowsMessageAndCursor(t *testing.T) {
	previous := "previous-campaign-template"
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			response := &responses.PaginatedCampaignTemplatesResponse{Object: "list", Pagination: common.PaginationInfo{PreviousCursor: &previous}}
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleCampaignTemplateList(response, ListConfig{EmptyMessage: "No campaign templates"}))
			assert.Equal(t, "No campaign templates\n--before previous-campaign-template\n", output.String())
		})
	}
}

func TestCampaignTemplateSingleShowsContent(t *testing.T) {
	template := campaignTemplateFixture()
	t.Run("plain", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleSingleCampaignTemplate(&template, SingleConfig{SuccessMessage: "Template"}))
		text := output.String()
		assert.Contains(t, text, "Editor: advanced\n")
		assert.Contains(t, text, "MJML:\n  <mjml></mjml>\n")
		assert.Contains(t, text, "HTML:\n  <p>News</p>\n")
		assert.Contains(t, text, "Text Is Custom: No\n")
	})
	t.Run("table", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleSingleCampaignTemplate(&template, SingleConfig{SuccessMessage: "Template"}))
		assert.Contains(t, output.String(), "<p>News</p>")
	})
}

func TestCampaignTemplateSingleHidesContentWhenAbsent(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			template := campaignTemplateFixture()
			template.Content = nil
			var output bytes.Buffer
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleSingleCampaignTemplate(&template, SingleConfig{SuccessMessage: "Template"}))
			assert.Contains(t, output.String(), "Newsletter")
			assert.NotContains(t, output.String(), "Text Is Custom")
		})
	}
}

func TestCampaignTemplateCSVWritesOneRowPerTemplate(t *testing.T) {
	template := campaignTemplateFixture()
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleSingleCampaignTemplate(&template, SingleConfig{}))
	headers, rows := readCSV(t, output.String())
	require.Len(t, rows, 1)
	assert.Equal(t, campaignTemplateListFields, headers)
	row := rows[0]
	assert.Equal(t, "Newsletter", row["name"])
	assert.Equal(t, "<p>News</p>", row["html"])
	assert.Equal(t, "false", row["text_is_custom"])
}

func TestCampaignTemplateRendersInEveryFormat(t *testing.T) {
	for _, format := range GetSupportedFormats() {
		t.Run(format, func(t *testing.T) {
			template := campaignTemplateFixture()
			var single, list bytes.Buffer
			require.NoError(t, testHandler(t, format, &single, os.Stderr).HandleSingleCampaignTemplate(&template, SingleConfig{SuccessMessage: "Template"}))
			require.NoError(t, testHandler(t, format, &list, os.Stderr).HandleCampaignTemplateList(campaignTemplatesPageFixture(),
				ListConfig{SuccessMessage: "Templates"}))
			assert.Contains(t, single.String(), "Newsletter")
			assert.Contains(t, list.String(), "Newsletter")
		})
	}
}
