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

func templateFixture() responses.Template {
	createdAt := time.Date(2026, time.September, 10, 8, 30, 0, 0, time.UTC)
	fromName := "Example"
	return responses.Template{
		Object:    "template",
		ID:        uuid.MustParse("33333333-3333-4333-8333-333333333333"),
		CreatedAt: createdAt,
		UpdatedAt: createdAt.Add(time.Hour),
		Name:      "Welcome",
		Subject:   "Hello {{ first_name }}",
		Preheader: "Read me",
		Variables: []responses.TemplateVariable{
			{Name: "first_name", Required: true},
			{Name: "nickname", Required: false},
		},
		From:     &common.SenderAddress{Email: "hello@example.com", Name: &fromName},
		ReplyTo:  &common.SenderAddress{Email: "support@example.com"},
		Editor:   "advanced",
		HasDraft: true,
		Content: &responses.TemplateContent{
			MJML:         "<mjml>\n<mj-body></mj-body>\n</mjml>",
			Text:         "Hello",
			TextIsCustom: true,
		},
	}
}

// templateWithoutContent returns a template as list items carry it: without
// content, sender or reply-to.
func templateWithoutContent() responses.Template {
	template := templateFixture()
	template.Content = nil
	template.From = nil
	template.ReplyTo = nil
	return template
}

func templatesPageFixture() *responses.PaginatedTemplatesResponse {
	next, previous := "next-template", "previous-template"
	return &responses.PaginatedTemplatesResponse{
		Object:     "list",
		Data:       []responses.Template{templateWithoutContent()},
		Pagination: common.PaginationInfo{HasMore: true, NextCursor: &next, PreviousCursor: &previous},
	}
}

func TestTemplateJSONDecodesIntoSDKTypes(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleTemplateList(templatesPageFixture(), ListConfig{}))
		var decoded responses.PaginatedTemplatesResponse
		require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
		assert.Equal(t, *templatesPageFixture(), decoded)
	})
	t.Run("single", func(t *testing.T) {
		template := templateFixture()
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleSingleTemplate(&template, SingleConfig{}))
		var decoded responses.Template
		require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
		assert.Equal(t, template, decoded)
	})
	t.Run("single without content leaves content out", func(t *testing.T) {
		template := templateWithoutContent()
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleSingleTemplate(&template, SingleConfig{}))
		assert.NotContains(t, output.String(), `"content"`)
		assert.NotContains(t, output.String(), `"from"`)
	})
}

func TestTemplateTableShowsColumnsAndCursorLines(t *testing.T) {
	useLocalTimeZone(t)
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleTemplateList(templatesPageFixture(),
		ListConfig{SuccessMessage: "Templates"}))
	text := output.String()
	for _, expected := range []string{"NAME", "EDITOR", "SUBJECT", "HAS DRAFT", "UPDATED", "Welcome", "advanced",
		"Hello {{ first_name }}", "Yes", "2026-09-10 11:30:00 UTC+2", "Showing 1 template (more available)"} {
		assert.Contains(t, text, expected)
	}
	assert.True(t, strings.HasSuffix(text, "--after next-template\n--before previous-template\n"), text)
}

func TestTemplatePlainListEndsWithCursorLines(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleTemplateList(templatesPageFixture(), ListConfig{SuccessMessage: "Templates"}))
	text := output.String()
	assert.Contains(t, text, "Name: Welcome\n")
	assert.Contains(t, text, "Has Draft: Yes\n")
	assert.True(t, strings.HasSuffix(text, "--after next-template\n--before previous-template\n"), text)
}

func TestTemplateEmptyPageShowsMessageAndCursor(t *testing.T) {
	previous := "previous-template"
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			response := &responses.PaginatedTemplatesResponse{Object: "list", Pagination: common.PaginationInfo{PreviousCursor: &previous}}
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleTemplateList(response, ListConfig{EmptyMessage: "No templates"}))
			assert.Equal(t, "No templates\n--before previous-template\n", output.String())
		})
	}
}

func TestTemplatePlainSingleListsVariablesAndContent(t *testing.T) {
	useLocalTimeZone(t)
	template := templateFixture()
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleSingleTemplate(&template, SingleConfig{SuccessMessage: "Template"}))
	assert.Equal(t, "Template\n"+
		"Name: Welcome\nID: 33333333-3333-4333-8333-333333333333\nEditor: advanced\n"+
		"Subject: Hello {{ first_name }}\nPreheader: Read me\nFrom: Example <hello@example.com>\n"+
		"Reply To: support@example.com\nHas Draft: Yes\n"+
		"Variables: first_name (required), nickname (optional)\n"+
		"Created: 2026-09-10 10:30:00 UTC+2\nUpdated: 2026-09-10 11:30:00 UTC+2\n"+
		"MJML:\n  <mjml>\n  <mj-body></mj-body>\n  </mjml>\nText:\n  Hello\nText Is Custom: Yes\n", output.String())
}

func TestTemplateSingleHidesContentWhenAbsent(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			template := templateWithoutContent()
			var output bytes.Buffer
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleSingleTemplate(&template, SingleConfig{SuccessMessage: "Template"}))
			text := output.String()
			assert.Contains(t, text, "first_name (required)")
			assert.NotContains(t, text, "MJML")
			assert.NotContains(t, text, "Text Is Custom")
		})
	}
}

func TestTemplateTableSingleShowsVariablesAndContent(t *testing.T) {
	template := templateFixture()
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleSingleTemplate(&template, SingleConfig{SuccessMessage: "Template"}))
	text := output.String()
	assert.Contains(t, text, "│ Variables      │ first_name (required), nickname (optional) │")
	assert.Contains(t, text, "<mj-body></mj-body>")
	assert.Contains(t, text, "Text Is Custom")
}

func TestTemplateCSVWritesOneRowPerTemplate(t *testing.T) {
	template := templateFixture()
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleSingleTemplate(&template, SingleConfig{}))
	headers, rows := readCSV(t, output.String())
	require.Len(t, rows, 1)
	assert.Equal(t, templateListFields, headers)
	row := rows[0]
	assert.Equal(t, "Welcome", row["name"])
	assert.Equal(t, "Example <hello@example.com>", row["from"])
	assert.Equal(t, "first_name (required), nickname (optional)", row["variables"])
	assert.Equal(t, "true", row["has_draft"])
	assert.Equal(t, template.Content.MJML, row["mjml"])
	assert.Equal(t, "true", row["text_is_custom"])
}

func TestTemplateCSVListLeavesContentCellsEmpty(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleTemplateList(templatesPageFixture(), ListConfig{}))
	_, rows := readCSV(t, output.String())
	require.Len(t, rows, 1)
	row := rows[0]
	assert.Equal(t, "", row["mjml"])
	assert.Equal(t, "", row["text_is_custom"])
	assert.Equal(t, "", row["from"])
}

func TestTemplateRendersInEveryFormat(t *testing.T) {
	for _, format := range GetSupportedFormats() {
		t.Run(format, func(t *testing.T) {
			template := templateFixture()
			var single, list bytes.Buffer
			require.NoError(t, testHandler(t, format, &single, os.Stderr).HandleSingleTemplate(&template, SingleConfig{SuccessMessage: "Template"}))
			require.NoError(t, testHandler(t, format, &list, os.Stderr).HandleTemplateList(templatesPageFixture(), ListConfig{SuccessMessage: "Templates"}))
			assert.Contains(t, single.String(), "Welcome")
			assert.Contains(t, list.String(), "Welcome")
		})
	}
}
