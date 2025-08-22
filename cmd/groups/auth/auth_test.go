package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthCommandStructure(t *testing.T) {
	// Create a fresh auth command and verify it has expected subcommands
	authCmd := NewCommand()
	expectedSubcommands := []string{"login", "logout", "status", "switch"}

	subcommands := make([]string, 0)
	for _, cmd := range authCmd.Commands() {
		subcommands = append(subcommands, cmd.Name())
	}

	for _, expected := range expectedSubcommands {
		assert.Contains(t, subcommands, expected, "auth command should have %s subcommand", expected)
	}

	assert.Equal(t, "auth", authCmd.Name())
	assert.Equal(t, "Manage authentication and profiles", authCmd.Short)
	assert.NotEmpty(t, authCmd.Long)
}
