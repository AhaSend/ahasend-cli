package config

import (
	"fmt"
)

// ProfileManager handles profile-specific operations
type ProfileManager struct {
	config *Config
}

// NewProfileManager creates a new profile manager
func NewProfileManager(config *Config) *ProfileManager {
	return &ProfileManager{
		config: config,
	}
}

// GetCurrentProfile returns the current default profile
func (pm *ProfileManager) GetCurrentProfile() (*Profile, error) {
	if pm.config.DefaultProfile == "" {
		return nil, fmt.Errorf("no default profile set")
	}

	profile, exists := pm.config.Profiles[pm.config.DefaultProfile]
	if !exists {
		return nil, fmt.Errorf("default profile '%s' not found", pm.config.DefaultProfile)
	}

	return &profile, nil
}

// SetProfile adds or updates a profile
func (pm *ProfileManager) SetProfile(name string, profile Profile) error {
	if pm.config.Profiles == nil {
		pm.config.Profiles = make(map[string]Profile)
	}
	pm.config.Profiles[name] = profile
	return nil
}

// RemoveProfile removes a profile (cannot remove default profile)
func (pm *ProfileManager) RemoveProfile(name string) error {
	if name == pm.config.DefaultProfile {
		return fmt.Errorf("cannot remove the default profile '%s'. Switch to another profile first", name)
	}
	delete(pm.config.Profiles, name)
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

// GetProfile returns a specific profile by name
func (pm *ProfileManager) GetProfile(name string) (*Profile, error) {
	profile, exists := pm.config.Profiles[name]
	if !exists {
		return nil, fmt.Errorf("profile '%s' not found", name)
	}
	return &profile, nil
}
