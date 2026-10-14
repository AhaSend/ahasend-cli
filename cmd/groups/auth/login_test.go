package auth

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/config"
	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/testenv"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Enhanced test cases for better coverage

func TestLoginCommand_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() (string, func())
		args      []string
		errorMsg  string
	}{
		{
			name: "invalid config directory setup",
			setupFunc: func() (string, func()) {
				tempDir := t.TempDir()
				// Create a file where directory should be
				badPath := filepath.Join(tempDir, ".ahasend")
				err := os.WriteFile(badPath, []byte("not a directory"), 0644)
				require.NoError(t, err)

				testenv.SetHome(t, tempDir)
				return tempDir, func() {}
			},
			args: []string{
				"--api-key", "test-key",
				"--account-id", "test-account",
			},
			errorMsg: "configuration", // Some config-related error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cleanup func()
			if tt.setupFunc != nil {
				_, cleanup = tt.setupFunc()
				defer cleanup()
			}

			cmd := NewLoginCommand()
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			assert.Error(t, err)
			if tt.errorMsg != "" {
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.errorMsg))
			}
		})
	}
}

func TestLoginCommand_DefaultValues(t *testing.T) {
	cmd := NewLoginCommand()

	// Test default values
	profile, _ := cmd.Flags().GetString("profile")
	assert.Empty(t, profile, "Profile should default to empty string")

	apiURL, _ := cmd.Flags().GetString("api-url")
	assert.Equal(t, "https://api.ahasend.com", apiURL, "API URL should have default value")

	apiKey, _ := cmd.Flags().GetString("api-key")
	assert.Empty(t, apiKey, "API key should default to empty string")

	accountID, _ := cmd.Flags().GetString("account-id")
	assert.Empty(t, accountID, "Account ID should default to empty string")
}

// Benchmark tests
func BenchmarkLoginCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewLoginCommand()
	}
}

func BenchmarkLoginCommand_FlagParsing(b *testing.B) {
	cmd := NewLoginCommand()
	args := []string{
		"--api-key", "test-key",
		"--account-id", "test-account",
		"--profile", "test-profile",
		"--api-url", "https://api.example.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := cmd.ParseFlags(args)
		if err != nil {
			b.Fatal(err)
		}
		// Reset flags for next iteration
		cmd.ResetFlags()
		cmd = NewLoginCommand()
	}
}

func TestLoginValidatesAgainstTheAPIURLAndSavesIt(t *testing.T) {
	testutil.UseNewHome(t)
	api := newTestAPI(t, http.StatusOK)

	_, err := testutil.RunCommandCombined(t, NewLoginCommand(), "plain",
		"--api-key", "aha-sk-test", "--account-id", testAccountID, "--api-url", api.URL)
	require.NoError(t, err)
	assert.Equal(t, []string{"/v2/accounts/" + testAccountID}, api.Paths())

	manager, err := config.NewManager()
	require.NoError(t, err)
	require.NoError(t, manager.Load())
	assert.Equal(t, api.URL, manager.GetConfig().Profiles["default"].APIURL)
}

// TestLoginAgainKeepsTheKeysOfTheProfileItReplaces checks that logging in
// to a profile again replaces its credentials and keeps the keys the CLI
// does not know, which another tool or a later version may have written.
func TestLoginAgainKeepsTheKeysOfTheProfileItReplaces(t *testing.T) {
	home := testutil.UseNewHome(t)
	api := newTestAPI(t, http.StatusOK)
	configPath := testutil.WriteConfig(t, home, "default_profile: work\nprofiles:\n  work:\n"+
		"    api_key: aha-sk-old\n    api_url: "+api.URL+"\n    account_id: "+testAccountID+"\n    team: ops\n")

	_, err := testutil.RunCommandCombined(t, NewLoginCommand(), "plain",
		"--profile", "work", "--api-key", "aha-sk-new", "--account-id", testAccountID, "--api-url", api.URL)
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "aha-sk-new")
	assert.Contains(t, string(data), "team: ops")
}

