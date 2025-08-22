package integration

import (
	"os"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-cli/cmd"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type DomainsIntegrationTestSuite struct {
	suite.Suite
	tempDir string
}

func (suite *DomainsIntegrationTestSuite) SetupTest() {
	tempDir, err := os.MkdirTemp("", "ahasend-integration-test-*")
	require.NoError(suite.T(), err)

	suite.tempDir = tempDir
	suite.T().Setenv("HOME", tempDir)
}

func (suite *DomainsIntegrationTestSuite) TearDownTest() {
	os.RemoveAll(suite.tempDir)
}

func (suite *DomainsIntegrationTestSuite) TestDomainsCommandStructure() {
	t := suite.T()

	// Test domains help
	output, err := testutil.ExecuteCommand(cmd.GetRootCmd(), "domains", "--help")
	require.NoError(t, err)

	assert.Contains(t, output, "Manage your email sending domains")
	assert.Contains(t, output, "create")
	assert.Contains(t, output, "delete")
	assert.Contains(t, output, "get")
	assert.Contains(t, output, "list")
	assert.Contains(t, output, "verify")
}

func (suite *DomainsIntegrationTestSuite) TestDomainsSubcommands() {
	t := suite.T()

	// Test each subcommand help
	subcommands := []string{"create", "delete", "get", "list", "verify"}

	for _, subcmd := range subcommands {
		output, err := testutil.ExecuteCommand(cmd.GetRootCmd(), "domains", subcmd, "--help")
		require.NoError(t, err, "Failed to get help for domains %s", subcmd)

		switch subcmd {
		case "create":
			assert.Contains(t, output, "Create a new domain in your AhaSend account")
			assert.Contains(t, output, "--format")
			assert.Contains(t, output, "--no-dns-help")
		case "delete":
			assert.Contains(t, output, "Delete a domain")
			assert.Contains(t, output, "--force")
			assert.Contains(t, output, "cannot be undone")
		case "get":
			assert.Contains(t, output, "Get detailed information about a specific domain")
		case "list":
			assert.Contains(t, output, "List all domains")
			assert.Contains(t, output, "--limit")
			assert.Contains(t, output, "--cursor")
			assert.Contains(t, output, "--status")
		case "verify":
			assert.Contains(t, output, "Check the DNS configuration status")
			assert.Contains(t, output, "--verbose")
		}
	}
}

func (suite *DomainsIntegrationTestSuite) TestDomainValidation() {
	t := suite.T()

	// Test domain validation with invalid domains
	// Note: These will fail with auth errors, but should validate domain format first
	invalidDomains := []string{
		"invalid..domain.com",
		"-invalid.com",
		"invalid.com-",
		"",
	}

	for _, domain := range invalidDomains {
		// We expect these to fail, but for validation reasons
		output, err := testutil.ExecuteCommand(cmd.GetRootCmd(), "domains", "create", domain)

		// With new error handling, errors are printed but not returned
		// Should get validation error, usage info, or auth error
		assert.NoError(t, err, "Command should handle errors internally")
		// The exact response depends on where validation occurs
		// We just ensure the command handles invalid input gracefully
		hasErrorOrUsage := strings.Contains(output, "Error:") || strings.Contains(output, "Usage:")
		assert.True(t, hasErrorOrUsage, "Should show error message or usage info in output: %s", output)
		assert.NotEmpty(t, output)
	}
}

func (suite *DomainsIntegrationTestSuite) TestDomainCommandsRequireAuth() {
	t := suite.T()

	// Test that domain commands require authentication
	commandsRequiringAuth := [][]string{
		{"domains", "list"},
		{"domains", "create", "example.com"},
		{"domains", "get", "example.com"},
		{"domains", "verify", "example.com"},
		{"domains", "delete", "example.com", "--force"},
	}

	for _, cmdArgs := range commandsRequiringAuth {
		output, err := testutil.ExecuteCommand(cmd.GetRootCmd(), cmdArgs...)

		// With new error handling, errors are printed but not returned
		// Check that error message appears in output instead
		assert.NoError(t, err, "Command should handle errors internally")
		assert.Contains(t, output, "Error:", "Should show error message in output")
		assert.NotEmpty(t, output)
	}
}

func TestDomainsIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(DomainsIntegrationTestSuite))
}
