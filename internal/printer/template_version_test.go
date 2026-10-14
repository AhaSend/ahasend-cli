package printer

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const templatePublisherID = "55555555-5555-4555-8555-555555555555"

func templateVersionsFixture() *responses.TemplateVersionsResponse {
	publishedAt := time.Date(2026, time.September, 12, 8, 0, 0, 0, time.UTC)
	return &responses.TemplateVersionsResponse{
		Object: "list",
		Data: []responses.TemplateVersion{
			{
				Object:      "template_version",
				ID:          uuid.MustParse("44444444-4444-4444-8444-444444444444"),
				Version:     2,
				PublishedAt: publishedAt,
				PublishedBy: &responses.TemplatePublisher{Type: "api_key", ID: uuid.MustParse(templatePublisherID)},
			},
			{
				Object:      "template_version",
				ID:          uuid.MustParse("66666666-6666-4666-8666-666666666666"),
				Version:     1,
				PublishedAt: publishedAt.Add(-time.Hour),
			},
		},
	}
}

func templateVersionDetailFixture() responses.TemplateVersionDetail {
	return responses.TemplateVersionDetail{
		Object:      "template_version",
		ID:          uuid.MustParse("44444444-4444-4444-8444-444444444444"),
		Version:     2,
		PublishedAt: time.Date(2026, time.September, 12, 8, 0, 0, 0, time.UTC),
		PublishedBy: &responses.TemplatePublisher{Type: "user", ID: uuid.MustParse(templatePublisherID)},
		Subject:     "Version subject",
		Variables:   []responses.TemplateVariable{{Name: "first_name"}},
		Content:     &responses.TemplateContent{Text: "Hi"},
	}
}

func TestTemplateVersionJSONDecodesIntoSDKTypes(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleTemplateVersionList(templateVersionsFixture(), ListConfig{}))
		var decoded responses.TemplateVersionsResponse
		require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
		assert.Equal(t, *templateVersionsFixture(), decoded)
		assert.NotContains(t, output.String(), `"published_by": null`)
	})
	t.Run("single", func(t *testing.T) {
		version := templateVersionDetailFixture()
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleSingleTemplateVersion(&version, SingleConfig{}))
		var decoded responses.TemplateVersionDetail
		require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
		assert.Equal(t, version, decoded)
	})
}

func TestTemplateVersionTableShowsColumnsAndPublisher(t *testing.T) {
	useLocalTimeZone(t)
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleTemplateVersionList(templateVersionsFixture(), ListConfig{SuccessMessage: "Versions"}))
	text := output.String()
	for _, expected := range []string{"VERSION", "PUBLISHED AT", "PUBLISHED BY", "2026-09-12 10:00:00 UTC+2", "api_key:" + templatePublisherID} {
		assert.Contains(t, text, expected)
	}
}

func TestTemplateVersionPlainLeavesOutUnknownPublisher(t *testing.T) {
	useLocalTimeZone(t)
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleTemplateVersionList(templateVersionsFixture(), ListConfig{SuccessMessage: "Versions"}))
	assert.Equal(t, "Versions\n"+
		"Version: 2\nID: 44444444-4444-4444-8444-444444444444\nPublished At: 2026-09-12 10:00:00 UTC+2\n"+
		"Published By: api_key:"+templatePublisherID+"\n"+
		"\nVersion: 1\nID: 66666666-6666-4666-8666-666666666666\nPublished At: 2026-09-12 09:00:00 UTC+2\n"+
		"\nShowing 2 versions\n",
		output.String())
}

func TestTemplateVersionCSVWritesOneRowPerVersion(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleTemplateVersionList(templateVersionsFixture(), ListConfig{}))
	headers, rows := readCSV(t, output.String())
	require.Len(t, rows, 2)
	assert.Equal(t, templateVersionFields, headers)
	assert.Equal(t, "api_key:"+templatePublisherID, rows[0]["published_by"])
	assert.Equal(t, "", rows[1]["published_by"])
}

func TestTemplateVersionPlainSingleListsVariablesAndContent(t *testing.T) {
	useLocalTimeZone(t)
	version := templateVersionDetailFixture()
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleSingleTemplateVersion(&version, SingleConfig{SuccessMessage: "Version"}))
	assert.Equal(t, "Version\n"+
		"Version: 2\nID: 44444444-4444-4444-8444-444444444444\nPublished At: 2026-09-12 10:00:00 UTC+2\n"+
		"Published By: user:"+templatePublisherID+"\nSubject: Version subject\n"+
		"Variables: first_name (optional)\nText:\n  Hi\nText Is Custom: No\n", output.String())
}

func TestTemplateVersionSingleHidesContentWhenAbsent(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			version := templateVersionDetailFixture()
			version.Content = nil
			var output bytes.Buffer
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleSingleTemplateVersion(&version, SingleConfig{SuccessMessage: "Version"}))
			assert.Contains(t, output.String(), "first_name (optional)")
			assert.NotContains(t, output.String(), "Text Is Custom")
		})
	}
}

func TestTemplateVersionCSVSingleWritesOneRow(t *testing.T) {
	version := templateVersionDetailFixture()
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleSingleTemplateVersion(&version, SingleConfig{}))
	headers, rows := readCSV(t, output.String())
	require.Len(t, rows, 1)
	assert.Equal(t, templateVersionDetailCSVHeaders, headers)
	row := rows[0]
	assert.Equal(t, "Version subject", row["subject"])
	assert.Equal(t, "Hi", row["text"])
}

func TestTemplateVersionRendersInEveryFormat(t *testing.T) {
	for _, format := range GetSupportedFormats() {
		t.Run(format, func(t *testing.T) {
			version := templateVersionDetailFixture()
			var single, list bytes.Buffer
			require.NoError(t, testHandler(t, format, &single, os.Stderr).HandleSingleTemplateVersion(&version, SingleConfig{SuccessMessage: "Version"}))
			require.NoError(t, testHandler(t, format, &list, os.Stderr).HandleTemplateVersionList(templateVersionsFixture(), ListConfig{SuccessMessage: "Versions"}))
			assert.Contains(t, single.String(), "Version subject")
			assert.Contains(t, list.String(), "44444444-4444-4444-8444-444444444444")
		})
	}
}

func TestTemplateVersionEmptyListShowsMessage(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			response := &responses.TemplateVersionsResponse{Object: "list"}
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleTemplateVersionList(response, ListConfig{EmptyMessage: "No versions"}))
			assert.Equal(t, "No versions\n", output.String())
		})
	}
}