// TestLoginSavesNoProfileNameAndKeepsAnOldOne checks that a new profile gets
// no name key, which the CLI does not read, and that logging in to a profile
// again keeps the name an earlier version saved.
func TestLoginSavesNoProfileNameAndKeepsAnOldOne(t *testing.T) {
	home := testutil.UseNewHome(t)
	api := newTestAPI(t, http.StatusOK)
	configPath := testutil.WriteConfig(t, home, "default_profile: work\nprofiles:\n  work:\n"+
		"    api_key: aha-sk-old\n    api_url: "+api.URL+"\n    account_id: "+testAccountID+"\n    name: AhaSend work\n")

	for _, profile := range []string{"work", "personal"} {
		_, err := testutil.RunCommandCombined(t, NewLoginCommand(), "plain",
			"--profile", profile, "--api-key", "aha-sk-new", "--account-id", testAccountID, "--api-url", api.URL)
		require.NoError(t, err)
	}

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var saved struct {
		Profiles map[string]map[string]any `yaml:"profiles"`
	}
	require.NoError(t, yaml.Unmarshal(data, &saved))
	assert.Equal(t, "AhaSend work", saved.Profiles["work"]["name"])
	assert.Equal(t, "aha-sk-new", saved.Profiles["personal"]["api_key"])
	assert.NotContains(t, saved.Profiles["personal"], "name")
}

func TestLoginRejectedByTheAPISavesNoProfile(t *testing.T) {
	testutil.UseNewHome(t)
	api := newTestAPI(t, http.StatusUnauthorized)

	_, err := testutil.RunCommandCombined(t, NewLoginCommand(), "plain",
		"--api-key", "aha-sk-test", "--account-id", testAccountID, "--api-url", api.URL)
	require.Error(t, err)
	assert.Equal(t, []string{"/v2/accounts/" + testAccountID}, api.Paths())

	manager, err := config.NewManager()
	require.NoError(t, err)
	require.NoError(t, manager.Load())
	assert.Empty(t, manager.GetConfig().Profiles)
}

