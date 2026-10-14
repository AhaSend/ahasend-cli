package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-go"

	"github.com/AhaSend/ahasend-cli/internal/client"
	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusCommand_DefaultValues(t *testing.T) {
	cmd := NewStatusCommand()

	// Test default values
	all, _ := cmd.Flags().GetBool("all")
	assert.False(t, all, "All flag should default to false")

	profile, _ := cmd.Flags().GetString("profile")
	assert.Empty(t, profile, "Profile should default to empty string")
}

// TestAuthStatus_ParentAccountIDRendering verifies that the printer handlers
// surface the parent account ID when the authenticated account is itself a
// sub-account (non-nil ParentAccountID), and omit it otherwise. It does not
// alter the AuthStatus construction contract.
func TestAuthStatus_ParentAccountIDRendering(t *testing.T) {
	parentID := uuid.New()

	subAccountStatus := func() *printer.AuthStatus {
		return &printer.AuthStatus{
			Profile:   "default",
			AccountID: testAccountID,
			Valid:     ahasend.Bool(true),
			Account: &responses.Account{
				ID:              uuid.New(),
				Name:            "Sub Account",
				ParentAccountID: &parentID,
			},
		}
	}

	rootAccountStatus := func() *printer.AuthStatus {
		return &printer.AuthStatus{
			Profile:   "default",
			AccountID: testAccountID,
			Valid:     ahasend.Bool(true),
			Account: &responses.Account{
				ID:   uuid.New(),
				Name: "Root Account",
			},
		}
	}

	for _, format := range []string{"table", "plain", "csv"} {
		t.Run(format+" with parent account", func(t *testing.T) {
			var buf bytes.Buffer
			handler := testutil.ResponseHandler(t, format, &buf, os.Stderr)
			require.NoError(t, handler.HandleAuthStatus(subAccountStatus()))
			assert.Contains(t, buf.String(), parentID.String())
		})

		t.Run(format+" without parent account", func(t *testing.T) {
			var buf bytes.Buffer
			handler := testutil.ResponseHandler(t, format, &buf, os.Stderr)
			require.NoError(t, handler.HandleAuthStatus(rootAccountStatus()))
			assert.NotContains(t, buf.String(), parentID.String())
		})
	}

	t.Run("csv header includes parent_account_id", func(t *testing.T) {
		var buf bytes.Buffer
		handler := testutil.ResponseHandler(t, "csv", &buf, os.Stderr)
		require.NoError(t, handler.HandleAuthStatus(subAccountStatus()))
		assert.Contains(t, buf.String(), "parent_account_id")
	})
}

// Benchmark tests
func BenchmarkStatusCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewStatusCommand()
	}
}

func TestStatusChecksTheProfileAgainstItsAPIURL(t *testing.T) {
	for name, args := range map[string][]string{"default profile": nil, "named profile": {"--profile", "production"}} {
		t.Run(name, func(t *testing.T) {
			api := newTestAPI(t, http.StatusOK)
			writeProfiles(t, api.URL)

			_, err := testutil.RunCommandCombined(t, NewStatusCommand(), "plain", args...)
			require.NoError(t, err)
			assert.Equal(t, []string{"/v2/accounts/" + testAccountID}, api.Paths())
		})
	}
}

func TestStatusAllListsProfilesInOrder(t *testing.T) {
	api := newTestAPI(t, http.StatusOK)
	writeProfiles(t, api.URL)

	stdout, _, err := testutil.RunCommand(t, NewStatusCommand(), "plain", "", "--all")

	require.NoError(t, err)
	assert.Equal(t, "Found 2 profiles: default, production\n", stdout)
}

func TestStatusAllCSVHasARowPerProfile(t *testing.T) {
	api := newTestAPI(t, http.StatusOK)
	writeProfiles(t, api.URL)

	stdout, _, err := testutil.RunCommand(t, NewStatusCommand(), "csv", "", "--all")

	require.NoError(t, err)
	assert.Equal(t, "profile\ndefault\nproduction\n", stdout)
}

