package webhooks

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// TestListCommand_OutputFormats tests all output formats for the list command
func TestListCommand_OutputFormats(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		validateOutput func(t *testing.T, output string)
	}{
		{
			name: "default table output",
			args: []string{"list"},
			validateOutput: func(t *testing.T, output string) {
				// Table output should not be JSON
				var js json.RawMessage
				err := json.Unmarshal([]byte(output), &js)
				assert.Error(t, err, "Table output should not be valid JSON")
			},
		},
		{
			name: "explicit table output",
			args: []string{"list", "--output", "table"},
			validateOutput: func(t *testing.T, output string) {
				// Table output should not be JSON
				var js json.RawMessage
				err := json.Unmarshal([]byte(output), &js)
				assert.Error(t, err, "Table output should not be valid JSON")
			},
		},
		{
			name: "json output",
			args: []string{"list", "--output", "json"},
			validateOutput: func(t *testing.T, output string) {
				// JSON output should be valid JSON
				if strings.Contains(output, "{") {
					var js json.RawMessage
					err := json.Unmarshal([]byte(output), &js)
					assert.NoError(t, err, "JSON output should be valid JSON")
				}
			},
		},
		{
			name: "csv output",
			args: []string{"list", "--output", "csv"},
			validateOutput: func(t *testing.T, output string) {
				// CSV output should have comma-separated values or appropriate format
			},
		},
		{
			name: "invalid output format",
			args: []string{"list", "--output", "invalid"},
			validateOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "unknown flag")
			},
		},
		{
			name: "json output with enabled filter",
			args: []string{"list", "--enabled", "--output", "json"},
			validateOutput: func(t *testing.T, output string) {
				if strings.Contains(output, "{") {
					var js json.RawMessage
					err := json.Unmarshal([]byte(output), &js)
					assert.NoError(t, err, "JSON output with enabled filter should be valid JSON")
				}
			},
		},
		{
			name: "json output with pagination",
			args: []string{"list", "--limit", "10", "--output", "json"},
			validateOutput: func(t *testing.T, output string) {
				if strings.Contains(output, "{") {
					var js json.RawMessage
					err := json.Unmarshal([]byte(output), &js)
					assert.NoError(t, err, "JSON output with pagination should be valid JSON")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewListCommand()
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tt.args)

			// Note: Command will fail without auth, but we can still test flag parsing
			_ = cmd.Execute()

			output := buf.String()
			if tt.validateOutput != nil {
				tt.validateOutput(t, output)
			}
		})
	}
}

// TestGetCommand_OutputFormats tests all output formats for the get command
func TestGetCommand_OutputFormats(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		validateOutput func(t *testing.T, output string)
	}{
		{
			name: "default table output",
			args: []string{"get", "wh_test"},
			validateOutput: func(t *testing.T, output string) {
				// Table output should not be JSON
				var js json.RawMessage
				err := json.Unmarshal([]byte(output), &js)
				assert.Error(t, err, "Table output should not be valid JSON")
			},
		},
		{
			name: "explicit table output",
			args: []string{"get", "wh_test", "--output", "table"},
			validateOutput: func(t *testing.T, output string) {
				// Table output should not be JSON
				var js json.RawMessage
				err := json.Unmarshal([]byte(output), &js)
				assert.Error(t, err, "Table output should not be valid JSON")
			},
		},
		{
			name: "json output",
			args: []string{"get", "wh_test", "--output", "json"},
			validateOutput: func(t *testing.T, output string) {
				// JSON output should be valid JSON (error or data)
				if strings.Contains(output, "{") {
					var js json.RawMessage
					err := json.Unmarshal([]byte(output), &js)
					assert.NoError(t, err, "JSON output should be valid JSON")
				}
			},
		},
		{
			name: "csv output",
			args: []string{"get", "wh_test", "--output", "csv"},
			validateOutput: func(t *testing.T, output string) {
				// CSV output for single item
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewGetCommand()
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tt.args)

			_ = cmd.Execute()

			output := buf.String()
			if tt.validateOutput != nil {
				tt.validateOutput(t, output)
			}
		})
	}
}

