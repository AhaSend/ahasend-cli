// Package config provides comprehensive configuration management for the AhaSend CLI.
//
// This package handles all aspects of CLI configuration including:
//
//   - Multi-profile authentication with API keys and account IDs
//   - User preferences (output format, colors, timeouts, concurrency)
//   - Configuration file loading and saving (YAML format)
//   - Profile switching and management operations
//   - Validation of configuration values and preferences
//   - Backward compatibility with existing configuration files
//
// The package provides both a legacy Manager for compatibility and a new
// ManagerV2 with improved architecture using focused components. All configuration
// is stored in ~/.ahasend/config.yaml following standard CLI conventions.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

// Config represents the CLI configuration
type Config struct {
	DefaultProfile string             `mapstructure:"default_profile" yaml:"default_profile"`
	Profiles       map[string]Profile `mapstructure:"profiles" yaml:"profiles"`
	Preferences    Preferences        `mapstructure:"preferences" yaml:"preferences"`
}

// Profile represents an AhaSend account profile
type Profile struct {
	APIKey         string    `mapstructure:"api_key" yaml:"api_key"`
	APIURL         string    `mapstructure:"api_url" yaml:"api_url"`
	AccountID      string    `mapstructure:"account_id" yaml:"account_id"`
	Name           string    `mapstructure:"name" yaml:"name"`
	AccountName    string    `mapstructure:"account_name" yaml:"account_name,omitempty"`
	AccountUpdated time.Time `mapstructure:"account_updated" yaml:"account_updated,omitempty"`
}

// Preferences represents user preferences for the CLI
type Preferences struct {
	OutputFormat     string `mapstructure:"output_format" yaml:"output_format"`
	ColorOutput      bool   `mapstructure:"color_output" yaml:"color_output"`
	WebhookTimeout   string `mapstructure:"webhook_timeout" yaml:"webhook_timeout"`
	LogLevel         string `mapstructure:"log_level" yaml:"log_level"`
	DefaultDomain    string `mapstructure:"default_domain" yaml:"default_domain"`
	BatchConcurrency int    `mapstructure:"batch_concurrency" yaml:"batch_concurrency"`
}

// DefaultPreferences returns default preferences
func DefaultPreferences() Preferences {
	return Preferences{
		OutputFormat:     "table",
		ColorOutput:      true,
		WebhookTimeout:   "30s",
		LogLevel:         "info",
		BatchConcurrency: 5,
	}
}

// Manager handles configuration loading and saving
// This is kept for backward compatibility - new code should use ManagerV2
type Manager struct {
	configDir  string
	configFile string
	config     *Config

	// Internal components (for gradual migration)
	profileManager    *ProfileManager
	preferenceManager *PreferenceManager
}

// NewManager creates a new configuration manager
func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".ahasend")
	configFile := filepath.Join(configDir, "config.yaml")

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	config := &Config{
		DefaultProfile: "default",
		Profiles:       make(map[string]Profile),
		Preferences:    DefaultPreferences(),
	}

	manager := &Manager{
		configDir:         configDir,
		configFile:        configFile,
		config:            config,
		profileManager:    NewProfileManager(config),
		preferenceManager: NewPreferenceManager(config),
	}

	return manager, nil
}

// Load loads configuration from file
func (m *Manager) Load() error {
	viper.SetConfigFile(m.configFile)
	viper.SetConfigType("yaml")

	// Set default values
	viper.SetDefault("default_profile", "default")
	viper.SetDefault("preferences.output_format", "table")
	viper.SetDefault("preferences.color_output", true)
	viper.SetDefault("preferences.webhook_timeout", "30s")
	viper.SetDefault("preferences.log_level", "info")
	viper.SetDefault("preferences.batch_concurrency", 5)

	// Read environment variables
	viper.SetEnvPrefix("AHASEND")
	viper.AutomaticEnv()

	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		// File doesn't exist, create with defaults
		return m.Save()
	}

	// Unmarshal config
	if err := viper.Unmarshal(m.config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Reinitialize managers with loaded config
	m.profileManager = NewProfileManager(m.config)
	m.preferenceManager = NewPreferenceManager(m.config)

	return nil
}

// Save saves configuration to file
func (m *Manager) Save() error {
	viper.Set("default_profile", m.config.DefaultProfile)
	viper.Set("profiles", m.config.Profiles)
	viper.Set("preferences", m.config.Preferences)

	return viper.WriteConfigAs(m.configFile)
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *Config {
	return m.config
}

// GetCurrentProfile returns the current active profile
func (m *Manager) GetCurrentProfile() (*Profile, error) {
	return m.profileManager.GetCurrentProfile()
}

// SetProfile adds or updates a profile
func (m *Manager) SetProfile(name string, profile Profile) error {
	err := m.profileManager.SetProfile(name, profile)
	if err != nil {
		return err
	}
	return m.Save()
}

// RemoveProfile removes a profile
func (m *Manager) RemoveProfile(name string) error {
	err := m.profileManager.RemoveProfile(name)
	if err != nil {
		return err
	}
	return m.Save()
}

// SetDefaultProfile sets the default profile
func (m *Manager) SetDefaultProfile(name string) error {
	err := m.profileManager.SetDefaultProfile(name)
	if err != nil {
		return err
	}
	return m.Save()
}

// ListProfiles returns all profile names
func (m *Manager) ListProfiles() []string {
	return m.profileManager.ListProfiles()
}

// SetPreference sets a preference value
func (m *Manager) SetPreference(key, value string) error {
	err := m.preferenceManager.SetPreference(key, value)
	if err != nil {
		return err
	}
	return m.Save()
}

// GetPreference gets a preference value
func (m *Manager) GetPreference(key string) (string, error) {
	return m.preferenceManager.GetPreference(key)
}
