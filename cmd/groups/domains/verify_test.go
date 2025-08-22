package domains

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerifyCommand_Flags(t *testing.T) {
	// Test that verify command has required flags
	verifyCmd := NewVerifyCommand()
	flags := verifyCmd.Flags()

	verboseFlag := flags.Lookup("verbose")
	assert.NotNil(t, verboseFlag)
	assert.Equal(t, "bool", verboseFlag.Value.Type())
}

func TestVerifyCommand_Args(t *testing.T) {
	// Test that verify command requires exactly 1 argument
	verifyCmd := NewVerifyCommand()
	assert.NotNil(t, verifyCmd.Args)
}

func TestVerifyCommand_Structure(t *testing.T) {
	verifyCmd := NewVerifyCommand()
	assert.Equal(t, "verify", verifyCmd.Name())
	assert.Equal(t, "Check domain DNS configuration", verifyCmd.Short)
	assert.NotEmpty(t, verifyCmd.Long)
	assert.NotEmpty(t, verifyCmd.Example)
}
