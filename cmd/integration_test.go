package cmd

import (
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHierarchicalCommandStructure verifies the new hierarchical command structure works end-to-end
func TestHierarchicalCommandStructure(t *testing.T) {
	// Test auth command group
	t.Run("AuthCommandGroup", func(t *testing.T) {
		output, err := testutil.ExecuteCommand(GetRootCmd(), "auth", "--help")
		require.NoError(t, err)

		assert.Contains(t, output, "Authenticate with AhaSend and manage multiple authentication profiles")
		assert.Contains(t, output, "login")
		assert.Contains(t, output, "logout")
		assert.Contains(t, output, "status")
		assert.Contains(t, output, "switch")

		// Test individual auth subcommands
		authSubcommands := []string{"login", "logout", "status", "switch"}
		for _, subcmd := range authSubcommands {
			t.Run("auth_"+subcmd, func(t *testing.T) {
				output, err := testutil.ExecuteCommand(GetRootCmd(), "auth", subcmd, "--help")
				require.NoError(t, err)
				assert.NotEmpty(t, output)
			})
		}
	})

	// Test domains command group
	t.Run("DomainsCommandGroup", func(t *testing.T) {
		output, err := testutil.ExecuteCommand(GetRootCmd(), "domains", "--help")
		require.NoError(t, err)

		assert.Contains(t, output, "Manage your email sending domains")
		assert.Contains(t, output, "create")
		assert.Contains(t, output, "delete")
		assert.Contains(t, output, "get")
		assert.Contains(t, output, "list")
		assert.Contains(t, output, "verify")

		// Test individual domains subcommands
		domainsSubcommands := []string{"create", "delete", "get", "list", "verify"}
		for _, subcmd := range domainsSubcommands {
			t.Run("domains_"+subcmd, func(t *testing.T) {
				output, err := testutil.ExecuteCommand(GetRootCmd(), "domains", subcmd, "--help")
				require.NoError(t, err)
				assert.NotEmpty(t, output)
			})
		}
	})
}

// TestCommandIsolation ensures commands can be created independently without state contamination
func TestCommandIsolation(t *testing.T) {
	// Create multiple root command instances and verify they're independent
	for i := 0; i < 3; i++ {
		t.Run("isolated_instance", func(t *testing.T) {
			rootCmd := NewRootCmdForTesting()

			// Verify command structure
			assert.Equal(t, "ahasend", rootCmd.Name())

			// Check that auth and domains commands are present
			var hasAuth, hasDomains bool
			for _, cmd := range rootCmd.Commands() {
				if cmd.Name() == "auth" {
					hasAuth = true
				}
				if cmd.Name() == "domains" {
					hasDomains = true
				}
			}

			assert.True(t, hasAuth, "Auth command group should be present")
			assert.True(t, hasDomains, "Domains command group should be present")
		})
	}
}

// TestRootCommandStructure verifies the root command has the expected structure
func TestRootCommandStructure(t *testing.T) {
	rootCmd := GetRootCmd()

	assert.Equal(t, "ahasend", rootCmd.Name())
	assert.Equal(t, "AhaSend CLI - Command line interface for AhaSend email service", rootCmd.Short)
	assert.NotEmpty(t, rootCmd.Long)

	// Check for required global flags
	flags := rootCmd.PersistentFlags()
	requiredFlags := []string{"api-key", "account-id", "profile", "output", "no-color", "verbose", "debug"}

	for _, flagName := range requiredFlags {
		flag := flags.Lookup(flagName)
		assert.NotNil(t, flag, "Global flag '%s' should be present", flagName)
	}
}
