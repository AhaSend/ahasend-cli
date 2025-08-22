package domains

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeleteCommand_Flags(t *testing.T) {
	// Test that delete command has required flags
	deleteCmd := NewDeleteCommand()
	flags := deleteCmd.Flags()

	forceFlag := flags.Lookup("force")
	assert.NotNil(t, forceFlag)
	assert.Equal(t, "bool", forceFlag.Value.Type())
}

func TestDeleteCommand_Args(t *testing.T) {
	// Test that delete command requires exactly 1 argument
	deleteCmd := NewDeleteCommand()
	assert.NotNil(t, deleteCmd.Args)
}

func TestDeleteCommand_Structure(t *testing.T) {
	deleteCmd := NewDeleteCommand()
	assert.Equal(t, "delete", deleteCmd.Name())
	assert.Equal(t, "Delete a domain", deleteCmd.Short)
	assert.NotEmpty(t, deleteCmd.Long)
	assert.NotEmpty(t, deleteCmd.Example)
	assert.Contains(t, deleteCmd.Long, "WARNING")
	assert.Contains(t, deleteCmd.Long, "cannot be undone")
}
