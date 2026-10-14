package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/config"
	"github.com/AhaSend/ahasend-cli/internal/testenv"
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
	testenv.SetHome(suite.T(), tempDir)

	// Create config manager
	suite.configMgr, err = config.NewManager()
	require.NoError(suite.T(), err)
}

func (suite *AuthIntegrationTestSuite) TearDownTest() {
	os.RemoveAll(suite.tempDir)
}

func (suite *AuthIntegrationTestSuite) TestAuthStatusWithoutProfiles() {
	for _, all := range [][]string{nil, {"--all"}} {
		for _, format := range []string{"table", "plain"} {
			suite.Run(fmt.Sprintf("%s %v", format, all), func() {
				stdout, stderr, code := execRoot(format, "", append([]string{"auth", "status"}, all...)...)

				suite.Equal(0, code, stderr)
				want := "No authentication profiles found\nRun 'ahasend auth login' to add one.\n"
				if all != nil {
					want = "No authentication profiles found\n"
				}
				suite.Equal(want, stdout)
				suite.Empty(stderr)
			})
		}
	}
	for args, want := range map[string]string{
		"auth status":       `{"empty": true, "message": "No authentication profiles found"}`,
		"auth status --all": `{"profiles": []}`,
		"auth logout":       `{"profile": "", "success": true, "message": "No authentication profiles found"}`,
	} {
		suite.Run("json "+args, func() {
			stdout, stderr, code := execRoot("json", "", strings.Fields(args)...)

			suite.Equal(0, code, stderr)
			suite.JSONEq(want, stdout)
			suite.Empty(stderr)
		})
	}
}

func (suite *AuthIntegrationTestSuite) TestProfileManagement() {
	t := suite.T()

	// Create a test profile directly using config manager
	profile := config.Profile{
		APIKey:    "test-key",
		APIURL:    "http://127.0.0.1:1",
		AccountID: "test-account",
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
			APIURL:    "http://127.0.0.1:1",
			AccountID: "dev-account",
		},
		"prod": {
			APIKey:    "prod-key",
			APIURL:    "http://127.0.0.1:1",
			AccountID: "prod-account",
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
