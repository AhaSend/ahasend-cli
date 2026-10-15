package integration

import (
	"bytes"
	"os"
	"testing"

	"github.com/AhaSend/ahasend-cli/cmd"
	"github.com/AhaSend/ahasend-cli/internal/testenv"
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
	testenv.SetHome(suite.T(), tempDir)
}

func (suite *DomainsIntegrationTestSuite) TearDownTest() {
	os.RemoveAll(suite.tempDir)
}

func (suite *DomainsIntegrationTestSuite) TestDomainValidation() {
	t := suite.T()

	// Invalid domains fail as validation errors before authentication
	invalidDomains := []string{
		"invalid..domain.com",
		"-invalid.com",
		"invalid.com-",
		"",
	}

	for _, domain := range invalidDomains {
		var stdout, stderr bytes.Buffer
		root := cmd.NewRootCmdForTesting()
		root.SetOut(&stdout)
		root.SetErr(&stderr)

		code := cmd.RunForTesting(root, []string{"domains", "create", domain})

		assert.Equal(t, 4, code, "domain %q: %s", domain, stderr.String())
		assert.Empty(t, stdout.String())
		assert.Contains(t, stderr.String(), "Error:")
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
		var stdout, stderr bytes.Buffer
		root := cmd.NewRootCmdForTesting()
		root.SetOut(&stdout)
		root.SetErr(&stderr)

		code := cmd.RunForTesting(root, cmdArgs)

		assert.Equal(t, 2, code, "%v: %s", cmdArgs, stderr.String())
		assert.Empty(t, stdout.String(), "%v", cmdArgs)
		assert.Equal(t, "Error: no profiles found. Run 'ahasend auth login' to authenticate\n",
			stderr.String(), "%v", cmdArgs)
	}
}

func TestDomainsIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(DomainsIntegrationTestSuite))
}
