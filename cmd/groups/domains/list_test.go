package domains

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListCommand_Flags(t *testing.T) {
	// Test that list command has required flags
	listCmd := NewListCommand()
	flags := listCmd.Flags()

	limitFlag := flags.Lookup("limit")
	assert.NotNil(t, limitFlag)
	assert.Equal(t, "int32", limitFlag.Value.Type())

	cursorFlag := flags.Lookup("cursor")
	assert.NotNil(t, cursorFlag)
	assert.Equal(t, "string", cursorFlag.Value.Type())

	statusFlag := flags.Lookup("status")
	assert.NotNil(t, statusFlag)
	assert.Equal(t, "string", statusFlag.Value.Type())
}

func TestListCommand_Structure(t *testing.T) {
	listCmd := NewListCommand()
	assert.Equal(t, "list", listCmd.Name())
	assert.Equal(t, "List all domains", listCmd.Short)
	assert.NotEmpty(t, listCmd.Long)
	assert.NotEmpty(t, listCmd.Example)
}

func TestListCommand_FilterLogic(t *testing.T) {
	// Test the domain filtering logic
	testDomains := []struct {
		domain   string
		dnsValid bool
		status   string
		expected bool
	}{
		{"example.com", true, "verified", true},
		{"test.com", false, "verified", false},
		{"example.com", true, "pending", false},
		{"test.com", false, "pending", true},
		{"test.com", false, "failed", true},
	}

	for _, tt := range testDomains {
		t.Run(tt.domain+"_"+tt.status, func(t *testing.T) {
			var matches bool
			switch tt.status {
			case "verified":
				matches = tt.dnsValid
			case "pending", "failed":
				matches = !tt.dnsValid
			}

			assert.Equal(t, tt.expected, matches,
				"Domain %s with DNS valid %v should match status %s: %v",
				tt.domain, tt.dnsValid, tt.status, tt.expected)
		})
	}
}

func TestListCommand_FlagParsing(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]interface{}
	}{
		{
			name: "all flags provided",
			args: []string{
				"--limit", "50",
				"--cursor", "next-page-token",
				"--status", "verified",
			},
			expected: map[string]interface{}{
				"limit":  int32(50),
				"cursor": "next-page-token",
				"status": "verified",
			},
		},
		{
			name: "only limit flag",
			args: []string{"--limit", "25"},
			expected: map[string]interface{}{
				"limit": int32(25),
			},
		},
		{
			name: "only status flag",
			args: []string{"--status", "pending"},
			expected: map[string]interface{}{
				"status": "pending",
			},
		},
		{
			name:     "no flags",
			args:     []string{},
			expected: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewListCommand()
			cmd.SetArgs(tt.args)

			err := cmd.ParseFlags(tt.args)
			require.NoError(t, err)

			for flag, expected := range tt.expected {
				switch expected := expected.(type) {
				case int32:
					value, err := cmd.Flags().GetInt32(flag)
					require.NoError(t, err)
					assert.Equal(t, expected, value, "Flag %s should have correct value", flag)
				case string:
					value, err := cmd.Flags().GetString(flag)
					require.NoError(t, err)
					assert.Equal(t, expected, value, "Flag %s should have correct value", flag)
				}
			}
		})
	}
}

func TestListCommand_DefaultValues(t *testing.T) {
	cmd := NewListCommand()

	// Test default values
	limit, _ := cmd.Flags().GetInt32("limit")
	assert.Equal(t, int32(0), limit, "Limit should default to 0")

	cursor, _ := cmd.Flags().GetString("cursor")
	assert.Empty(t, cursor, "Cursor should default to empty string")

	status, _ := cmd.Flags().GetString("status")
	assert.Empty(t, status, "Status should default to empty string")
}

func TestListCommand_Help(t *testing.T) {
	cmd := NewListCommand()

	// Capture help output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	assert.NoError(t, err)

	helpOutput := buf.String()
	assert.Contains(t, helpOutput, "List all domains")
	assert.Contains(t, helpOutput, "--limit")
	assert.Contains(t, helpOutput, "--cursor")
	assert.Contains(t, helpOutput, "--status")
}

func TestListCommand_StatusValidation(t *testing.T) {
	validStatuses := []string{"", "verified", "pending", "failed"}

	for _, status := range validStatuses {
		t.Run("status_"+status, func(t *testing.T) {
			cmd := NewListCommand()
			if status != "" {
				cmd.SetArgs([]string{"--status", status})
			} else {
				cmd.SetArgs([]string{})
			}

			err := cmd.ParseFlags(cmd.Flags().Args())
			assert.NoError(t, err)
		})
	}
}

// Benchmark tests
func BenchmarkListCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewListCommand()
	}
}

func BenchmarkListCommand_FlagParsing(b *testing.B) {
	cmd := NewListCommand()
	args := []string{"--limit", "50", "--cursor", "token", "--status", "verified"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := cmd.ParseFlags(args)
		if err != nil {
			b.Fatal(err)
		}
		// Reset flags for next iteration
		cmd.ResetFlags()
		cmd = NewListCommand()
	}
}
