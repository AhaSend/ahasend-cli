package config

import (
	"fmt"
)

// ProfileManager handles profile-specific operations
type ProfileManager struct {
	config *Config
}

// newProfileManager creates a new profile manager
func newProfileManager(config *Config) *ProfileManager {
	return &ProfileManager{
		config: config,
	}
}

// SetProfile adds or updates a profile
func (pm *ProfileManager) SetProfile(name string, profile Profile) error {
	if pm.config.Profiles == nil {
		pm.config.Profiles = make(map[string]Profile)
	}
	pm.config.Profiles[name] = profile
	return nil
}

// SetDefaultProfile sets the default profile
func (pm *ProfileManager) SetDefaultProfile(name string) error {
	if _, exists := pm.config.Profiles[name]; !exists {
		return fmt.Errorf("profile '%s' does not exist", name)
	}
	pm.config.DefaultProfile = name
	return nil
}

// ListProfiles returns a list of all profile names
func (pm *ProfileManager) ListProfiles() []string {
	profiles := make([]string, 0, len(pm.config.Profiles))
	for name := range pm.config.Profiles {
		profiles = append(profiles, name)
	}
	return profiles
}
