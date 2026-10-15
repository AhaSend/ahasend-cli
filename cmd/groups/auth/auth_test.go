package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAccountID = "00000000-0000-0000-0000-0000000000aa"

// otherAccountID is an account the test profiles' keys do not belong to.
const otherAccountID = "00000000-0000-0000-0000-0000000000bb"

// testAPI is a test AhaSend API that answers ping and account requests with
// status and records the paths it was asked for. Like the API, it answers
// /v2/ping for any key, and an account route with 401 when the key belongs
// to another account than the one in the path. A 403 is the key's scope
// refusal, which the API gives on the account route only.
type testAPI struct {
	URL   string
	mu    sync.Mutex
	paths []string
}

func newTestAPI(t *testing.T, status int) *testAPI {
	t.Helper()
	return newTestAPIForKeyOf(t, testAccountID, status)
}

// newTestAPIForKeyOf is newTestAPI for a key that belongs to keyAccount.
func newTestAPIForKeyOf(t *testing.T, keyAccount string, status int) *testAPI {
	t.Helper()
	api := &testAPI{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.mu.Lock()
		api.paths = append(api.paths, r.URL.Path)
		api.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch {
		case status == http.StatusForbidden && r.URL.Path != "/v2/ping":
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"message":"insufficient permissions for the required scope"}`))
		case status != http.StatusOK && status != http.StatusForbidden:
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"message":"invalid API key"}`))
		case r.URL.Path == "/v2/ping":
			_, _ = w.Write([]byte(`{"message":"OK"}`))
		case r.URL.Path != "/v2/accounts/"+keyAccount:
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"invalid API key"}`))
		default:
			_, _ = fmt.Fprintf(w, `{"object":"account","id":%q,"name":"Acme","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`, keyAccount)
		}
	}))
	t.Cleanup(server.Close)
	api.URL = server.URL
	return api
}

// newRefusingTestAPI is a test AhaSend API that answers every request with
// status and the API error message, as the API answers a key it refuses
// before it reads the path: from an IP address the key's allow-list leaves
// out (403), or of a paused account (401).
func newRefusingTestAPI(t *testing.T, status int, message string) *testAPI {
	t.Helper()
	api := &testAPI{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.mu.Lock()
		api.paths = append(api.paths, r.URL.Path)
		api.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = fmt.Fprintf(w, `{"message":%q}`, message)
	}))
	t.Cleanup(server.Close)
	api.URL = server.URL
	return api
}

// Paths returns the paths requested so far.
func (api *testAPI) Paths() []string {
	api.mu.Lock()
	defer api.mu.Unlock()
	return append([]string(nil), api.paths...)
}

// writeConfig points HOME at a new directory holding config.
func writeConfig(t *testing.T, config string) {
	t.Helper()
	testutil.WriteConfig(t, testutil.UseNewHome(t), config)
}

// writeProfiles writes a config with a default profile "default" and a
// second profile "production", both using apiURL.
func writeProfiles(t *testing.T, apiURL string) {
	t.Helper()
	testutil.WriteProfiles(t, testutil.UseNewHome(t), apiURL, testAccountID, "default", "production")
}

// TestAuthResultsShareOneShape checks that login, switch and logout print
// the same JSON object and CSV columns, with a row in every case.
func TestAuthResultsShareOneShape(t *testing.T) {
	tests := []struct {
		name    string
		command func() *cobra.Command
		args    []string
		profile string
		message string
	}{
		{"login", NewLoginCommand, []string{"--profile", "staging", "--api-key", "aha-sk-test", "--account-id", testAccountID},
			"staging", "Profile 'staging' authenticated and saved successfully. Profile 'default' is the default; run 'ahasend auth switch staging' to use this one"},
		{"switch", NewSwitchCommand, []string{"production"}, "production", "Switched to profile 'production'"},
		{"switch to the current profile", NewSwitchCommand, []string{"default"}, "default", "Already using profile 'default'"},
		{"logout", NewLogoutCommand, []string{"production"}, "production", "Logged out from profile 'production'"},
		{"logout all", NewLogoutCommand, []string{"--all", "--force"}, "", "Logged out from all profiles (2 removed)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := newTestAPI(t, http.StatusOK)
			writeProfiles(t, api.URL)
			args := tt.args
			if tt.name == "login" {
				args = append(args, "--api-url", api.URL)
			}

			stdout, _, err := testutil.RunCommand(t, tt.command(), "json", "", args...)
			require.NoError(t, err)
			assert.JSONEq(t, fmt.Sprintf(`{"profile":%q,"success":true,"message":%q}`, tt.profile, tt.message), stdout)

			writeProfiles(t, api.URL)
			stdout, _, err = testutil.RunCommand(t, tt.command(), "csv", "", args...)
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("profile,success,message\n%s,true,%s\n", tt.profile, tt.message), stdout)
		})
	}
}