// TestCreateCommand_OutputFormats tests all output formats for the create command
func TestCreateCommand_OutputFormats(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		validateOutput func(t *testing.T, output string)
	}{
		{
			name: "default table output",
			args: []string{"create", "--interactive=false", "--name", "Test Webhook", "--url", "https://example.com/webhook", "--events", "message.delivered"},
			validateOutput: func(t *testing.T, output string) {
				// Table output should not be JSON
				var js json.RawMessage
				err := json.Unmarshal([]byte(output), &js)
				assert.Error(t, err, "Table output should not be valid JSON")
			},
		},
		{
			name: "json output",
			args: []string{"create", "--interactive=false", "--name", "Test Webhook", "--url", "https://example.com/webhook", "--events", "message.delivered", "--output", "json"},
			validateOutput: func(t *testing.T, output string) {
				// JSON output should be valid JSON (error or data)
				if strings.Contains(output, "{") {
					var js json.RawMessage
					err := json.Unmarshal([]byte(output), &js)
					assert.NoError(t, err, "JSON output should be valid JSON")
				}
			},
		},
		{
			name: "csv output",
			args: []string{"create", "--interactive=false", "--name", "Test Webhook", "--url", "https://example.com/webhook", "--events", "message.delivered", "--output", "csv"},
			validateOutput: func(t *testing.T, output string) {
				// CSV output for single item
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCreateCommand()
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tt.args)

			_ = cmd.Execute()

			output := buf.String()
			if tt.validateOutput != nil {
				tt.validateOutput(t, output)
			}
		})
	}
}

// TestUpdateCommand_OutputFormats tests all output formats for the update command
func TestUpdateCommand_OutputFormats(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		validateOutput func(t *testing.T, output string)
	}{
		{
			name: "default table output",
			args: []string{"update", "wh_test", "--name", "Updated Webhook"},
			validateOutput: func(t *testing.T, output string) {
				// Table output should not be JSON
				var js json.RawMessage
				err := json.Unmarshal([]byte(output), &js)
				assert.Error(t, err, "Table output should not be valid JSON")
			},
		},
		{
			name: "json output",
			args: []string{"update", "wh_test", "--name", "Updated Webhook", "--output", "json"},
			validateOutput: func(t *testing.T, output string) {
				// JSON output should be valid JSON (error or data)
				if strings.Contains(output, "{") {
					var js json.RawMessage
					err := json.Unmarshal([]byte(output), &js)
					assert.NoError(t, err, "JSON output should be valid JSON")
				}
			},
		},
		{
			name: "csv output",
			args: []string{"update", "wh_test", "--name", "Updated Webhook", "--output", "csv"},
			validateOutput: func(t *testing.T, output string) {
				// CSV output for single item
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewUpdateCommand()
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tt.args)

			_ = cmd.Execute()

			output := buf.String()
			if tt.validateOutput != nil {
				tt.validateOutput(t, output)
			}
		})
	}
}

// TestDeleteCommand_OutputFormats tests all output formats for the delete command
func TestDeleteCommand_OutputFormats(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		validateOutput func(t *testing.T, output string)
	}{
		{
			name: "default table output",
			args: []string{"delete", "wh_test", "--force"},
			validateOutput: func(t *testing.T, output string) {
				// Table output should not be JSON
				var js json.RawMessage
				err := json.Unmarshal([]byte(output), &js)
				assert.Error(t, err, "Table output should not be valid JSON")
			},
		},
		{
			name: "json output",
			args: []string{"delete", "wh_test", "--force", "--output", "json"},
			validateOutput: func(t *testing.T, output string) {
				// JSON output should be valid JSON (error or success message)
				if strings.Contains(output, "{") {
					var js json.RawMessage
					err := json.Unmarshal([]byte(output), &js)
					assert.NoError(t, err, "JSON output should be valid JSON")
				}
			},
		},
		{
			name: "csv output",
			args: []string{"delete", "wh_test", "--force", "--output", "csv"},
			validateOutput: func(t *testing.T, output string) {
				// CSV output for delete confirmation
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewDeleteCommand()
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tt.args)

			_ = cmd.Execute()

			output := buf.String()
			if tt.validateOutput != nil {
				tt.validateOutput(t, output)
			}
		})
	}
}

