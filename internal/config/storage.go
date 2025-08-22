package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// ConfigStorage handles configuration file operations
type ConfigStorage struct {
	configDir  string
	configFile string
}

// NewConfigStorage creates a new configuration storage handler
func NewConfigStorage() (*ConfigStorage, error) {
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

	return &ConfigStorage{
		configDir:  configDir,
		configFile: configFile,
	}, nil
}

// GetConfigPath returns the configuration file path
func (cs *ConfigStorage) GetConfigPath() string {
	return cs.configFile
}

// GetConfigDir returns the configuration directory path
func (cs *ConfigStorage) GetConfigDir() string {
	return cs.configDir
}

// Load loads configuration from file
func (cs *ConfigStorage) Load() (*Config, error) {
	config := &Config{
		DefaultProfile: "default",
		Profiles:       make(map[string]Profile),
		Preferences:    DefaultPreferences(),
	}

	viper.SetConfigFile(cs.configFile)
	viper.SetConfigType("yaml")

	// Set default values
	cs.setDefaults()

	// Read environment variables
	viper.SetEnvPrefix("AHASEND")
	viper.AutomaticEnv()

	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file doesn't exist, use defaults
	}

	// Unmarshal into config struct
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return config, nil
}

// Save saves configuration to file
func (cs *ConfigStorage) Save(config *Config) error {
	// Clear viper and set new values
	viper.Reset()
	cs.setDefaults()

	// Set values from config
	viper.Set("default_profile", config.DefaultProfile)
	viper.Set("profiles", config.Profiles)
	viper.Set("preferences", config.Preferences)

	viper.SetConfigFile(cs.configFile)
	viper.SetConfigType("yaml")

	return viper.WriteConfig()
}

// setDefaults sets default configuration values in viper
func (cs *ConfigStorage) setDefaults() {
	viper.SetDefault("default_profile", "default")
	viper.SetDefault("preferences.output_format", "table")
	viper.SetDefault("preferences.color_output", true)
	viper.SetDefault("preferences.webhook_timeout", "30s")
	viper.SetDefault("preferences.log_level", "info")
	viper.SetDefault("preferences.batch_concurrency", 5)
}
