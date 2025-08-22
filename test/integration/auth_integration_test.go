package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AhaSend/ahasend-cli/cmd"
	"github.com/AhaSend/ahasend-cli/internal/config"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type AuthIntegrationTestSuite struct {
	suite.Suite
	tempDir   string
	configMgr *config.Manager
}

func (suite *AuthIntegrationTestSuite) SetupTest() {
	tempDir, err := os.MkdirTemp("", "ahasend-integration-test-*")
	require.NoError(suite.T(), err)

	suite.tempDir = tempDir
	suite.T().Setenv("HOME", tempDir)

	// Create config manager
	suite.configMgr, err = config.NewManager()
	require.NoError(suite.T(), err)
}

func (suite *AuthIntegrationTestSuite) TearDownTest() {
	os.RemoveAll(suite.tempDir)
}

func (suite *AuthIntegrationTestSuite) TestAuthWorkflow() {
	t := suite.T()

	// Test auth status when no profiles exist - should run without error
	// Note: Output goes to stderr via formatter, so we just test it doesn't crash
	_, err := testutil.ExecuteCommand(cmd.GetRootCmd(), "auth", "status")
	assert.NoError(t, err)

	// Note: We can't test actual login without mocking the API
	// but we can test the command structure and validation
}

func (suite *AuthIntegrationTestSuite) TestProfileManagement() {
	t := suite.T()

	// Create a test profile directly using config manager
	profile := config.Profile{
		APIKey:    "test-key",
		APIURL:    "https://api.ahasend.com",
		AccountID: "test-account",
		Name:      "Test Profile",
	}

	err := suite.configMgr.SetProfile("test", profile)
	require.NoError(t, err)

	err = suite.configMgr.SetDefaultProfile("test")
	require.NoError(t, err)

	err = suite.configMgr.Save()
	require.NoError(t, err)

	// Test that config file was created
	configPath := filepath.Join(suite.tempDir, ".ahasend", "config.yaml")
	_, err = os.Stat(configPath)
	assert.NoError(t, err)

	// Test loading configuration
	newMgr, err := config.NewManager()
	require.NoError(t, err)

	err = newMgr.Load()
	require.NoError(t, err)

	retrievedProfile := newMgr.GetConfig().Profiles["test"]
	assert.Equal(t, profile, retrievedProfile)

	defaultProfile := newMgr.GetConfig().DefaultProfile
	assert.Equal(t, "test", defaultProfile)
}

func (suite *AuthIntegrationTestSuite) TestMultipleProfiles() {
	t := suite.T()

	// Create multiple profiles
	profiles := map[string]config.Profile{
		"dev": {
			APIKey:    "dev-key",
			APIURL:    "https://api.ahasend.com",
			AccountID: "dev-account",
			Name:      "Development",
		},
		"prod": {
			APIKey:    "prod-key",
			APIURL:    "https://api.ahasend.com",
			AccountID: "prod-account",
			Name:      "Production",
		},
	}

	for name, profile := range profiles {
		err := suite.configMgr.SetProfile(name, profile)
		require.NoError(t, err)
	}

	err := suite.configMgr.SetDefaultProfile("dev")
	require.NoError(t, err)

	err = suite.configMgr.Save()
	require.NoError(t, err)

	// Verify all profiles exist
	profileList := suite.configMgr.ListProfiles()
	assert.Len(t, profileList, 2)
	assert.Contains(t, profileList, "dev")
	assert.Contains(t, profileList, "prod")

	// Verify default profile
	defaultProfile := suite.configMgr.GetConfig().DefaultProfile
	assert.Equal(t, "dev", defaultProfile)

	// Test switching default profile
	err = suite.configMgr.SetDefaultProfile("prod")
	require.NoError(t, err)

	defaultProfile = suite.configMgr.GetConfig().DefaultProfile
	assert.Equal(t, "prod", defaultProfile)
}

func TestAuthIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(AuthIntegrationTestSuite))
}
