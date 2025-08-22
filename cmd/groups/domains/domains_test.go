package domains

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDomainsCommand_Structure(t *testing.T) {
	// Create a fresh domains command and verify it has expected subcommands
	domainsCmd := NewCommand()
	expectedSubcommands := []string{"create", "delete", "get", "list", "verify"}

	subcommands := make([]string, 0)
	for _, cmd := range domainsCmd.Commands() {
		subcommands = append(subcommands, cmd.Name())
	}

	for _, expected := range expectedSubcommands {
		assert.Contains(t, subcommands, expected, "domains command should have %s subcommand", expected)
	}

	assert.Equal(t, "domains", domainsCmd.Name())
	assert.Equal(t, "Manage your email sending domains", domainsCmd.Short)
	assert.NotEmpty(t, domainsCmd.Long)
}

func TestDomainsCommand_Help(t *testing.T) {
	cmd := NewCommand()

	// Capture help output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	assert.NoError(t, err)

	helpOutput := buf.String()
	assert.Contains(t, helpOutput, "Manage your email sending domains")
	assert.Contains(t, helpOutput, "create")
	assert.Contains(t, helpOutput, "delete")
	assert.Contains(t, helpOutput, "get")
	assert.Contains(t, helpOutput, "list")
	assert.Contains(t, helpOutput, "verify")
}

func TestDomainsCommand_SubcommandCount(t *testing.T) {
	cmd := NewCommand()
	subcommands := cmd.Commands()

	// Should have exactly 5 subcommands
	assert.Equal(t, 5, len(subcommands), "domains command should have exactly 5 subcommands")
}

// Benchmark tests
func BenchmarkDomainsCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewCommand()
	}
}
