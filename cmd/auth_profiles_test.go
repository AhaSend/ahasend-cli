package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// TestAMissingDefaultProfileIsAConfigErrorEverywhere checks that a config
// with profiles but no usable default fails every command that needs the
// default profile with the configuration exit code.
func TestAMissingDefaultProfileIsAConfigErrorEverywhere(t *testing.T) {
	for defaultName, message := range map[string]string{
		`""`:   "no default profile set. Run 'ahasend auth switch <profile>' to choose one",
		"gone": "default profile 'gone' not found. Run 'ahasend auth switch <profile>' to choose one",
	} {
		for _, args := range [][]string{{"auth", "status"}, {"domains", "list"}} {
			t.Run(defaultName+" "+strings.Join(args, " "), func(t *testing.T) {
				testutil.WriteConfig(t, testutil.UseNewHome(t),
					"default_profile: "+defaultName+"\nprofiles:\n  staging:\n    name: staging\n    api_key: aha-sk-staging\n"+
						"    api_url: http://127.0.0.1:1\n    account_id: "+rawTestID+"\n")

				stdout, stderr, code := runRoot(t, args...)

				assert.Equal(t, 3, code, "stderr: %s", stderr)
				assert.Equal(t, "Error: "+message+"\n", stderr)
				assert.Empty(t, stdout)
			})
		}
	}
	t.Run("auth logout", func(t *testing.T) {
		testutil.WriteConfig(t, testutil.UseNewHome(t),
			"default_profile: \"\"\nprofiles:\n  staging:\n    name: staging\n    api_key: aha-sk-staging\n"+
				"    api_url: http://127.0.0.1:1\n    account_id: "+rawTestID+"\n")

		_, stderr, code := runRoot(t, "auth", "logout")

		assert.Equal(t, 3, code, "stderr: %s", stderr)
	})
}

// TestWithoutProfilesCommandsAskToLogIn checks that a command that needs a
// profile, run before any login, exits 2 and says how to log in.
func TestWithoutProfilesCommandsAskToLogIn(t *testing.T) {
	testutil.UseNewHome(t)

	_, stderr, code := runRoot(t, "domains", "list")

	assert.Equal(t, 2, code)
	assert.Equal(t, "Error: no profiles found. Run 'ahasend auth login' to authenticate\n", stderr)
}

// TestAProfileAPIURLThatIsNotAnAPIURLIsAConfigError checks that a profile
// whose api_url was edited into something the CLI cannot send to fails when
// the profile is resolved, before any request.
func TestAProfileAPIURLThatIsNotAnAPIURLIsAConfigError(t *testing.T) {
	for apiURL, reason := range map[string]string{
		"ftp://127.0.0.1:1":         "must be an HTTP or HTTPS URL with a host, such as https://api.ahasend.com",
		"htp://127.0.0.1:1":         "must be an HTTP or HTTPS URL with a host, such as https://api.ahasend.com",
		"http://127.0.0.1:1/prefix": "must have no path, query or fragment, such as https://api.ahasend.com",
		"http://127.0.0.1:1#top":    "must have no path, query or fragment, such as https://api.ahasend.com",
		"http://u:p@127.0.0.1:1":    "must have no user name or password, such as https://api.ahasend.com",
	} {
		for _, args := range [][]string{{"domains", "list"}, {"auth", "status"}, {"auth", "switch", "work"}} {
			t.Run(apiURL+" "+strings.Join(args, " "), func(t *testing.T) {
				home := testutil.UseNewHome(t)
				testutil.WriteProfiles(t, home, apiURL, rawTestID, "personal", "work")

				_, stderr, code := runRoot(t, args...)

				assert.Equal(t, 3, code, "stderr: %s", stderr)
				profile := "personal"
				if args[1] == "switch" {
					profile = "work"
				}
				assert.Equal(t, fmt.Sprintf("Error: the api_url %q of profile '%s' %s\n", apiURL, profile, reason), stderr)
			})
		}
	}
}

