package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/AhaSend/ahasend-cli/internal/config"
)

// MockConfigManager is a mock implementation of the ConfigManager interface
type MockConfigManager struct {
	mock.Mock
}

// Ensure MockConfigManager implements ConfigManager interface
var _ config.ConfigManager = (*MockConfigManager)(nil)

// Configuration file operations

func (m *MockConfigManager) Load() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockConfigManager) Save() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockConfigManager) GetConfig() *config.Config {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*config.Config)
}

// Profile management methods

func (m *MockConfigManager) GetCurrentProfile() (*config.Profile, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*config.Profile), args.Error(1)
}

func (m *MockConfigManager) SetProfile(name string, profile config.Profile) error {
	args := m.Called(name, profile)
	return args.Error(0)
}

func (m *MockConfigManager) RemoveProfile(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *MockConfigManager) ListProfiles() []string {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]string)
}

func (m *MockConfigManager) SetDefaultProfile(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

// Preference management methods

func (m *MockConfigManager) SetPreference(key, value string) error {
	args := m.Called(key, value)
	return args.Error(0)
}

func (m *MockConfigManager) GetPreference(key string) (string, error) {
	args := m.Called(key)
	return args.String(0), args.Error(1)
}

// Helper methods for creating mock data

// NewMockProfile creates a mock profile for testing
func (m *MockConfigManager) NewMockProfile(name, apiKey, accountID string) config.Profile {
	return config.Profile{
		Name:      name,
		APIKey:    apiKey,
		AccountID: accountID,
		APIURL:    "https://api.ahasend.com",
	}
}

// NewMockConfig creates a mock config for testing
func (m *MockConfigManager) NewMockConfig() *config.Config {
	return &config.Config{
		DefaultProfile: "default",
		Profiles:       make(map[string]config.Profile),
		Preferences:    config.DefaultPreferences(),
	}
}
