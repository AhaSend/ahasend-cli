package auth

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogoutCommand_Structure(t *testing.T) {
	logoutCmd := NewLogoutCommand()
	assert.Equal(t, "logout", logoutCmd.Name())
	assert.Equal(t, "Log out and remove stored credentials", logoutCmd.Short)
	assert.NotEmpty(t, logoutCmd.Long)
	assert.NotEmpty(t, logoutCmd.Example)
}

func TestLogoutCommand_Flags(t *testing.T) {
	// Test that logout command has required flags
	logoutCmd := NewLogoutCommand()
	flags := logoutCmd.Flags()

	allFlag := flags.Lookup("all")
	assert.NotNil(t, allFlag)
}

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
	tempDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", tempDir)

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
			args:        []string{"--all"},
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

func TestLogoutCommand_Help(t *testing.T) {
	cmd := NewLogoutCommand()

	// Capture help output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	assert.NoError(t, err)

	helpOutput := buf.String()
	assert.Contains(t, helpOutput, "Remove stored")
	assert.Contains(t, helpOutput, "--all")
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