func TestLoginEndingInputAtAPromptCancels(t *testing.T) {
	for name, stdin := range map[string]string{"API key": "", "account ID": "aha-sk-test\n"} {
		t.Run(name, func(t *testing.T) {
			testutil.UseNewHome(t)
			api := newTestAPI(t, http.StatusOK)

			_, _, err := testutil.RunCommand(t, NewLoginCommand(), "plain", stdin, "--api-url", api.URL)

			require.Error(t, err)
			assert.Equal(t, "operation cancelled", err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.Empty(t, api.Paths())
		})
	}
}

func TestLoginPromptsOnStderrAndReadsPipedInput(t *testing.T) {
	testutil.UseNewHome(t)
	api := newTestAPI(t, http.StatusOK)

	stdout, stderr, err := testutil.RunCommand(t, NewLoginCommand(), "json",
		"aha-sk-test\n"+testAccountID+"\n", "--api-url", api.URL)

	require.NoError(t, err)
	assert.Equal(t, "Enter your AhaSend API key: \nEnter your AhaSend Account ID: \n", stderr)
	assert.NotContains(t, stdout, "Enter")
	assert.NotContains(t, stdout, "aha-sk-test")
	assert.Equal(t, []string{"/v2/accounts/" + testAccountID}, api.Paths())
}

// TestLoginDropsSpacesAroundAPromptedKey checks that spaces pasted around the
// API key at the prompt are not saved: API keys hold none.
func TestLoginDropsSpacesAroundAPromptedKey(t *testing.T) {
	testutil.UseNewHome(t)
	api := newTestAPI(t, http.StatusOK)

	_, _, err := testutil.RunCommand(t, NewLoginCommand(), "plain",
		"  aha-sk-test \n"+testAccountID+"\n", "--api-url", api.URL)
	require.NoError(t, err)

	manager, err := config.NewManager()
	require.NoError(t, err)
	require.NoError(t, manager.Load())
	assert.Equal(t, "aha-sk-test", manager.GetConfig().Profiles["default"].APIKey)
}

// TestLoginHidesTheAPIKeyOnATerminal checks that the API key prompt hides the
// key as it is typed.
func TestLoginHidesTheAPIKeyOnATerminal(t *testing.T) {
	testutil.UseNewHome(t)
	api := newTestAPI(t, http.StatusOK)
	terminal := testutil.NewTerminal(t)

	wait := terminal.Start(t, NewLoginCommand(), "plain", "--api-url", api.URL)
	terminal.TypeHidden(t, "aha-sk-test")
	terminal.Type(t, testAccountID)
	_, stderr, err := wait()

	require.NoError(t, err)
	assert.Equal(t, "Enter your AhaSend API key: \nEnter your AhaSend Account ID: ", stderr)
	assert.Equal(t, []string{"/v2/accounts/" + testAccountID}, api.Paths())
}

// TestLoginReadsAnEmptyAccountIDAfterATypedKey checks that Enter alone at
// the Account ID prompt, after a key typed and ended with Enter, fails as an
// empty Account ID rather than waiting for another line.
func TestLoginReadsAnEmptyAccountIDAfterATypedKey(t *testing.T) {
	testutil.UseNewHome(t)
	api := newTestAPI(t, http.StatusOK)
	terminal := testutil.NewTerminal(t)

	wait := terminal.Start(t, NewLoginCommand(), "plain", "--api-url", api.URL)
	terminal.PressHidden(t, "aha-sk-test\r")
	terminal.WaitForEcho(t)
	terminal.Type(t, "")
	_, _, err := wait()

	require.Error(t, err)
	assert.Equal(t, "--account-id is required", err.Error())
	assert.Empty(t, api.Paths())
}

// TestLoginDropsControlKeysTypedAtTheKeyPrompt checks that an arrow key
// typed at the hidden API key prompt does not end up in the saved key.
func TestLoginDropsControlKeysTypedAtTheKeyPrompt(t *testing.T) {
	testutil.UseNewHome(t)
	api := newTestAPI(t, http.StatusOK)
	terminal := testutil.NewTerminal(t)

	wait := terminal.Start(t, NewLoginCommand(), "plain", "--api-url", api.URL)
	terminal.TypeHidden(t, "aha-sk-te\x1b[Dst")
	terminal.Type(t, testAccountID)
	_, _, err := wait()

	require.NoError(t, err)
	manager, err := config.NewManager()
	require.NoError(t, err)
	require.NoError(t, manager.Load())
	assert.Equal(t, "aha-sk-test", manager.GetConfig().Profiles["default"].APIKey)
}

// TestLoginStopsAtAKeyItCannotUse checks that an empty API key, or one with
// a control or non-ASCII character, fails before the Account ID is asked
// for, with exit code 4 and no request.
func TestLoginStopsAtAKeyItCannotUse(t *testing.T) {
	tests := map[string]string{
		"":              "--api-key is required",
		"aha-sk-\u00e9": "the API key must be printable ASCII, without control or non-ASCII characters",
		"aha-sk-\x01":   "the API key must be printable ASCII, without control or non-ASCII characters",
	}
	for key, message := range tests {
		t.Run(message, func(t *testing.T) {
			testutil.UseNewHome(t)
			api := newTestAPI(t, http.StatusOK)

			_, stderr, err := testutil.RunCommand(t, NewLoginCommand(), "plain", key+"\n"+testAccountID+"\n", "--api-url", api.URL)

			require.Error(t, err)
			assert.Equal(t, message, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.NotContains(t, stderr, "Account ID")
			assert.Empty(t, api.Paths())
		})
	}
}

// TestLoginRejectsAnAPIURLThatIsNotHTTP checks that --api-url is checked
// before the command prompts or sends a request.
func TestLoginRejectsAnAPIURLThatIsNotHTTP(t *testing.T) {
	for _, apiURL := range []string{"notaurl", "ftp://api.example.com", "https://", "https://:8080"} {
		t.Run(apiURL, func(t *testing.T) {
			testutil.UseNewHome(t)

			_, stderr, err := testutil.RunCommand(t, NewLoginCommand(), "plain", "", "--api-url", apiURL)

			require.Error(t, err)
			assert.Equal(t, "--api-url must be an HTTP or HTTPS URL with a host, such as https://api.ahasend.com", err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.NotContains(t, stderr, "Enter your", "the command must not prompt")
		})
	}
}

// TestLoginWarnsOfAnHTTPAPIURL checks the warning for an API URL that sends
// the key in plain text to another machine. The warning comes before the
// prompts, so the run ends at the first prompt without sending a request;
// the host is under .invalid, which never resolves.
func TestLoginWarnsOfAnHTTPAPIURL(t *testing.T) {
	testutil.UseNewHome(t)

	_, stderr, err := testutil.RunCommand(t, NewLoginCommand(), "plain", "",
		"--api-url", "http://api.ahasend.invalid")

	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.True(t, strings.HasPrefix(stderr, "Warning: the API URL http://api.ahasend.invalid uses plain HTTP, "+
		"so the API key is sent unencrypted. Use HTTPS.\nEnter your AhaSend API key: "), stderr)
}

// TestLoginRejectsAnAPIURLWithAPath checks that --api-url takes a host only:
// the API's paths start at the root, so a path would be lost.
func TestLoginRejectsAnAPIURLWithAPath(t *testing.T) {
	for _, apiURL := range []string{"http://127.0.0.1:9/prefix", "http://127.0.0.1:9?x=1", "http://127.0.0.1:9#top"} {
		t.Run(apiURL, func(t *testing.T) {
			testutil.UseNewHome(t)

			_, stderr, err := testutil.RunCommand(t, NewLoginCommand(), "plain", "", "--api-url", apiURL)

			require.Error(t, err)
			assert.Equal(t, "--api-url must have no path, query or fragment, such as https://api.ahasend.com", err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.NotContains(t, stderr, "Enter your", "the command must not prompt")
		})
	}
}

// TestLoginRejectsAnAPIURLWithCredentials checks that --api-url cannot hold a
// user name or password, which the CLI would never send.
func TestLoginRejectsAnAPIURLWithCredentials(t *testing.T) {
	for _, apiURL := range []string{"https://user:pass@127.0.0.1:9", "https://user@127.0.0.1:9"} {
		t.Run(apiURL, func(t *testing.T) {
			testutil.UseNewHome(t)

			_, stderr, err := testutil.RunCommand(t, NewLoginCommand(), "plain", "", "--api-url", apiURL)

			require.Error(t, err)
			assert.Equal(t, "--api-url must have no user name or password, such as https://api.ahasend.com", err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.NotContains(t, stderr, "Enter your", "the command must not prompt")
		})
	}
}

// TestLoginRefusesAKeyOfAnotherAccount checks that login reads the account,
// which the API refuses to a key of another account, rather than ping,
// which it answers for any key.
func TestLoginRefusesAKeyOfAnotherAccount(t *testing.T) {
	testutil.UseNewHome(t)
	api := newTestAPIForKeyOf(t, otherAccountID, http.StatusOK)

	_, err := testutil.RunCommandCombined(t, NewLoginCommand(), "plain",
		"--api-key", "aha-sk-test", "--account-id", testAccountID, "--api-url", api.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "the API key is not valid for account "+testAccountID+"; check both")
	assert.Equal(t, 2, clierrors.GetExitCode(err))

	manager, err := config.NewManager()
	require.NoError(t, err)
	require.NoError(t, manager.Load())
	assert.Empty(t, manager.GetConfig().Profiles)
}

// TestLoginSavesAKeyThatMayNotReadTheAccount checks that a 403 from the
// account route, which the API answers only once the key matched the
// account, passes the check.
func TestLoginSavesAKeyThatMayNotReadTheAccount(t *testing.T) {
	testutil.UseNewHome(t)
	api := newTestAPI(t, http.StatusForbidden)

	_, err := testutil.RunCommandCombined(t, NewLoginCommand(), "plain",
		"--api-key", "aha-sk-test", "--account-id", testAccountID, "--api-url", api.URL)
	require.NoError(t, err)

	manager, err := config.NewManager()
	require.NoError(t, err)
	require.NoError(t, manager.Load())
	assert.Contains(t, manager.GetConfig().Profiles, "default")
}

// TestLoginRefusesAKeyRefusedFromThisIP checks that a 403 the API gives on
// every route, as it does for an IP address the key's allow-list leaves
// out, is not taken for the scope refusal of the account route.
func TestLoginRefusesAKeyRefusedFromThisIP(t *testing.T) {
	testutil.UseNewHome(t)
	api := newRefusingTestAPI(t, http.StatusForbidden, "source IP 192.0.2.1 is not allowed for this API key")

	_, err := testutil.RunCommandCombined(t, NewLoginCommand(), "plain",
		"--api-key", "aha-sk-test", "--account-id", testAccountID, "--api-url", api.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "the API refuses the API key from this IP address: ")
	assert.Contains(t, err.Error(), "source IP 192.0.2.1 is not allowed for this API key")
	assert.Equal(t, 6, clierrors.GetExitCode(err))
	assert.Equal(t, []string{"/v2/accounts/" + testAccountID, "/v2/ping"}, api.Paths())

	manager, err := config.NewManager()
	require.NoError(t, err)
	require.NoError(t, manager.Load())
	assert.Empty(t, manager.GetConfig().Profiles)
}

// TestLoginShowsTheAPIReasonForAnotherRefusal checks that a 401 for another
// reason than an invalid key, such as a paused account, gives the API's
// reason rather than asking to check the key and account ID.
func TestLoginShowsTheAPIReasonForAnotherRefusal(t *testing.T) {
	testutil.UseNewHome(t)
	api := newRefusingTestAPI(t, http.StatusUnauthorized, "account is paused")

	_, err := testutil.RunCommandCombined(t, NewLoginCommand(), "plain",
		"--api-key", "aha-sk-test", "--account-id", testAccountID, "--api-url", api.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "the API refused the API key for account "+testAccountID+": ")
	assert.Contains(t, err.Error(), "account is paused")
	assert.NotContains(t, err.Error(), "check both")
	assert.Equal(t, 2, clierrors.GetExitCode(err))
}
