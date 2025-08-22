package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	tests := []struct {
		name      string
		setupHome func() string
		wantErr   bool
	}{
		{
			name: "valid home directory",
			setupHome: func() string {
				tmpDir, _ := os.MkdirTemp("", "ahasend-test-*")
				return tmpDir
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupHome != nil {
				homeDir := tt.setupHome()
				defer os.RemoveAll(homeDir)
				t.Setenv("HOME", homeDir)
			}

			mgr, err := NewManager()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, mgr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, mgr)
			}
		})
	}
}

func TestManager_SetAndGetProfile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ahasend-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	t.Setenv("HOME", tmpDir)

	mgr, err := NewManager()
	require.NoError(t, err)

	profile := Profile{
		APIKey:    "test-key",
		APIURL:    "https://api.ahasend.com",
		AccountID: "test-account",
		Name:      "Test Profile",
	}

	// Test setting profile
	err = mgr.SetProfile("test", profile)
	assert.NoError(t, err)

	// Test getting profile
	retrievedProfile := mgr.GetConfig().Profiles["test"]
	assert.Equal(t, profile, retrievedProfile)

	// Test getting non-existent profile
	_, exists := mgr.GetConfig().Profiles["nonexistent"]
	assert.False(t, exists)
}

func TestManager_ListProfiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ahasend-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	t.Setenv("HOME", tmpDir)

	mgr, err := NewManager()
	require.NoError(t, err)

	// Initially empty
	profiles := mgr.ListProfiles()
	assert.Empty(t, profiles)

	// Add profiles
	profile1 := Profile{APIKey: "key1", AccountID: "account1"}
	profile2 := Profile{APIKey: "key2", AccountID: "account2"}

	err = mgr.SetProfile("profile1", profile1)
	require.NoError(t, err)
	err = mgr.SetProfile("profile2", profile2)
	require.NoError(t, err)

	profiles = mgr.ListProfiles()
	assert.Len(t, profiles, 2)
	assert.Contains(t, profiles, "profile1")
	assert.Contains(t, profiles, "profile2")
}

func TestManager_DefaultProfile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ahasend-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	t.Setenv("HOME", tmpDir)

	mgr, err := NewManager()
	require.NoError(t, err)

	// Default profile should be "default" initially
	defaultProfile := mgr.GetConfig().DefaultProfile
	assert.Equal(t, "default", defaultProfile)

	// Set default
	err = mgr.SetProfile("test", Profile{APIKey: "test"})
	require.NoError(t, err)
	err = mgr.SetDefaultProfile("test")
	assert.NoError(t, err)

	defaultProfile = mgr.GetConfig().DefaultProfile
	assert.Equal(t, "test", defaultProfile)
}

func TestManager_DeleteProfile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ahasend-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	t.Setenv("HOME", tmpDir)

	mgr, err := NewManager()
	require.NoError(t, err)

	profile := Profile{APIKey: "test-key", AccountID: "test-account"}
	err = mgr.SetProfile("test", profile)
	require.NoError(t, err)

	// Verify profile exists
	_, exists := mgr.GetConfig().Profiles["test"]
	assert.True(t, exists)

	// Delete profile
	err = mgr.RemoveProfile("test")
	assert.NoError(t, err)

	// Verify profile is gone
	_, exists = mgr.GetConfig().Profiles["test"]
	assert.False(t, exists)
}

func TestManager_LoadAndSave(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ahasend-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	t.Setenv("HOME", tmpDir)

	// Create first manager and save data
	mgr1, err := NewManager()
	require.NoError(t, err)

	profile := Profile{
		APIKey:    "test-key",
		APIURL:    "https://api.ahasend.com",
		AccountID: "test-account",
		Name:      "Test Profile",
	}

	err = mgr1.SetProfile("test", profile)
	require.NoError(t, err)
	err = mgr1.SetDefaultProfile("test")
	require.NoError(t, err)
	err = mgr1.Save()
	require.NoError(t, err)

	// Create second manager and load data
	mgr2, err := NewManager()
	require.NoError(t, err)
	err = mgr2.Load()
	require.NoError(t, err)

	// Verify data was loaded correctly
	retrieved := mgr2.GetConfig().Profiles["test"]
	assert.Equal(t, profile, retrieved)

	defaultProfile := mgr2.GetConfig().DefaultProfile
	assert.Equal(t, "test", defaultProfile)
}

func TestManager_SaveCreatesDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ahasend-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	t.Setenv("HOME", tmpDir)

	// NewManager creates the directory during initialization
	mgr, err := NewManager()
	require.NoError(t, err)

	// Verify directory was created by NewManager
	configDir := filepath.Join(tmpDir, ".ahasend")
	_, err = os.Stat(configDir)
	assert.NoError(t, err)

	// Save should work with existing directory
	err = mgr.Save()
	assert.NoError(t, err)

	// Verify config file was created
	configFile := filepath.Join(configDir, "config.yaml")
	_, err = os.Stat(configFile)
	assert.NoError(t, err)
}
