package config

import (
	"fmt"
	"strconv"

	"github.com/AhaSend/ahasend-cli/internal/validation"
)

// PreferenceManager handles preference operations with validation
type PreferenceManager struct {
	config *Config
}

// NewPreferenceManager creates a new preference manager
func NewPreferenceManager(config *Config) *PreferenceManager {
	return &PreferenceManager{
		config: config,
	}
}

// SetPreference sets a preference value with validation
func (pm *PreferenceManager) SetPreference(key, value string) error {
	switch key {
	case "output_format":
		if err := validation.ValidateOutputFormat(value); err != nil {
			return err
		}
		pm.config.Preferences.OutputFormat = value

	case "color_output":
		if err := validation.ValidateBooleanString(value); err != nil {
			return err
		}
		pm.config.Preferences.ColorOutput = value == "true"

	case "webhook_timeout":
		// TODO: Add timeout validation (parse duration)
		pm.config.Preferences.WebhookTimeout = value

	case "log_level":
		if err := validation.ValidateLogLevel(value); err != nil {
			return err
		}
		pm.config.Preferences.LogLevel = value

	case "default_domain":
		if value != "" {
			if err := validation.ValidateDomainName(value); err != nil {
				return err
			}
		}
		pm.config.Preferences.DefaultDomain = value

	case "batch_concurrency":
		if err := validation.ValidateBatchConcurrency(value); err != nil {
			return err
		}
		concurrency, _ := strconv.Atoi(value) // Already validated above
		pm.config.Preferences.BatchConcurrency = concurrency

	default:
		return fmt.Errorf("unknown preference: %s", key)
	}

	return nil
}

// GetPreference gets a preference value
func (pm *PreferenceManager) GetPreference(key string) (string, error) {
	switch key {
	case "output_format":
		return pm.config.Preferences.OutputFormat, nil
	case "color_output":
		return strconv.FormatBool(pm.config.Preferences.ColorOutput), nil
	case "webhook_timeout":
		return pm.config.Preferences.WebhookTimeout, nil
	case "log_level":
		return pm.config.Preferences.LogLevel, nil
	case "default_domain":
		return pm.config.Preferences.DefaultDomain, nil
	case "batch_concurrency":
		return strconv.Itoa(pm.config.Preferences.BatchConcurrency), nil
	default:
		return "", fmt.Errorf("unknown preference: %s", key)
	}
}

// GetAllPreferences returns all preferences as a map
func (pm *PreferenceManager) GetAllPreferences() map[string]string {
	return map[string]string{
		"output_format":     pm.config.Preferences.OutputFormat,
		"color_output":      strconv.FormatBool(pm.config.Preferences.ColorOutput),
		"webhook_timeout":   pm.config.Preferences.WebhookTimeout,
		"log_level":         pm.config.Preferences.LogLevel,
		"default_domain":    pm.config.Preferences.DefaultDomain,
		"batch_concurrency": strconv.Itoa(pm.config.Preferences.BatchConcurrency),
	}
}
