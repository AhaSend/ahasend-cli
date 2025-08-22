package messages

import (
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestCancelCommand(t *testing.T) {
	// Test command structure
	cancelCmd := NewCancelCommand()

	assert.Equal(t, "cancel", cancelCmd.Name())
	assert.Equal(t, "Cancel a scheduled message", cancelCmd.Short)
	assert.NotEmpty(t, cancelCmd.Long)
	assert.NotEmpty(t, cancelCmd.Example)

	// Test that it requires at least one argument
	err := cancelCmd.Args(nil, []string{})
	assert.Error(t, err, "should require at least one argument")
	assert.Contains(t, err.Error(), "at least 1 arg")

	// Test flags
	forceFlag := cancelCmd.Flag("force")
	assert.NotNil(t, forceFlag)
	assert.Equal(t, "f", forceFlag.Shorthand)
	assert.Equal(t, "Skip confirmation prompt", forceFlag.Usage)
}

func TestCancelCommandValidation(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedError string
	}{
		{
			name:          "missing message ID",
			args:          []string{"cancel"},
			expectedError: "requires at least 1 arg(s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute command
			output, err := testutil.ExecuteCommand(NewCommand(), tt.args...)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			_ = output // Output might be empty for validation errors
		})
	}
}

// TestCancelCommandIntegration would test with real client
// but we skip it for unit tests as it requires auth setup
func TestCancelCommandHelp(t *testing.T) {
	cancelCmd := NewCancelCommand()

	// Check that help can be generated without errors
	helpFunc := cancelCmd.HelpFunc()
	assert.NotNil(t, helpFunc)

	// Check examples are present
	assert.Contains(t, cancelCmd.Example, "ahasend messages cancel")
	assert.Contains(t, cancelCmd.Example, "550e8400-e29b-41d4-a716-446655440000")
}