func TestStatusAllJSONListsTheProfiles(t *testing.T) {
	api := newTestAPI(t, http.StatusOK)
	writeProfiles(t, api.URL)

	stdout, _, err := testutil.RunCommand(t, NewStatusCommand(), "json", "", "--all")

	require.NoError(t, err)
	assert.JSONEq(t, `{"profiles":["default","production"]}`, stdout)
}

func TestStatusOfAMissingProfileIsAConfigError(t *testing.T) {
	api := newTestAPI(t, http.StatusOK)
	writeProfiles(t, api.URL)

	_, err := testutil.RunCommandCombined(t, NewStatusCommand(), "plain", "--profile", "staging")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "profile 'staging' not found")
	assert.Equal(t, 3, clierrors.GetExitCode(err))
	assert.Empty(t, api.Paths())
}

func TestStatusOfAMissingProfileWithNoProfilesIsAConfigError(t *testing.T) {
	_, err := testutil.RunCommandCombined(t, NewStatusCommand(), "json", "--profile", "staging")

	require.Error(t, err)
	assert.Equal(t, 3, clierrors.GetExitCode(err))
}

func TestStatusJSONUsesSnakeCaseKeysAndNeverPrintsTheAPIKey(t *testing.T) {
	api := newTestAPI(t, http.StatusOK)
	writeProfiles(t, api.URL)

	stdout, _, err := testutil.RunCommand(t, NewStatusCommand(), "json", "")

	require.NoError(t, err)
	assert.NotContains(t, stdout, "aha-sk-default")
	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &status))
	assert.Equal(t, "default", status["profile"])
	assert.Equal(t, testAccountID, status["account_id"])
	assert.Equal(t, true, status["valid"])
	assert.Equal(t, "Acme", status["account"].(map[string]any)["name"])
	assert.Len(t, status, 4)
}

func TestStatusOfRejectedCredentialsExitsTwo(t *testing.T) {
	for _, format := range []string{"json", "plain"} {
		t.Run(format, func(t *testing.T) {
			api := newTestAPI(t, http.StatusUnauthorized)
			writeProfiles(t, api.URL)

			stdout, _, err := testutil.RunCommand(t, NewStatusCommand(), format, "")

			require.Error(t, err)
			assert.Equal(t, 2, clierrors.GetExitCode(err))
			assert.NotContains(t, stdout, "aha-sk-default")
			if format == "json" {
				var status map[string]any
				require.NoError(t, json.Unmarshal([]byte(stdout), &status))
				assert.Equal(t, false, status["valid"])
				assert.Contains(t, status["error"], "(HTTP 401)")
				assert.Nil(t, status["account"])
			} else {
				// The failure is the error line the root command prints
				assert.True(t, strings.HasSuffix(stdout, "Valid: No\n"), stdout)
				assert.Contains(t, err.Error(), "credentials of profile 'default' could not be verified: ")
				assert.Contains(t, err.Error(), "(HTTP 401)")
			}
		})
	}
}

func TestStatusWhenTheAPIIsUnreachableExitsSeven(t *testing.T) {
	t.Cleanup(client.ShortenRetryDelaysForTesting())
	writeConfig(t, fmt.Sprintf(`default_profile: default
profiles:
  default:
    name: default
    api_key: aha-sk-default
    api_url: %s
    account_id: %s
`, testutil.HangUpURL(t), testAccountID))

	for _, format := range []string{"json", "plain"} {
		t.Run(format, func(t *testing.T) {
			stdout, _, err := testutil.RunCommand(t, NewStatusCommand(), format, "")

			require.Error(t, err)
			assert.Equal(t, 7, clierrors.GetExitCode(err))
			if format == "json" {
				var status map[string]any
				require.NoError(t, json.Unmarshal([]byte(stdout), &status))
				assert.Contains(t, status, "valid")
				assert.Nil(t, status["valid"], "an unreachable API says nothing of the key")
				assert.Contains(t, status["error"], "/v2/accounts/")
			} else {
				assert.NotContains(t, stdout, "Error")
				assert.NotContains(t, stdout, "Valid:")
				assert.Contains(t, err.Error(), "credentials of profile 'default' could not be verified: ")
			}
		})
	}
}