// TestAProfileWithoutAKeyOrAccountIDIsAConfigError checks that a profile
// whose api_key is empty or whose account_id is not a UUID fails when the
// profile is resolved, before any request, naming the profile and the field.
func TestAProfileWithoutAKeyOrAccountIDIsAConfigError(t *testing.T) {
	apiURL := newPingAPI(t)
	tests := map[string]struct {
		apiKey, accountID, message string
	}{
		"empty api_key": {`""`, rawTestID,
			"profile 'work' has no api_key. Run 'ahasend auth login --profile work' to log in again"},
		"account_id not a UUID": {"aha-sk-work", "not-a-uuid",
			`the account_id "not-a-uuid" of profile 'work' is not a UUID. Run 'ahasend auth login --profile work' to log in again`},
		"empty account_id": {"aha-sk-work", `""`,
			`the account_id "" of profile 'work' is not a UUID. Run 'ahasend auth login --profile work' to log in again`},
	}
	for name, test := range tests {
		// switch checks the profile it switches to, not the default
		for _, command := range []struct {
			defaultProfile string
			args           []string
		}{
			{"work", []string{"ping"}},
			{"work", []string{"auth", "status"}},
			{"personal", []string{"auth", "switch", "work"}},
		} {
			t.Run(name+" "+strings.Join(command.args, " "), func(t *testing.T) {
				testutil.WriteConfig(t, testutil.UseNewHome(t), fmt.Sprintf(
					"default_profile: %[1]s\nprofiles:\n  personal:\n    name: personal\n    api_key: aha-sk-personal\n    api_url: %[2]s\n    account_id: %[3]s\n"+
						"  work:\n    name: work\n    api_key: %[4]s\n    api_url: %[2]s\n    account_id: %[5]s\n",
					command.defaultProfile, apiURL, rawTestID, test.apiKey, test.accountID))

				stdout, stderr, code := runRoot(t, command.args...)

				assert.Equal(t, 3, code, "stderr: %s", stderr)
				assert.Equal(t, "Error: "+test.message+"\n", stderr)
				assert.Empty(t, stdout)
			})
		}
	}
}

// TestAuthCommandsRejectTheCredentialFlags checks that the auth commands
// that work on stored profiles refuse --api-key and --account-id, which
// they would otherwise ignore.
func TestAuthCommandsRejectTheCredentialFlags(t *testing.T) {
	for _, args := range [][]string{{"auth", "status"}, {"auth", "switch", "work"}, {"auth", "logout"}} {
		for _, flag := range []string{"--api-key", "--account-id"} {
			t.Run(strings.Join(args, " ")+" "+flag, func(t *testing.T) {
				home := testutil.UseNewHome(t)
				testutil.WriteProfiles(t, home, "http://127.0.0.1:1", rawTestID, "personal", "work")

				value := "aha-sk-test"
				if flag == "--account-id" {
					value = rawTestID
				}
				_, stderr, code := runRoot(t, append(args, flag, value)...)

				assert.Equal(t, 4, code, "stderr: %s", stderr)
				assert.Equal(t, fmt.Sprintf("Error: %s cannot be used with auth %s\n", flag, args[1]), stderr)
			})
		}
	}
}

// TestLogoutWithoutProfilesHasNothingToDo checks that auth logout, like
// auth status, says there are no profiles and exits with 0.
func TestLogoutWithoutProfilesHasNothingToDo(t *testing.T) {
	for _, args := range [][]string{{"auth", "logout"}, {"auth", "logout", "--all"}, {"auth", "status"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			testutil.UseNewHome(t)

			stdout, stderr, code := runRoot(t, args...)

			assert.Equal(t, 0, code, "stderr: %s", stderr)
			want := "No authentication profiles found\n"
			if args[1] == "status" {
				want += "Run 'ahasend auth login' to add one.\n"
			}
			assert.Equal(t, want, stdout)
		})
	}
}

// TestLogoutOfTheDefaultProfile checks which profile becomes the default
// when the default one logs out: the only one left, or none of several.
func TestLogoutOfTheDefaultProfile(t *testing.T) {
	tests := map[string]struct {
		profiles    []string
		message     string
		wantDefault string
	}{
		"one left": {[]string{"personal", "work"},
			"Logged out from profile 'personal'. Profile 'work' is now the default\n", "work"},
		"several left": {[]string{"personal", "work", "staging"},
			"Logged out from profile 'personal'. No profile is the default now; run 'ahasend auth switch <profile>' to choose one\n", ""},
		"none left": {[]string{"personal"}, "Logged out from profile 'personal'\n", ""},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			home := testutil.UseNewHome(t)
			configPath := testutil.WriteProfiles(t, home, "http://127.0.0.1:1", rawTestID, test.profiles...)

			stdout, stderr, code := runRoot(t, "auth", "logout")

			require.Equal(t, 0, code, "stderr: %s", stderr)
			assert.Equal(t, test.message, stdout)
			assert.Equal(t, test.wantDefault, readConfig(t, configPath).DefaultProfile)
		})
	}
}

// TestLogoutWithProfileLogsOutThatProfile checks that auth logout --profile
// logs out the named profile, the same as the argument, and keeps the
// default.
func TestLogoutWithProfileLogsOutThatProfile(t *testing.T) {
	for _, args := range [][]string{{"--profile", "work"}, {"work", "--profile", "work"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			home := testutil.UseNewHome(t)
			configPath := testutil.WriteProfiles(t, home, "http://127.0.0.1:1", rawTestID, "personal", "work")

			stdout, stderr, code := runRoot(t, append([]string{"auth", "logout"}, args...)...)

			require.Equal(t, 0, code, "stderr: %s", stderr)
			assert.Equal(t, "Logged out from profile 'work'\n", stdout)
			config := readConfig(t, configPath)
			assert.Equal(t, "personal", config.DefaultProfile)
			assert.Contains(t, config.Profiles, "personal")
			assert.NotContains(t, config.Profiles, "work")
		})
	}
}

