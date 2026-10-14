package cmd

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// cursorAliasListCommands are the list commands that take the deprecated
// --cursor as well as --after and --before.
var cursorAliasListCommands = [][]string{
	{"apikeys", "list"},
	{"domains", "list"},
	{"messages", "list", "--sender", "sender@example.com"},
	{"routes", "list"},
	{"smtp", "list"},
	{"subaccounts", "list"},
	{"subaccounts", "api-keys", "list", rawTestID},
	{"suppressions", "list"},
	{"webhooks", "list"},
}

// TestListsSendTheCursorFlagsAsAfterAndBefore checks that each of these
// lists sends --after and --before as the API's after and before
// parameters, and the deprecated --cursor as after, never as the cursor
// parameter the API's v2 contract does not have.
func TestListsSendTheCursorFlagsAsAfterAndBefore(t *testing.T) {
	var mu sync.Mutex
	var query url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		query = r.URL.Query()
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[],"pagination":{"has_more":false}}`))
	}))
	t.Cleanup(server.Close)
	apiClient, err := client.NewClient("aha-sk-test", rawTestID, server.URL)
	require.NoError(t, err)
	testutil.InjectClient(t, apiClient)

	for _, args := range cursorAliasListCommands {
		for flag, param := range map[string]string{"--after": "after", "--before": "before", "--cursor": "after"} {
			t.Run(strings.Join(args, " ")+" "+flag, func(t *testing.T) {
				_, stderr, code := runRoot(t, append(args, flag, "page-2")...)

				require.Equal(t, 0, code, "stderr: %s", stderr)
				mu.Lock()
				defer mu.Unlock()
				assert.Equal(t, "page-2", query.Get(param))
				assert.False(t, query.Has("cursor"), "query: %v", query)
			})
		}
	}
}
