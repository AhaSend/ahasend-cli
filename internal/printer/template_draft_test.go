package printer

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func templateDraftFixture() responses.TemplateDraft {
	return responses.TemplateDraft{
		Object:     "template_draft",
		TemplateID: uuid.MustParse("33333333-3333-4333-8333-333333333333"),
		UpdatedAt:  time.Date(2026, time.September, 11, 8, 0, 0, 0, time.UTC),
		Subject:    "Draft subject",
		Preheader:  "Draft preheader",
		Variables:  []responses.TemplateVariable{{Name: "order_id", Required: true}},
		From:       &common.SenderAddress{Email: "hello@example.com"},
		Content:    &responses.TemplateContent{HTML: "<p>Order {{ order_id }}</p>", Text: "Order"},
	}
}

func TestTemplateDraftJSONDecodesIntoSDKType(t *testing.T) {
	draft := templateDraftFixture()
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleTemplateDraft(&draft, SingleConfig{}))
	var decoded responses.TemplateDraft
	require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
	assert.Equal(t, draft, decoded)
	assert.Contains(t, output.String(), "<p>Order {{ order_id }}</p>")
}

func TestTemplateDraftPlainListsVariablesAndContent(t *testing.T) {
	useLocalTimeZone(t)
	draft := templateDraftFixture()
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleTemplateDraft(&draft, SingleConfig{SuccessMessage: "Draft"}))
	assert.Equal(t, "Draft\n"+
		"Subject: Draft subject\nTemplate ID: 33333333-3333-4333-8333-333333333333\n"+
		"Preheader: Draft preheader\nFrom: hello@example.com\nUpdated: 2026-09-11 10:00:00 UTC+2\n"+
		"Variables: order_id (required)\nHTML:\n  <p>Order {{ order_id }}</p>\nText:\n  Order\n"+
		"Text Is Custom: No\n", output.String())
}

func TestTemplateDraftHidesContentWhenAbsent(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			draft := templateDraftFixture()
			draft.Content = nil
			var output bytes.Buffer
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleTemplateDraft(&draft, SingleConfig{SuccessMessage: "Draft"}))
			assert.Contains(t, output.String(), "order_id (required)")
			assert.NotContains(t, output.String(), "HTML")
		})
	}
}

func TestTemplateDraftCSVWritesOneRow(t *testing.T) {
	draft := templateDraftFixture()
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleTemplateDraft(&draft, SingleConfig{}))
	headers, rows := readCSV(t, output.String())
	require.Len(t, rows, 1)
	assert.Equal(t, templateDraftCSVHeaders, headers)
	row := rows[0]
	assert.Equal(t, "Draft subject", row["subject"])
	assert.Equal(t, "order_id (required)", row["variables"])
	assert.Equal(t, "<p>Order {{ order_id }}</p>", row["html"])
}

func TestTemplateDraftRendersInEveryFormat(t *testing.T) {
	for _, format := range GetSupportedFormats() {
		t.Run(format, func(t *testing.T) {
			draft := templateDraftFixture()
			var output bytes.Buffer
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleTemplateDraft(&draft, SingleConfig{SuccessMessage: "Draft"}))
			assert.Contains(t, output.String(), "Draft subject")
		})
	}
}
