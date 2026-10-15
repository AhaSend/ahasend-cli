package printer

import (
	"bytes"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestProtectedSuppression() *responses.Suppression {
	return &responses.Suppression{
		Object:    "suppression",
		ID:        uuid.MustParse("66666666-6666-6666-6666-666666666666"),
		Email:     "user@example.com",
		Domain:    "example.com",
		Reason:    "unsubscribed",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		Protected: true,
	}
}

func TestSuppressionTable_ShowsProtected(t *testing.T) {
	suppression := newTestProtectedSuppression()

	var list, check bytes.Buffer
	require.NoError(t, testHandler(t, "table", &list, os.Stderr).HandleSuppressionList(
		&responses.PaginatedSuppressionsResponse{Data: []responses.Suppression{*suppression}},
		ListConfig{}))
	require.NoError(t, testHandler(t, "table", &check, os.Stderr).HandleCheckSuppression(
		&responses.PaginatedSuppressionsResponse{Data: []responses.Suppression{*suppression}}, CheckConfig{}))

	assert.Contains(t, list.String(), "PROTECTED")
	assert.Contains(t, list.String(), "Yes")
	assert.Contains(t, check.String(), "PROTECTED")
	assert.Contains(t, check.String(), "Yes")
}

// TestSuppressionCheckShowsEverySuppression checks that a check of an
// address suppressed on two domains shows both suppressions, each with its
// domain and whether it is protected, in table, plain and CSV output.
func TestSuppressionCheckShowsEverySuppression(t *testing.T) {
	first := *newTestProtectedSuppression()
	second := *newTestProtectedSuppression()
	second.ID = uuid.MustParse("77777777-7777-7777-7777-777777777777")
	second.Domain = "news.example.com"
	second.Reason = "bounce"
	second.Protected = false
	response := &responses.PaginatedSuppressionsResponse{Data: []responses.Suppression{first, second}}
	config := CheckConfig{FoundMessage: "Email user@example.com is suppressed"}
	render := func(h ResponseHandler) error { return h.HandleCheckSuppression(response, config) }

	t.Run("table", func(t *testing.T) {
		rows := tableCells(renderWith(t, "table", render))
		require.Len(t, rows, 3)
		domain := slices.Index(rows[0], "DOMAIN")
		protected := slices.Index(rows[0], "PROTECTED")
		assert.Equal(t, []string{"example.com", "Yes"}, []string{rows[1][domain], rows[1][protected]})
		assert.Equal(t, []string{"news.example.com", "No"}, []string{rows[2][domain], rows[2][protected]})
	})
	t.Run("plain", func(t *testing.T) {
		output := renderWith(t, "plain", render)
		assert.True(t, strings.HasPrefix(output, "Email user@example.com is suppressed\n"), output)
		assert.Contains(t, output, "Domain: example.com\n")
		assert.Contains(t, output, "Domain: news.example.com\n")
		assert.Equal(t, 2, strings.Count(output, "Protected: "), output)
	})
	t.Run("csv", func(t *testing.T) {
		_, rows := readCSV(t, renderWith(t, "csv", render))
		require.Len(t, rows, 2)
		assert.Equal(t, []string{"example.com", "true"}, []string{rows[0]["domain"], rows[0]["protected"]})
		assert.Equal(t, []string{"news.example.com", "false"}, []string{rows[1]["domain"], rows[1]["protected"]})
	})
}

func TestSuppressionPlain_ShowsProtected(t *testing.T) {
	suppression := newTestProtectedSuppression()

	var list, check bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &list, os.Stderr).HandleSuppressionList(
		&responses.PaginatedSuppressionsResponse{Data: []responses.Suppression{*suppression}}, ListConfig{}))
	require.NoError(t, testHandler(t, "plain", &check, os.Stderr).HandleCheckSuppression(
		&responses.PaginatedSuppressionsResponse{Data: []responses.Suppression{*suppression}}, CheckConfig{}))

	assert.Contains(t, list.String(), "\nProtected: Yes")
	assert.Contains(t, check.String(), "Protected: Yes")
}

func TestSuppressionJSON_IncludesProtected(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, testHandler(t, "json", &buf, os.Stderr).HandleSuppressionList(
		&responses.PaginatedSuppressionsResponse{Data: []responses.Suppression{*newTestProtectedSuppression()}}, ListConfig{}))

	var got responses.PaginatedSuppressionsResponse
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Len(t, got.Data, 1)
	assert.True(t, got.Data[0].Protected)
}

func TestSuppressionCSV_IncludesProtected(t *testing.T) {
	suppression := newTestProtectedSuppression()
	views := map[string]func(ResponseHandler) error{
		"list": func(h ResponseHandler) error {
			return h.HandleSuppressionList(&responses.PaginatedSuppressionsResponse{Data: []responses.Suppression{*suppression}}, ListConfig{})
		},
		"create": func(h ResponseHandler) error {
			return h.HandleCreateSuppression(&responses.CreateSuppressionResponse{Data: []responses.Suppression{*suppression}}, ResultConfig{})
		},
		"check": func(h ResponseHandler) error {
			return h.HandleCheckSuppression(
				&responses.PaginatedSuppressionsResponse{Data: []responses.Suppression{*suppression}}, CheckConfig{})
		},
	}

	for view, render := range views {
		t.Run(view, func(t *testing.T) {
			var buf bytes.Buffer
			require.NoError(t, render(testHandler(t, "csv", &buf, os.Stderr)))
			_, rows := readCSV(t, buf.String())
			require.Len(t, rows, 1)
			assert.Equal(t, "true", rows[0]["protected"])
		})
	}
}
