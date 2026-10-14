package printer

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// apiKeyAllowing returns the fixture API key with ipAllowList.
func apiKeyAllowing(ipAllowList []string) *responses.APIKey {
	key := mocks.NewAPIKey()
	key.IPAllowList = ipAllowList
	return key
}

// renderAPIKeyViews renders key through the list, single, create and update
// methods of a format, keyed by view name.
func renderAPIKeyViews(t *testing.T, format string, key *responses.APIKey) map[string]string {
	t.Helper()
	views := map[string]func(ResponseHandler) error{
		"list": func(h ResponseHandler) error {
			return h.HandleAPIKeyList(&responses.PaginatedAPIKeysResponse{Data: []responses.APIKey{*key}}, ListConfig{})
		},
		"single": func(h ResponseHandler) error { return h.HandleSingleAPIKey(key, SingleConfig{}) },
		"create": func(h ResponseHandler) error { return h.HandleCreateAPIKey(key, ResultConfig{}) },
		"update": func(h ResponseHandler) error { return h.HandleUpdateAPIKey(key, ResultConfig{}) },
	}

	out := make(map[string]string, len(views))
	for name, render := range views {
		var buf bytes.Buffer
		require.NoError(t, render(testHandler(t, format, &buf, os.Stderr)), name)
		out[name] = buf.String()
	}
	return out
}

func TestAPIKeyViews_ShowIPAllowList(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		for view, out := range renderAPIKeyViews(t, format, apiKeyAllowing([]string{"203.0.113.0/24", "2001:db8::1"})) {
			if format == "table" && view == "list" {
				continue // a table of keys shows its summary columns
			}
			t.Run(format+"/"+view, func(t *testing.T) {
				assert.Regexp(t, `(?i)ip allow list`, out)
				assert.Contains(t, out, "203.0.113.0/24, 2001:db8::1")
			})
		}
	}
}

func TestAPIKeyViews_EmptyIPAllowListShowsAny(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		for view, out := range renderAPIKeyViews(t, format, apiKeyAllowing(nil)) {
			if format == "table" && view == "list" {
				continue // a table of keys shows its summary columns
			}
			t.Run(format+"/"+view, func(t *testing.T) {
				assert.Regexp(t, `(?i)ip allow list`, out)
				assert.Regexp(t, `\bany\b`, out)
			})
		}
	}
}

func TestAPIKeyJSON_IncludesIPAllowList(t *testing.T) {
	var buf bytes.Buffer
	key := apiKeyAllowing([]string{"203.0.113.0/24"})
	require.NoError(t, testHandler(t, "json", &buf, os.Stderr).HandleSingleAPIKey(key, SingleConfig{}))

	var got responses.APIKey
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, []string{"203.0.113.0/24"}, got.IPAllowList)
	assert.Contains(t, buf.String(), `"ip_allow_list"`)
}

func TestAPIKeyCSV_IncludesIPAllowList(t *testing.T) {
	for view, out := range renderAPIKeyViews(t, "csv", apiKeyAllowing([]string{"203.0.113.0/24", "2001:db8::1"})) {
		t.Run(view, func(t *testing.T) {
			_, rows := readCSV(t, out)
			require.Len(t, rows, 1)
			assert.Equal(t, "203.0.113.0/24, 2001:db8::1", rows[0]["ip_allow_list"])
		})
	}
}
