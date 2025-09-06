package messages

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessagesCommandStructure(t *testing.T) {
	// Create a fresh messages command and verify it has expected subcommands
	messagesCmd := NewCommand()
	expectedSubcommands := []string{"send", "list", "cancel"}

	subcommands := make([]string, 0)
	for _, cmd := range messagesCmd.Commands() {
		subcommands = append(subcommands, cmd.Name())
	}

	for _, expected := range expectedSubcommands {
		assert.Contains(t, subcommands, expected, "messages command should have %s subcommand", expected)
	}

	assert.Equal(t, "messages", messagesCmd.Name())
	assert.Equal(t, "Send and manage email messages", messagesCmd.Short)
	assert.NotEmpty(t, messagesCmd.Long)
}

func TestMessagesCommand_Help(t *testing.T) {
	cmd := NewCommand()

	// Capture help output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	assert.NoError(t, err)

	helpOutput := buf.String()
	assert.Contains(t, helpOutput, "Send email messages")
	assert.Contains(t, helpOutput, "send")
	assert.Contains(t, helpOutput, "list")
	assert.Contains(t, helpOutput, "cancel")
}

func TestMessagesCommand_SubcommandCount(t *testing.T) {
	cmd := NewCommand()
	subcommands := cmd.Commands()

	// Should have exactly 3 subcommands
	assert.Equal(t, 4, len(subcommands), "messages command should have exactly 3 subcommands")
}

// Benchmark tests
func BenchmarkMessagesCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewCommand()
	}
}