// TestLogoutAndSwitchRejectConflictingProfiles checks that auth logout
// refuses to be told two different profiles, or a profile and --all, and
// that auth switch refuses --profile, before touching the config.
func TestLogoutAndSwitchRejectConflictingProfiles(t *testing.T) {
	tests := map[string]struct {
		args    []string
		message string
	}{
		"argument and another --profile": {[]string{"auth", "logout", "work", "--profile", "personal"},
			`the profile argument "work" and --profile "personal" cannot be used together`},
		"--all and an argument": {[]string{"auth", "logout", "--all", "work"},
			"--all and a profile argument cannot be used together"},
		"--all and --profile": {[]string{"auth", "logout", "--all", "--profile", "work"},
			"--all and --profile cannot be used together"},
		"switch with --profile": {[]string{"auth", "switch", "work", "--profile", "personal"},
			"--profile cannot be used with auth switch; give the profile as the argument"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			home := testutil.UseNewHome(t)
			configPath := testutil.WriteProfiles(t, home, "http://127.0.0.1:1", rawTestID, "personal", "work")
			before := readFile(t, configPath)

			stdout, stderr, code := runRoot(t, test.args...)

			assert.Equal(t, 4, code, "stderr: %s", stderr)
			assert.Equal(t, "Error: "+test.message+"\n", stderr)
			assert.Empty(t, stdout)
			assert.Equal(t, before, readFile(t, configPath))
		})
	}
}

// readFile returns the content of the file at path.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

// TestLoginMakesTheProfileTheDefaultOnlyWithoutOne checks that login sets
// the default profile only when there is none, whatever the new profile is
// called, and says which profile stays the default otherwise.
func TestLoginMakesTheProfileTheDefaultOnlyWithoutOne(t *testing.T) {
	apiURL := newPingAPI(t)
	tests := map[string]struct {
		config      string
		profile     string
		message     string
		wantDefault string
	}{
		"first profile": {"", "work",
			"Profile 'work' authenticated and saved successfully\n", "work"},
		"default names no profile": {"default_profile: gone\nprofiles:\n  personal:\n    api_key: aha-sk-personal\n    api_url: " + apiURL + "\n    account_id: " + rawTestID + "\n",
			"default", "Profile 'default' authenticated and saved successfully\n", "default"},
		"another profile is the default": {"default_profile: personal\nprofiles:\n  personal:\n    api_key: aha-sk-personal\n    api_url: " + apiURL + "\n    account_id: " + rawTestID + "\n",
			"default", "Profile 'default' authenticated and saved successfully. Profile 'personal' is the default; run 'ahasend auth switch default' to use this one\n", "personal"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			home := testutil.UseNewHome(t)
			configPath := filepath.Join(home, ".ahasend", "config.yaml")
			if test.config != "" {
				testutil.WriteConfig(t, home, test.config)
			}

			stdout, stderr, code := runRoot(t, "auth", "login", "--profile", test.profile,
				"--api-key", "aha-sk-new", "--account-id", rawTestID, "--api-url", apiURL)

			require.Equal(t, 0, code, "stderr: %s", stderr)
			assert.Equal(t, test.message, stdout)
			assert.Equal(t, test.wantDefault, readConfig(t, configPath).DefaultProfile)
		})
	}
}

// savedConfig is the part of the config file the auth tests check.
type savedConfig struct {
	DefaultProfile string `yaml:"default_profile"`
	Profiles       map[string]struct {
		APIKey string `yaml:"api_key"`
		APIURL string `yaml:"api_url"`
	} `yaml:"profiles"`
}

func readConfig(t *testing.T, path string) savedConfig {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var config savedConfig
	require.NoError(t, yaml.Unmarshal(data, &config))
	return config
}

// newPingAPI returns the URL of a local API that answers ping and the
// account request.
func newPingAPI(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v2/ping" {
			_, _ = w.Write([]byte(`{"message":"OK"}`))
			return
		}
		_, _ = fmt.Fprintf(w, `{"object":"account","id":%q,"name":"Acme","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`, rawTestID)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

