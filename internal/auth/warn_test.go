package auth_test

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// TestGetAuthenticatedClientWarnsOnceOfAnUnencryptedAPIKey checks that a
// profile whose API URL is plain HTTP to another host gets one warning per
// run, however many clients the command makes, and a loopback one none.
func TestGetAuthenticatedClientWarnsOnceOfAnUnencryptedAPIKey(t *testing.T) {
	tests := map[string]struct {
		apiURL string
		want   string
	}{
		"another host": {"http://203.0.113.10", "Warning: the API URL http://203.0.113.10 uses plain HTTP, " +
			"so the API key is sent unencrypted. Use HTTPS.\n"},
		"loopback": {"http://127.0.0.1:9", ""},
		"https":    {"https://203.0.113.10", ""},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			home := testutil.UseNewHome(t)
			testutil.WriteProfiles(t, home, test.apiURL, "11111111-1111-4111-8111-111111111111", "default")
			cmd := &cobra.Command{Use: "test"}
			cmd.Flags().String("api-key", "", "")
			cmd.Flags().String("account-id", "", "")
			cmd.Flags().String("profile", "", "")
			var stderr bytes.Buffer
			cmd.SetErr(&stderr)

			for range 2 {
				_, err := auth.GetAuthenticatedClient(cmd)
				require.NoError(t, err)
			}

			assert.Equal(t, test.want, stderr.String())
		})
	}
}

// TestProfileAuthLogNamesTheProfileKey checks that the debug log of profile
// authentication names the profile by the key commands use, not by its
// display name.
func TestProfileAuthLogNamesTheProfileKey(t *testing.T) {
	home := testutil.UseNewHome(t)
	testutil.WriteConfig(t, home, "default_profile: work\nprofiles:\n  work:\n    name: AhaSend Work Display\n"+
		"    api_key: aha-sk-work\n    api_url: http://127.0.0.1:1\n    account_id: 11111111-1111-4111-8111-111111111111\n")
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("api-key", "", "")
	cmd.Flags().String("account-id", "", "")
	cmd.Flags().String("profile", "", "")
	cmd.Flags().Bool("debug", true, "")
	cmd.Flags().Bool("verbose", false, "")
	cmd.Flags().Bool("no-color", true, "")
	var log bytes.Buffer
	cmd.SetErr(&log)
	logger.Initialize(cmd)
	t.Cleanup(func() { logger.Initialize(&cobra.Command{}) })

	_, err := auth.GetAuthenticatedClient(cmd)

	require.NoError(t, err)
	assert.Contains(t, log.String(), "profile=work")
	assert.NotContains(t, log.String(), "Display")
}
