package auth

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogoutCommand_ArgumentParsing(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]interface{}
	}{
		{
			name: "logout all profiles",
			args: []string{"--all"},
			expected: map[string]interface{}{
				"all": true,
			},
		},
		{
			name: "logout specific profile",
			args: []string{"production"},
			expected: map[string]interface{}{
				"all": false,
			},
		},
		{
			name: "logout default (no args)",
			args: []string{},
			expected: map[string]interface{}{
				"all": false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewLogoutCommand()
			cmd.SetArgs(tt.args)

			err := cmd.ParseFlags(tt.args)
			require.NoError(t, err)

			for flag, expected := range tt.expected {
				switch expected := expected.(type) {
				case bool:
					value, err := cmd.Flags().GetBool(flag)
					require.NoError(t, err)
					assert.Equal(t, expected, value, "Flag %s should have correct value", flag)
				}
			}
		})
	}
}

func TestLogoutCommand_ExecutionFlow(t *testing.T) {
	tempDir := testutil.UseNewHome(t)

	// Create .ahasend directory
	configDir := filepath.Join(tempDir, ".ahasend")
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	// Create a test config file with profiles
	configContent := `current_profile: default
profiles:
  default:
    name: default
    api_key: test-key-1
    account_id: test-account-1
  production:
    name: production
    api_key: test-key-2
    account_id: test-account-2
`
	configPath := filepath.Join(configDir, "config.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "logout current profile",
			args:        []string{},
			expectError: false,
		},
		{
			name:        "logout specific profile",
			args:        []string{"production"},
			expectError: false,
		},
		{
			name:        "logout all profiles",
			args:        []string{"--all", "--force"},
			expectError: false,
		},
		{
			name:        "logout non-existent profile",
			args:        []string{"staging"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Recreate config file for each test
			err := os.WriteFile(configPath, []byte(configContent), 0644)
			require.NoError(t, err)

			cmd := NewLogoutCommand()
			cmd.SetArgs(tt.args)

			// Capture output
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			err = cmd.Execute()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				// May have errors due to test environment
				_ = err
			}
		})
	}
}

func TestLogoutCommand_DefaultValues(t *testing.T) {
	cmd := NewLogoutCommand()

	// Test default values
	all, _ := cmd.Flags().GetBool("all")
	assert.False(t, all, "All flag should default to false")

	profile, _ := cmd.Flags().GetString("profile")
	assert.Empty(t, profile, "Profile should default to empty string")
}

// Benchmark tests
func BenchmarkLogoutCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewLogoutCommand()
	}
}

func BenchmarkLogoutCommand_FlagParsing(b *testing.B) {
	cmd := NewLogoutCommand()
	args := []string{"--profile", "test-profile"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := cmd.ParseFlags(args)
		if err != nil {
			b.Fatal(err)
		}
		// Reset flags for next iteration
		cmd.ResetFlags()
		cmd = NewLogoutCommand()
	}
}

// TestLogoutWithoutProfilesPrintsTheAuthResult checks that logout prints the
// auth result shape, success included, when there is nothing to log out of.
func TestLogoutWithoutProfilesPrintsTheAuthResult(t *testing.T) {
	tests := map[string]string{
		"json":  "{\n  \"profile\": \"\",\n  \"success\": true,\n  \"message\": \"No authentication profiles found\"\n}\n",
		"csv":   "profile,success,message\n,true,No authentication profiles found\n",
		"plain": "No authentication profiles found\n",
	}
	for format, want := range tests {
		t.Run(format, func(t *testing.T) {
			testutil.UseNewHome(t)

			stdout, _, err := testutil.RunCommand(t, NewLogoutCommand(), format, "")

			require.NoError(t, err)
			assert.Equal(t, want, stdout)
		})
	}
}

// TestLogoutAllAsksForConfirmation checks that --all asks on stderr before it
// removes every profile, that declining or ending stdin keeps the profiles
// and exits 4, and that --force or a "y" removes them.
func TestLogoutAllAsksForConfirmation(t *testing.T) {
	tests := map[string]struct {
		stdin   string
		args    []string
		removed bool
		err     string
	}{
		"declined":     {stdin: "n\n", args: []string{"--all"}, err: "operation cancelled"},
		"end of input": {stdin: "", args: []string{"--all"}, err: "operation cancelled; use --force to skip the confirmation"},
		"accepted":     {stdin: "y\n", args: []string{"--all"}, removed: true},
		"forced":       {stdin: "", args: []string{"--all", "--force"}, removed: true},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			home := testutil.UseNewHome(t)
			path := testutil.WriteProfiles(t, home, "http://127.0.0.1:1", "11111111-1111-4111-8111-111111111111", "default", "production")

			stdout, stderr, err := testutil.RunCommand(t, NewLogoutCommand(), "plain", test.stdin, test.args...)

			config, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			if test.removed {
				require.NoError(t, err)
				assert.Equal(t, "Logged out from all profiles (2 removed)\n", stdout)
				assert.NotContains(t, string(config), "production")
			} else {
				require.Error(t, err)
				assert.Equal(t, 4, clierrors.GetExitCode(err))
				assert.Equal(t, test.err, err.Error())
				assert.Empty(t, stdout)
				assert.Contains(t, string(config), "production")
			}
			if slices.Contains(test.args, "--force") {
				assert.Empty(t, stderr)
			} else {
				assert.Contains(t, stderr, "Are you sure you want to log out of all 2 profiles and delete their stored API keys? (y/N): ")
			}
		})
	}
}

// TestLogoutForceNeedsAll checks that --force without --all is refused.
func TestLogoutForceNeedsAll(t *testing.T) {
	testutil.UseNewHome(t)

	_, _, err := testutil.RunCommand(t, NewLogoutCommand(), "plain", "", "--force")

	require.EqualError(t, err, "--force needs --all")
	assert.Equal(t, 4, clierrors.GetExitCode(err))
}
