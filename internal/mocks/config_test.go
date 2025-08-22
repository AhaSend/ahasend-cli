package mocks

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AhaSend/ahasend-cli/internal/config"
)

func TestMockConfigManager_InterfaceCompliance(t *testing.T) {
	// This test verifies that MockConfigManager implements the ConfigManager interface
	var _ config.ConfigManager = (*MockConfigManager)(nil)
}

func TestMockConfigManager_ConfigurationMethods(t *testing.T) {
	mockConfigManager := &MockConfigManager{}

	// Test Load
	mockConfigManager.On("Load").Return(nil)
	err := mockConfigManager.Load()
	assert.NoError(t, err)
	mockConfigManager.AssertExpectations(t)

	// Test Save
	mockConfigManager = &MockConfigManager{}
	mockConfigManager.On("Save").Return(nil)
	err = mockConfigManager.Save()
	assert.NoError(t, err)
	mockConfigManager.AssertExpectations(t)

	// Test GetConfig
	mockConfigManager = &MockConfigManager{}
	expectedConfig := &config.Config{
		DefaultProfile: "default",
		Profiles:       make(map[string]config.Profile),
		Preferences:    config.DefaultPreferences(),
	}
	mockConfigManager.On("GetConfig").Return(expectedConfig)

	cfg := mockConfigManager.GetConfig()
	assert.Equal(t, expectedConfig, cfg)
	mockConfigManager.AssertExpectations(t)
}

func TestMockConfigManager_ProfileMethods(t *testing.T) {
	mockConfigManager := &MockConfigManager{}

	// Test SetProfile
	profile := config.Profile{
		Name:      "test",
		APIKey:    "test-key",
		AccountID: "test-account",
		APIURL:    "https://api.ahasend.com",
	}
	mockConfigManager.On("SetProfile", "test", profile).Return(nil)

	err := mockConfigManager.SetProfile("test", profile)
	assert.NoError(t, err)
	mockConfigManager.AssertExpectations(t)

	// Test GetCurrentProfile
	mockConfigManager = &MockConfigManager{}
	mockConfigManager.On("GetCurrentProfile").Return(&profile, nil)

	currentProfile, err := mockConfigManager.GetCurrentProfile()
	assert.NoError(t, err)
	assert.Equal(t, &profile, currentProfile)
	mockConfigManager.AssertExpectations(t)

	// Test ListProfiles
	mockConfigManager = &MockConfigManager{}
	expectedProfiles := []string{"default", "test"}
	mockConfigManager.On("ListProfiles").Return(expectedProfiles)

	profiles := mockConfigManager.ListProfiles()
	assert.Equal(t, expectedProfiles, profiles)
	mockConfigManager.AssertExpectations(t)

	// Test RemoveProfile
	mockConfigManager = &MockConfigManager{}
	mockConfigManager.On("RemoveProfile", "test").Return(nil)

	err = mockConfigManager.RemoveProfile("test")
	assert.NoError(t, err)
	mockConfigManager.AssertExpectations(t)

	// Test SetDefaultProfile
	mockConfigManager = &MockConfigManager{}
	mockConfigManager.On("SetDefaultProfile", "test").Return(nil)

	err = mockConfigManager.SetDefaultProfile("test")
	assert.NoError(t, err)
	mockConfigManager.AssertExpectations(t)
}

func TestMockConfigManager_PreferenceMethods(t *testing.T) {
	mockConfigManager := &MockConfigManager{}

	// Test SetPreference
	mockConfigManager.On("SetPreference", "output_format", "json").Return(nil)

	err := mockConfigManager.SetPreference("output_format", "json")
	assert.NoError(t, err)
	mockConfigManager.AssertExpectations(t)

	// Test GetPreference
	mockConfigManager = &MockConfigManager{}
	mockConfigManager.On("GetPreference", "output_format").Return("json", nil)

	value, err := mockConfigManager.GetPreference("output_format")
	assert.NoError(t, err)
	assert.Equal(t, "json", value)
	mockConfigManager.AssertExpectations(t)
}

func TestMockConfigManager_HelperMethods(t *testing.T) {
	mockConfigManager := &MockConfigManager{}

	// Test helper method for creating mock profile
	profile := mockConfigManager.NewMockProfile("test", "api-key", "account-123")
	assert.Equal(t, "test", profile.Name)
	assert.Equal(t, "api-key", profile.APIKey)
	assert.Equal(t, "account-123", profile.AccountID)
	assert.Equal(t, "https://api.ahasend.com", profile.APIURL)

	// Test helper method for creating mock config
	cfg := mockConfigManager.NewMockConfig()
	assert.Equal(t, "default", cfg.DefaultProfile)
	assert.NotNil(t, cfg.Profiles)
	assert.Equal(t, config.DefaultPreferences(), cfg.Preferences)
}

func TestMockConfigManager_ErrorHandling(t *testing.T) {
	mockConfigManager := &MockConfigManager{}

	// Test error cases return nil for pointers and proper errors
	mockConfigManager.On("GetCurrentProfile").Return(nil, assert.AnError)

	profile, err := mockConfigManager.GetCurrentProfile()
	assert.Error(t, err)
	assert.Nil(t, profile)
	mockConfigManager.AssertExpectations(t)

	// Test error in Load operation
	mockConfigManager = &MockConfigManager{}
	mockConfigManager.On("Load").Return(assert.AnError)

	err = mockConfigManager.Load()
	assert.Error(t, err)
	mockConfigManager.AssertExpectations(t)
}

// BenchmarkMockConfigManager_BasicOperations benchmarks basic mock operations
func BenchmarkMockConfigManager_BasicOperations(b *testing.B) {
	mockConfigManager := &MockConfigManager{}
	mockConfigManager.On("GetPreference", "output_format").Return("table", nil)
	mockConfigManager.On("SetPreference", "output_format", "json").Return(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mockConfigManager.GetPreference("output_format")
		_ = mockConfigManager.SetPreference("output_format", "json")
	}
}
