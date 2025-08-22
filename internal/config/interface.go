package config

// ConfigManager defines the interface for configuration management operations
// This interface allows for better testability and mocking
type ConfigManager interface {
	// Configuration file operations
	Load() error
	Save() error
	GetConfig() *Config

	// Profile management
	GetCurrentProfile() (*Profile, error)
	SetProfile(name string, profile Profile) error
	RemoveProfile(name string) error
	ListProfiles() []string
	SetDefaultProfile(name string) error

	// Preference management
	SetPreference(key, value string) error
	GetPreference(key string) (string, error)
}

// Ensure Manager implements ConfigManager interface
var _ ConfigManager = (*Manager)(nil)