// TestProfileNamesKeepTheirCaseAndDots checks that profile names with
// capitals and dots are saved, found and kept exactly through login, status,
// switch and later saves.
func TestProfileNamesKeepTheirCaseAndDots(t *testing.T) {
	home := testutil.UseNewHome(t)
	apiURL := newPingAPI(t)
	login := func(profile string) {
		t.Helper()
		_, stderr, code := runRoot(t, "auth", "login", "--profile", profile,
			"--api-key", "aha-sk-"+profile, "--account-id", rawTestID, "--api-url", apiURL)
		require.Equal(t, 0, code, "stderr: %s", stderr)
	}

	login("Work")
	login("prod.eu")
	for _, profile := range []string{"Work", "prod.eu"} {
		_, stderr, code := runRoot(t, "auth", "status", "--profile", profile)
		assert.Equal(t, 0, code, "auth status --profile %s: %s", profile, stderr)
	}
	_, stderr, code := runRoot(t, "auth", "switch", "prod.eu")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	_, stderr, code = runRoot(t, "auth", "switch", "Work")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	config := readConfig(t, filepath.Join(home, ".ahasend", "config.yaml"))
	assert.Equal(t, "Work", config.DefaultProfile)
	require.Len(t, config.Profiles, 2)
	assert.Equal(t, "aha-sk-Work", config.Profiles["Work"].APIKey)
	assert.Equal(t, "aha-sk-prod.eu", config.Profiles["prod.eu"].APIKey)
	assert.Equal(t, apiURL, config.Profiles["prod.eu"].APIURL)
}

// TestTheEnvironmentDoesNotChooseTheProfile checks that no environment
// variable overrides the default profile or reaches the saved config.
func TestTheEnvironmentDoesNotChooseTheProfile(t *testing.T) {
	home := testutil.UseNewHome(t)
	configPath := testutil.WriteProfiles(t, home, "http://127.0.0.1:1", rawTestID, "personal", "work")
	t.Setenv("AHASEND_DEFAULT_PROFILE", "work")

	stdout, stderr, code := runRoot(t, "auth", "logout", "work")

	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, "Logged out from profile 'work'\n", stdout)
	assert.Equal(t, "personal", readConfig(t, configPath).DefaultProfile)
}

// unreachableHint is what an error line says to check when the API could not
// be reached.
const unreachableHint = " (check the network connection and the API URL: auth login --api-url or the profile's api_url)"

// TestAuthStatusPrintsAnUnreachableAPIOnce checks that auth status, whose
// check of the key cannot reach the API, prints the failure as one error
// line with what to check, as other commands do, and not in its status too.
func TestAuthStatusPrintsAnUnreachableAPIOnce(t *testing.T) {
	t.Cleanup(client.ShortenRetryDelaysForTesting())
	testutil.WriteProfiles(t, testutil.UseNewHome(t), testutil.HangUpURL(t), rawTestID, "default")

	stdout, stderr, code := runRoot(t, "auth", "status")

	assert.Equal(t, 7, code)
	assert.NotContains(t, stdout, "Error")
	assert.Equal(t, 1, strings.Count(stderr, "\n"), stderr)
	assert.True(t, strings.HasPrefix(stderr, "Error: credentials of profile 'default' could not be verified: "), stderr)
	assert.True(t, strings.HasSuffix(stderr, unreachableHint+"\n"), stderr)
}

// TestAuthSwitchSaysWhatToCheckWhenTheAPIIsUnreachable checks that the
// warning of a key check that cannot reach the API says what to check.
func TestAuthSwitchSaysWhatToCheckWhenTheAPIIsUnreachable(t *testing.T) {
	t.Cleanup(client.ShortenRetryDelaysForTesting())
	testutil.WriteProfiles(t, testutil.UseNewHome(t), testutil.HangUpURL(t), rawTestID, "default", "staging")

	_, stderr, code := runRoot(t, "auth", "switch", "staging")

	assert.Equal(t, 0, code)
	assert.True(t, strings.HasPrefix(stderr, "Warning: checking the key of profile 'staging' failed: "), stderr)
	assert.True(t, strings.HasSuffix(stderr, unreachableHint+"\n"), stderr)
}

// TestLogoutTakesAnArgumentAndProfileThatNameOneProfile checks that the
// profile argument and --profile of auth logout may differ in case when they
// name the same profile, and are refused when they name two.
func TestLogoutTakesAnArgumentAndProfileThatNameOneProfile(t *testing.T) {
	testutil.WriteProfiles(t, testutil.UseNewHome(t), "http://127.0.0.1:1", rawTestID, "staging", "Prod")

	stdout, stderr, code := runRoot(t, "auth", "logout", "Staging", "--profile", "staging", "--output", "plain")

	assert.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, "Logged out from profile 'staging'. Profile 'Prod' is now the default\n", stdout)

	testutil.WriteProfiles(t, testutil.UseNewHome(t), "http://127.0.0.1:1", rawTestID, "Prod", "PROD")
	_, stderr, code = runRoot(t, "auth", "logout", "Prod", "--profile", "PROD")

	assert.Equal(t, 4, code)
	assert.Equal(t, "Error: the profile argument \"Prod\" and --profile \"PROD\" cannot be used together\n", stderr)
}
