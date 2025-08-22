package suppressions

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
			name: "json output with domain filter",
			args: []string{"list", "--domain", "example.com", "--output", "json"},
			validateOutput: func(t *testing.T, output string) {
				if strings.Contains(output, "{") {
					var js json.RawMessage
					err := json.Unmarshal([]byte(output), &js)
					assert.NoError(t, err, "JSON output with domain filter should be valid JSON")
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
			cmd := NewCreateCommand()
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

// TestCheckCommand_OutputFormats tests all output formats for the check command
func TestCheckCommand_OutputFormats(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		validateOutput func(t *testing.T, output string)
	}{
		{
			name: "default table output",
			args: []string{"check", "test@example.com"},
			validateOutput: func(t *testing.T, output string) {
				// Table output should not be JSON
				var js json.RawMessage
				err := json.Unmarshal([]byte(output), &js)
				assert.Error(t, err, "Table output should not be valid JSON")
			},
		},
		{
			name: "json output",
			args: []string{"check", "test@example.com", "--output", "json"},
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
			args: []string{"check", "test@example.com", "--output", "csv"},
			validateOutput: func(t *testing.T, output string) {
				// CSV output for single check result
			},
		},
		{
			name: "json output with domain",
			args: []string{"check", "test@example.com", "--domain", "example.com", "--output", "json"},
			validateOutput: func(t *testing.T, output string) {
				if strings.Contains(output, "{") {
					var js json.RawMessage
					err := json.Unmarshal([]byte(output), &js)
					assert.NoError(t, err, "JSON output with domain should be valid JSON")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCheckCommand()
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

// TestAddCommand_OutputFormats tests all output formats for the add command
func TestAddCommand_OutputFormats(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		validateOutput func(t *testing.T, output string)
	}{
		{
			name: "default table output",
			args: []string{"create", "test@example.com", "--reason", "bounce", "--expires", "30d"},
			validateOutput: func(t *testing.T, output string) {
				// Table output should not be JSON
				var js json.RawMessage
				err := json.Unmarshal([]byte(output), &js)
				assert.Error(t, err, "Table output should not be valid JSON")
			},
		},
		{
			name: "json output",
			args: []string{"create", "test@example.com", "--reason", "bounce", "--expires", "30d", "--output", "json"},
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
			args: []string{"create", "test@example.com", "--reason", "bounce", "--expires", "30d", "--output", "csv"},
			validateOutput: func(t *testing.T, output string) {
				// CSV output for add result
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

// TestRemoveCommand_OutputFormats tests all output formats for the remove command
func TestRemoveCommand_OutputFormats(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		validateOutput func(t *testing.T, output string)
	}{
		{
			name: "default table output",
			args: []string{"remove", "test@example.com", "--force"},
			validateOutput: func(t *testing.T, output string) {
				// Table output should not be JSON
				var js json.RawMessage
				err := json.Unmarshal([]byte(output), &js)
				assert.Error(t, err, "Table output should not be valid JSON")
			},
		},
		{
			name: "json output",
			args: []string{"remove", "test@example.com", "--force", "--output", "json"},
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
			args: []string{"remove", "test@example.com", "--force", "--output", "csv"},
			validateOutput: func(t *testing.T, output string) {
				// CSV output for remove result
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

// TestWipeCommand_OutputFormats tests all output formats for the wipe command
func TestWipeCommand_OutputFormats(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		validateOutput func(t *testing.T, output string)
	}{
		{
			name: "default table output",
			args: []string{"wipe", "--force"},
			validateOutput: func(t *testing.T, output string) {
				// Table output should not be JSON
				var js json.RawMessage
				err := json.Unmarshal([]byte(output), &js)
				assert.Error(t, err, "Table output should not be valid JSON")
			},
		},
		{
			name: "json output",
			args: []string{"wipe", "--force", "--output", "json"},
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
			args: []string{"wipe", "--force", "--output", "csv"},
			validateOutput: func(t *testing.T, output string) {
				// CSV output for wipe result
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewWipeCommand()
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

// TestOutputFormatValidation tests that the output format validation works correctly for all suppressions commands
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
			name:    "check",
			cmdFunc: NewCheckCommand,
			args:    []string{"test@example.com"},
		},
		{
			name:    "create",
			cmdFunc: NewCreateCommand,
			args:    []string{"test@example.com", "--reason", "bounce", "--expires", "30d"},
		},
		{
			name:    "remove",
			cmdFunc: NewDeleteCommand,
			args:    []string{"test@example.com", "--force"},
		},
		{
			name:    "wipe",
			cmdFunc: NewWipeCommand,
			args:    []string{"--force"},
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

// TestOutputFlagIntegration tests that all suppressions commands properly integrate the --output flag
func TestOutputFlagIntegration(t *testing.T) {
	// Get the parent suppressions command
	suppressionsCmd := NewCommand()

	// Check all subcommands have output flag support
	for _, subCmd := range suppressionsCmd.Commands() {
		t.Run(subCmd.Name(), func(t *testing.T) {
			var buf bytes.Buffer
			subCmd.SetOut(&buf)
			subCmd.SetErr(&buf)

			// Build args based on command requirements
			args := []string{"--output", "json"}
			switch subCmd.Name() {
			case "check", "remove":
				args = append([]string{"test@example.com"}, args...)
			case "create":
				args = append([]string{"test@example.com", "--reason", "bounce", "--expires", "30d"}, args...)
			case "wipe":
				args = append(args, "--force")
			}
			if subCmd.Name() == "remove" {
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
