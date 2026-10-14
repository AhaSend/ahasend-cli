package auth

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/config"
	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeV020Profiles writes a configuration file as v0.2.0 saved it after
// `auth login --profile staging` and `auth login --profile Prod`: the profile
// names keep the case the user typed, and v0.2.0 matched them ignoring case.
func writeV020Profiles(t *testing.T, apiURL string) {
	t.Helper()
	writeConfig(t, fmt.Sprintf(`default_profile: staging
preferences:
  batch_concurrency: 5
  color_output: true
  log_level: info
  output_format: table
  webhook_timeout: 30s
profiles:
  Prod:
    account_id: %[2]s
    api_key: aha-sk-prod
    api_url: %[1]s
    name: AhaSend Prod
  staging:
    account_id: %[2]s
    api_key: aha-sk-staging
    api_url: %[1]s
    name: AhaSend staging
`, apiURL, testAccountID))
}

// loadedConfig loads the configuration file the commands wrote.
func loadedConfig(t *testing.T) *config.Config {
	t.Helper()
	manager, err := config.NewManager()
	require.NoError(t, err)
	require.NoError(t, manager.Load())
	return manager.GetConfig()
}

func TestStatusFindsAProfileNamedInAnotherCase(t *testing.T) {
	api := newTestAPI(t, http.StatusOK)
	writeV020Profiles(t, api.URL)

	stdout, _, err := testutil.RunCommand(t, NewStatusCommand(), "json", "", "--profile", "prod")

	require.NoError(t, err)
	assert.Contains(t, stdout, `"profile": "Prod"`)
}

func TestSwitchFindsAProfileNamedInAnotherCase(t *testing.T) {
	api := newTestAPI(t, http.StatusOK)
	writeV020Profiles(t, api.URL)

	stdout, _, err := testutil.RunCommand(t, NewSwitchCommand(), "plain", "", "prod")

	require.NoError(t, err)
	assert.Equal(t, "Switched to profile 'Prod'\n", stdout)
	assert.Equal(t, "Prod", loadedConfig(t).DefaultProfile)
}

func TestLogoutFindsAProfileNamedInAnotherCase(t *testing.T) {
	api := newTestAPI(t, http.StatusOK)
	writeV020Profiles(t, api.URL)

	stdout, _, err := testutil.RunCommand(t, NewLogoutCommand(), "plain", "", "prod")

	require.NoError(t, err)
	assert.Equal(t, "Logged out from profile 'Prod'\n", stdout)
	assert.NotContains(t, loadedConfig(t).Profiles, "Prod")
}

// TestLoginAgainUpdatesTheProfileNamedInAnotherCase checks that logging in
// again with --profile prod replaces the key of profile Prod rather than
// adding a second profile that differs from it only in case.
func TestLoginAgainUpdatesTheProfileNamedInAnotherCase(t *testing.T) {
	api := newTestAPI(t, http.StatusOK)
	writeV020Profiles(t, api.URL)

	_, _, err := testutil.RunCommand(t, NewLoginCommand(), "plain", "",
		"--profile", "prod", "--api-key", "aha-sk-new", "--account-id", testAccountID, "--api-url", api.URL)

	require.NoError(t, err)
	profiles := loadedConfig(t).Profiles
	assert.Len(t, profiles, 2)
	assert.Equal(t, "aha-sk-new", profiles["Prod"].APIKey)
}

func TestStatusRefusesANameSeveralProfilesMatchIgnoringCase(t *testing.T) {
	api := newTestAPI(t, http.StatusOK)
	testutil.WriteProfiles(t, testutil.UseNewHome(t), api.URL, testAccountID, "Prod", "PROD")

	_, err := testutil.RunCommandCombined(t, NewStatusCommand(), "plain", "--profile", "prod")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "profile 'prod' not found; the profiles 'PROD', 'Prod' differ from it only in case")
	assert.Equal(t, 3, clierrors.GetExitCode(err))
	assert.Empty(t, api.Paths())
}

// writeV020DefaultInAnotherCase writes a configuration file as v0.2.0 could
// save it: the default names profile prod as Prod, which v0.2.0 matched
// ignoring case.
func writeV020DefaultInAnotherCase(t *testing.T, apiURL string) {
	t.Helper()
	writeConfig(t, fmt.Sprintf(`default_profile: Prod
profiles:
  prod:
    account_id: %[2]s
    api_key: aha-sk-prod
    api_url: %[1]s
    name: AhaSend prod
  staging:
    account_id: %[2]s
    api_key: aha-sk-staging
    api_url: %[1]s
    name: AhaSend staging
`, apiURL, testAccountID))
}

// TestLogoutOfADefaultNamedInAnotherCaseChoosesTheNextDefault checks that
// logging out of the default profile, which the file names in another case,
// leaves no default that names a profile no more: the one profile left
// becomes the default.
func TestLogoutOfADefaultNamedInAnotherCaseChoosesTheNextDefault(t *testing.T) {
	api := newTestAPI(t, http.StatusOK)
	writeV020DefaultInAnotherCase(t, api.URL)

	stdout, _, err := testutil.RunCommand(t, NewLogoutCommand(), "plain", "")

	require.NoError(t, err)
	assert.Equal(t, "Logged out from profile 'prod'. Profile 'staging' is now the default\n", stdout)
	assert.Equal(t, "staging", loadedConfig(t).DefaultProfile)
}

// TestSwitchToADefaultNamedInAnotherCaseIsAlreadyInUse checks that switching
// to the default profile, which the file names in another case, says it is
// already in use and changes nothing.
func TestSwitchToADefaultNamedInAnotherCaseIsAlreadyInUse(t *testing.T) {
	api := newTestAPI(t, http.StatusOK)
	writeV020DefaultInAnotherCase(t, api.URL)

	stdout, _, err := testutil.RunCommand(t, NewSwitchCommand(), "plain", "", "prod")

	require.NoError(t, err)
	assert.Equal(t, "Already using profile 'prod'\n", stdout)
	assert.Empty(t, api.Paths())
}

// TestStatusShowsADefaultNamedInAnotherCase checks that status, with no
// profile given, shows the default profile the file names in another case,
// by its own name.
func TestStatusShowsADefaultNamedInAnotherCase(t *testing.T) {
	api := newTestAPI(t, http.StatusOK)
	writeV020DefaultInAnotherCase(t, api.URL)

	stdout, _, err := testutil.RunCommand(t, NewStatusCommand(), "json", "")

	require.NoError(t, err)
	assert.Contains(t, stdout, `"profile": "prod"`)
}

// TestLoginRefusesANameSeveralProfilesMatchIgnoringCase checks that login
// with a name that two profiles match ignoring case fails before asking for
// anything, as the other commands do, and adds no third profile.
func TestLoginRefusesANameSeveralProfilesMatchIgnoringCase(t *testing.T) {
	api := newTestAPI(t, http.StatusOK)
	testutil.WriteProfiles(t, testutil.UseNewHome(t), api.URL, testAccountID, "Prod", "PROD")

	_, stderr, err := testutil.RunCommand(t, NewLoginCommand(), "plain", "", "--profile", "prod", "--api-url", api.URL)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "profile 'prod' not found; the profiles 'PROD', 'Prod' differ from it only in case")
	assert.Equal(t, 3, clierrors.GetExitCode(err))
	assert.NotContains(t, stderr, "API key")
	assert.Empty(t, api.Paths())
	assert.Len(t, loadedConfig(t).Profiles, 2)
}
