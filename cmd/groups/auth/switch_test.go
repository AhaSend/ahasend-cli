package auth

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/config"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSwitchCommand_Args(t *testing.T) {
	// Test that switch command has args validation
	switchCmd := NewSwitchCommand()
	assert.NotNil(t, switchCmd.Args)
}

func TestSwitchCommand_ArgumentValidation(t *testing.T) {
	cmd := NewSwitchCommand()

	tests := []struct {
		name      string
		args      []string
		expectErr bool
	}{
		{
			name:      "valid profile name",
			args:      []string{"production"},
			expectErr: false,
		},
		{
			name:      "no arguments",
			args:      []string{},
			expectErr: true,
		},
		{
			name:      "too many arguments",
			args:      []string{"profile1", "profile2"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cmd.Args(cmd, tt.args)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Benchmark tests
func BenchmarkSwitchCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewSwitchCommand()
	}
}

func TestSwitchChecksTheProfileAgainstItsAPIURL(t *testing.T) {
	api := newTestAPI(t, http.StatusOK)
	writeProfiles(t, api.URL)

	_, err := testutil.RunCommandCombined(t, NewSwitchCommand(), "plain", "production")
	require.NoError(t, err)
	assert.Equal(t, []string{"/v2/accounts/" + testAccountID}, api.Paths())

	manager, err := config.NewManager()
	require.NoError(t, err)
	require.NoError(t, manager.Load())
	assert.Equal(t, "production", manager.GetConfig().DefaultProfile)
}

func TestSwitchWithoutAnAPICall(t *testing.T) {
	tests := []struct {
		profile string
		err     string
	}{
		{profile: "default"},
		{profile: "staging", err: "profile 'staging' not found"},
	}
	for _, test := range tests {
		t.Run(test.profile, func(t *testing.T) {
			api := newTestAPI(t, http.StatusOK)
			writeProfiles(t, api.URL)

			_, err := testutil.RunCommandCombined(t, NewSwitchCommand(), "plain", test.profile)
			if test.err == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.err)
			}
			assert.Empty(t, api.Paths())
		})
	}
}

func TestSwitchPrintsTheSuccessOnce(t *testing.T) {
	api := newTestAPI(t, http.StatusOK)
	writeProfiles(t, api.URL)

	stdout, stderr, err := testutil.RunCommand(t, NewSwitchCommand(), "plain", "", "production")
	require.NoError(t, err)
	assert.Equal(t, "Switched to profile 'production'\n", stdout)
	assert.Empty(t, stderr)
}

func TestSwitchWarnsWhenTheKeyFailsItsCheckAndSwitchesAnyway(t *testing.T) {
	api := newTestAPI(t, http.StatusUnauthorized)
	writeProfiles(t, api.URL)

	stdout, stderr, err := testutil.RunCommand(t, NewSwitchCommand(), "plain", "", "production")
	require.NoError(t, err)
	assert.Equal(t, "Switched to profile 'production'\n", stdout)
	assert.True(t, strings.HasPrefix(stderr, "Warning: checking the key of profile 'production' failed: "), stderr)

	manager, err := config.NewManager()
	require.NoError(t, err)
	require.NoError(t, manager.Load())
	assert.Equal(t, "production", manager.GetConfig().DefaultProfile)
}

func TestSwitchLogsTheProfileItSwitchedFrom(t *testing.T) {
	api := newTestAPI(t, http.StatusOK)
	writeProfiles(t, api.URL)
	var logs bytes.Buffer
	debug := &cobra.Command{}
	debug.Flags().Bool("debug", true, "")
	debug.Flags().Bool("no-color", true, "")
	debug.SetErr(&logs)
	logger.Initialize(debug)
	t.Cleanup(func() { logger.Initialize(&cobra.Command{}) })

	_, _, err := testutil.RunCommand(t, NewSwitchCommand(), "plain", "", "production")

	require.NoError(t, err)
	assert.Contains(t, logs.String(), "from_profile=default operation=switch_complete")
}

func TestSwitchWarnsOfAKeyOfAnotherAccount(t *testing.T) {
	api := newTestAPIForKeyOf(t, otherAccountID, http.StatusOK)
	writeProfiles(t, api.URL)

	_, stderr, err := testutil.RunCommand(t, NewSwitchCommand(), "plain", "", "production")
	require.NoError(t, err)
	assert.Contains(t, stderr, "the API key is not valid for account "+testAccountID)
}

func TestSwitchWarningEscapesTheAPIMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"bad \u001b]0;title\u0007 key"}`))
	}))
	t.Cleanup(server.Close)
	writeProfiles(t, server.URL)

	_, stderr, err := testutil.RunCommand(t, NewSwitchCommand(), "plain", "", "production")
	require.NoError(t, err)
	assert.Contains(t, stderr, `bad \u001b]0;title\u0007 key`)
	assert.NotContains(t, stderr, "\x1b")
}