// TestStatusReadsAConfigWithCachedAccountDetails checks that a config file
// written by an earlier version, which cached the account's name, still
// loads, and that status leaves the file as it is.
func TestStatusReadsAConfigWithCachedAccountDetails(t *testing.T) {
	api := newTestAPI(t, http.StatusOK)
	config := fmt.Sprintf(`default_profile: default
profiles:
  default:
    name: default
    api_key: aha-sk-default
    api_url: %s
    account_id: %s
    account_name: Acme
    account_updated: 2020-01-01T00:00:00Z
`, api.URL, testAccountID)
	path := testutil.WriteConfig(t, testutil.UseNewHome(t), config)

	stdout, _, err := testutil.RunCommand(t, NewStatusCommand(), "plain", "")

	require.NoError(t, err)
	assert.Contains(t, stdout, "Valid: Yes")
	saved, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, config, string(saved))
}

// TestStatusAllWithoutProfilesListsNone checks that --all prints an empty
// list in its own shape: a CSV header, and an empty JSON array.
func TestStatusAllWithoutProfilesListsNone(t *testing.T) {
	tests := map[string]string{
		"json":  "{\n  \"profiles\": []\n}\n",
		"csv":   "profile\n",
		"plain": "No authentication profiles found\n",
	}
	for format, want := range tests {
		t.Run(format, func(t *testing.T) {
			testutil.UseNewHome(t)

			stdout, _, err := testutil.RunCommand(t, NewStatusCommand(), format, "", "--all")

			require.NoError(t, err)
			assert.Equal(t, want, stdout)
		})
	}
}

func TestStatusOfAKeyOfAnotherAccountIsInvalid(t *testing.T) {
	api := newTestAPIForKeyOf(t, otherAccountID, http.StatusOK)
	writeProfiles(t, api.URL)

	stdout, _, err := testutil.RunCommand(t, NewStatusCommand(), "json", "")

	require.Error(t, err)
	assert.Equal(t, 2, clierrors.GetExitCode(err))
	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &status))
	assert.Equal(t, false, status["valid"])
	assert.Contains(t, status["error"], "the API key is not valid for account "+testAccountID+"; check both")
}

func TestStatusOfAKeyThatMayNotReadTheAccountIsValid(t *testing.T) {
	api := newTestAPI(t, http.StatusForbidden)
	writeProfiles(t, api.URL)

	stdout, _, err := testutil.RunCommand(t, NewStatusCommand(), "json", "")

	require.NoError(t, err)
	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &status))
	assert.Equal(t, true, status["valid"])
	assert.Contains(t, status["note"], "the API key cannot read the account details: ")
	assert.Nil(t, status["account"])
}

func TestStatusOfAKeyRefusedFromThisIPIsInvalid(t *testing.T) {
	api := newRefusingTestAPI(t, http.StatusForbidden, "source IP 192.0.2.1 is not allowed for this API key")
	writeProfiles(t, api.URL)

	stdout, _, err := testutil.RunCommand(t, NewStatusCommand(), "json", "")

	require.Error(t, err)
	assert.Equal(t, 6, clierrors.GetExitCode(err))
	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &status))
	assert.Equal(t, false, status["valid"])
	assert.Contains(t, status["error"], "source IP 192.0.2.1 is not allowed for this API key")
}

func TestStatusOfAKeyOfAPausedAccountGivesTheAPIReason(t *testing.T) {
	api := newRefusingTestAPI(t, http.StatusUnauthorized, "account is paused")
	writeProfiles(t, api.URL)

	stdout, _, err := testutil.RunCommand(t, NewStatusCommand(), "json", "")

	require.Error(t, err)
	assert.Equal(t, 2, clierrors.GetExitCode(err))
	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &status))
	assert.Equal(t, false, status["valid"])
	assert.Contains(t, status["error"], "account is paused")
	assert.NotContains(t, status["error"], "check both")
}
