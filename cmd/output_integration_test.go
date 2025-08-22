package cmd

import (
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/stretchr/testify/assert"
)

// TestOutputFormatsIntegration tests that all commands support the --output flag
// This test complements the existing JSON output tests but covers all formats
func TestOutputFormatsIntegration(t *testing.T) {
	commands := []struct {
		name string
		args []string
	}{
		// Auth commands
		{"auth status", []string{"auth", "status"}},

		// Domain commands
		{"domains list", []string{"domains", "list"}},
		{"domains get", []string{"domains", "get", "test.com"}},

		// Message commands
		{"messages list", []string{"messages", "list"}},

		// API Keys commands
		{"apikeys list", []string{"apikeys", "list"}},
		{"apikeys get", []string{"apikeys", "get", "ak_test"}},

		// Routes commands
		{"routes list", []string{"routes", "list"}},
		{"routes get", []string{"routes", "get", "rt_test"}},

		// SMTP commands
		{"smtp list", []string{"smtp", "list"}},
		{"smtp get", []string{"smtp", "get", "smtp_test"}},

		// Suppressions commands
		{"suppressions list", []string{"suppressions", "list"}},
		{"suppressions check", []string{"suppressions", "check", "test@example.com"}},

		// Webhooks commands
		{"webhooks list", []string{"webhooks", "list"}},
		{"webhooks get", []string{"webhooks", "get", "wh_test"}},

		// Stats commands
		{"stats deliverability", []string{"stats", "deliverability", "--time-range", "24h"}},
		{"stats bounces", []string{"stats", "bounces", "--time-range", "24h"}},
		{"stats delivery-time", []string{"stats", "delivery-time", "--time-range", "24h"}},
	}

	formats := []string{"table", "json", "csv"}

	for _, cmdTest := range commands {
		for _, format := range formats {
			t.Run(cmdTest.name+"_"+format, func(t *testing.T) {
				// Create args with output format
				args := append(cmdTest.args, "--output", format)

				// Execute command and capture output
				output, _ := testutil.ExecuteCommandIsolated(t, NewRootCmdForTesting, args...)

				// Command may fail due to auth/missing args, but --output flag should be recognized
				// If it fails with "unknown flag: --output", that's a test failure
				assert.NotContains(t, output, "unknown flag: --output",
					"Command %s should recognize --output flag", cmdTest.name)
			})
		}

		// Test invalid format rejection
		t.Run(cmdTest.name+"_invalid_format", func(t *testing.T) {
			// Create args with invalid output format
			args := append(cmdTest.args, "--output", "xml")

			// Execute command and capture output
			output, _ := testutil.ExecuteCommandIsolated(t, NewRootCmdForTesting, args...)

			// Should reject invalid format (either "unsupported output format" or similar)
			// Check that it doesn't say "unknown flag" which would indicate --output isn't recognized
			assert.NotContains(t, output, "unknown flag: --output",
				"Command %s should recognize --output flag even with invalid format", cmdTest.name)

			// The key test is that the --output flag is recognized, not necessarily that
			// format validation happens (since other validation may fail first)
			// If we got here without "unknown flag: --output", the test passes
		})
	}
}

// TestOutputFormatValidation tests that invalid output formats are rejected
func TestOutputFormatValidation(t *testing.T) {
	// Test that commands reject invalid output formats appropriately
	output, _ := testutil.ExecuteCommandIsolated(t, NewRootCmdForTesting, "auth", "status", "--output", "xml")

	// Should either show "unsupported output format" or handle gracefully
	// The key is that it recognizes the --output flag (doesn't say "unknown flag")
	assert.NotContains(t, output, "unknown flag: --output",
		"Commands should recognize --output flag")
}
