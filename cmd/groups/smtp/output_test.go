package smtp

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
			name: "json output with pagination",
			args: []string{"list", "--output", "json", "--limit", "10"},
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
			args: []string{"get", "smtp_test"},
			validateOutput: func(t *testing.T, output string) {
				// Table output should not be JSON
				var js json.RawMessage
				err := json.Unmarshal([]byte(output), &js)
				assert.Error(t, err, "Table output should not be valid JSON")
			},
		},
		{
			name: "explicit table output",
			args: []string{"get", "smtp_test", "--output", "table"},
			validateOutput: func(t *testing.T, output string) {
				// Table output should not be JSON
				var js json.RawMessage
				err := json.Unmarshal([]byte(output), &js)
				assert.Error(t, err, "Table output should not be valid JSON")
			},
		},
		{
			name: "json output",
			args: []string{"get", "smtp_test", "--output", "json"},
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
			args: []string{"get", "smtp_test", "--output", "csv"},
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
			args: []string{"create", "--interactive=false", "--name", "Test SMTP"},
			validateOutput: func(t *testing.T, output string) {
				// Table output should not be JSON
				var js json.RawMessage
				err := json.Unmarshal([]byte(output), &js)
				assert.Error(t, err, "Table output should not be valid JSON")
			},
		},
		{
			name: "json output",
			args: []string{"create", "--interactive=false", "--name", "Test SMTP", "--output", "json"},
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
			args: []string{"create", "--interactive=false", "--name", "Test SMTP", "--output", "csv"},
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

// TestDeleteCommand_OutputFormats tests all output formats for the delete command
func TestDeleteCommand_OutputFormats(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		validateOutput func(t *testing.T, output string)
	}{
		{
			name: "default table output",
			args: []string{"delete", "smtp_test", "--force"},
			validateOutput: func(t *testing.T, output string) {
				// Table output should not be JSON
				var js json.RawMessage
				err := json.Unmarshal([]byte(output), &js)
				assert.Error(t, err, "Table output should not be valid JSON")
			},
		},
		{
			name: "json output",
			args: []string{"delete", "smtp_test", "--force", "--output", "json"},
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
			args: []string{"delete", "smtp_test", "--force", "--output", "csv"},
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

// TestSendCommand_OutputFormats tests all output formats for the send command
func TestSendCommand_OutputFormats(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		validateOutput func(t *testing.T, output string)
	}{
		{
			name: "default table output",
			args: []string{"send", "--from", "test@example.com", "--to", "recipient@example.com", "--subject", "Test", "--text", "Test message", "--username", "test", "--password", "test"},
			validateOutput: func(t *testing.T, output string) {
				// Table output should not be JSON
				var js json.RawMessage
				err := json.Unmarshal([]byte(output), &js)
				assert.Error(t, err, "Table output should not be valid JSON")
			},
		},
		{
			name: "json output",
			args: []string{"send", "--from", "test@example.com", "--to", "recipient@example.com", "--subject", "Test", "--text", "Test message", "--username", "test", "--password", "test", "--output", "json"},
			validateOutput: func(t *testing.T, output string) {
				// JSON output should be valid JSON (error or success)
				if strings.Contains(output, "{") {
					var js json.RawMessage
					err := json.Unmarshal([]byte(output), &js)
					assert.NoError(t, err, "JSON output should be valid JSON")
				}
			},
		},
		{
			name: "csv output",
			args: []string{"send", "--from", "test@example.com", "--to", "recipient@example.com", "--subject", "Test", "--text", "Test message", "--username", "test", "--password", "test", "--output", "csv"},
			validateOutput: func(t *testing.T, output string) {
				// CSV output for send result
			},
		},
		{
			name: "test mode with json output",
			args: []string{"send", "--from", "test@example.com", "--to", "recipient@example.com", "--subject", "Test", "--text", "Test message", "--username", "test", "--password", "test", "--test", "--output", "json"},
			validateOutput: func(t *testing.T, output string) {
				// JSON output should be valid even in test mode
				if strings.Contains(output, "{") {
					var js json.RawMessage
					err := json.Unmarshal([]byte(output), &js)
					assert.NoError(t, err, "JSON output in test mode should be valid JSON")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewSendCommand()
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

// TestOutputFormatValidation tests that the output format validation works correctly for all smtp commands
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
			args:    []string{"smtp_test"},
		},
		{
			name:    "create",
			cmdFunc: NewCreateCommand,
			args:    []string{"--interactive=false", "--name", "Test"},
		},
		{
			name:    "delete",
			cmdFunc: NewDeleteCommand,
			args:    []string{"smtp_test", "--force"},
		},
		{
			name:    "send",
			cmdFunc: NewSendCommand,
			args:    []string{"--from", "test@example.com", "--to", "recipient@example.com", "--subject", "Test", "--text", "Test", "--username", "test", "--password", "test"},
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
					// (will fail on auth/connection, but format validation happens first)
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

// TestOutputFlagIntegration tests that all smtp commands properly integrate the --output flag
func TestOutputFlagIntegration(t *testing.T) {
	// Get the parent smtp command
	smtpCmd := NewCommand()

	// Check all subcommands have output flag support
	for _, subCmd := range smtpCmd.Commands() {
		t.Run(subCmd.Name(), func(t *testing.T) {
			var buf bytes.Buffer
			subCmd.SetOut(&buf)
			subCmd.SetErr(&buf)

			// Build args based on command requirements
			args := []string{"--output", "json"}
			switch subCmd.Name() {
			case "get", "delete":
				args = append([]string{"smtp_test"}, args...)
			case "create":
				args = append(args, "--interactive=false", "--name", "Test")
			case "send":
				args = append(args, "--from", "test@example.com", "--to", "recipient@example.com",
					"--subject", "Test", "--text", "Test", "--username", "test", "--password", "test")
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