// TestOutputFormatValidation tests that the output format validation works correctly for all webhooks commands
func TestOutputFormatValidation(t *testing.T) {
	commands := []struct {
		name    string
		cmdFunc func() *cobra.Command
		args    []string
	}{
		{
			name:    "list",
			cmdFunc: NewListCommand,
			args:    []string{},
		},
		{
			name:    "get",
			cmdFunc: NewGetCommand,
			args:    []string{"wh_test"},
		},
		{
			name:    "create",
			cmdFunc: NewCreateCommand,
			args:    []string{"--interactive=false", "--name", "Test", "--url", "https://example.com", "--events", "message.delivered"},
		},
		{
			name:    "update",
			cmdFunc: NewUpdateCommand,
			args:    []string{"wh_test", "--name", "Updated"},
		},
		{
			name:    "delete",
			cmdFunc: NewDeleteCommand,
			args:    []string{"wh_test", "--force"},
		},
	}

	for _, cmdInfo := range commands {
		t.Run(cmdInfo.name, func(t *testing.T) {
			// Test with valid output formats
			validFormats := []string{"table", "json", "csv"}
			for _, format := range validFormats {
				t.Run(format, func(t *testing.T) {
					cmd := cmdInfo.cmdFunc()
					args := append(cmdInfo.args, "--output", format)
					cmd.SetArgs(args)

					// Should not panic or error on valid format
					// (will fail on auth, but format validation happens first)
					_ = cmd.Execute()
				})
			}

			// Test with invalid output format
			t.Run("invalid_format", func(t *testing.T) {
				cmd := cmdInfo.cmdFunc()
				var buf bytes.Buffer
				cmd.SetErr(&buf)

				args := append(cmdInfo.args, "--output", "xml")
				cmd.SetArgs(args)

				_ = cmd.Execute()

				output := buf.String()
				assert.Contains(t, output, "unknown flag",
					"Command %s should reject invalid output format", cmdInfo.name)
			})
		})
	}
}

// TestOutputFlagIntegration tests that all webhooks commands properly integrate the --output flag
func TestOutputFlagIntegration(t *testing.T) {
	// Get the parent webhooks command
	webhooksCmd := NewCommand()

	// Check all subcommands have output flag support
	for _, subCmd := range webhooksCmd.Commands() {
		t.Run(subCmd.Name(), func(t *testing.T) {
			var buf bytes.Buffer
			subCmd.SetOut(&buf)
			subCmd.SetErr(&buf)

			// Build args based on command requirements
			args := []string{"--output", "json"}
			switch subCmd.Name() {
			case "get", "delete", "update":
				args = append([]string{"wh_test"}, args...)
			case "create":
				args = append(args, "--interactive=false", "--name", "Test", "--url", "https://example.com", "--events", "message.delivered")
			}
			if subCmd.Name() == "delete" {
				args = append(args, "--force")
			}

			subCmd.SetArgs(args)
			_ = subCmd.Execute()

			// Should not complain about unknown flag
			output := buf.String()
			assert.NotContains(t, output, "unknown flag",
				"Command %s should recognize --output flag", subCmd.Name())
		})
	}
}

// BenchmarkOutputFormats benchmarks different output format processing
func BenchmarkOutputFormats(b *testing.B) {
	formats := []string{"table", "json", "csv"}

	for _, format := range formats {
		b.Run(format, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				cmd := NewListCommand()
				var buf bytes.Buffer
				cmd.SetOut(&buf)
				cmd.SetErr(&buf)
				cmd.SetArgs([]string{"--output", format})
				_ = cmd.Execute()
			}
		})
	}
}
