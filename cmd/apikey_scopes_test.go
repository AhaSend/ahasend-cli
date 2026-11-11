package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// TestKeyCommandsSendTheOtherStatisticsNameAsGiven checks that the commands
// that create and update an account's or a sub-account's API key send a
// scope under the other name the API accepts for the statistics scope, in
// both forms, in the request body as it was given: the API saves and returns
// a scope under the name it was sent with.
func TestKeyCommandsSendTheOtherStatisticsNameAsGiven(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(body))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"api_key","id":"` + rawTestOtherID + `","label":"ci"}`))
	}))
	t.Cleanup(server.Close)
	apiClient, err := client.NewClient("aha-sk-test", rawTestID, server.URL)
	require.NoError(t, err)
	testutil.InjectClient(t, apiClient)

	commands := [][]string{
		{"apikeys", "create", "--label", "ci"},
		{"apikeys", "update", rawTestOtherID},
		{"subaccounts", "api-keys", "create", rawTestID, "--label", "ci"},
		{"subaccounts", "api-keys", "update", rawTestID, rawTestOtherID},
	}
	for _, command := range commands {
		for _, scope := range []string{"statistics-transactional:read:all", "statistics-transactional:read:{Example.com}"} {
			t.Run(strings.Join(command, " ")+" "+scope, func(t *testing.T) {
				mu.Lock()
				bodies = nil
				mu.Unlock()

				_, stderr, code := runRoot(t, append(command, "--scope", scope)...)

				require.Equal(t, 0, code, "stderr: %s", stderr)
				mu.Lock()
				defer mu.Unlock()
				require.Len(t, bodies, 1)
				var sent struct {
					Scopes []string `json:"scopes"`
				}
				require.NoError(t, json.Unmarshal([]byte(bodies[0]), &sent))
				assert.Equal(t, []string{scope}, sent.Scopes)
			})
		}
	}
}
