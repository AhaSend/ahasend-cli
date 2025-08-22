package config

// ManagerV2 coordinates configuration operations using focused components
// This is the new refactored manager that replaces the monolithic Manager
type ManagerV2 struct {
	storage           *ConfigStorage
	profileManager    *ProfileManager
	preferenceManager *PreferenceManager
	config            *Config
}

// NewManagerV2 creates a new refactored configuration manager
func NewManagerV2() (*ManagerV2, error) {
	storage, err := NewConfigStorage()
	if err != nil {
		return nil, err
	}

	// Load configuration
	config, err := storage.Load()
	if err != nil {
		return nil, err
	}

	return &ManagerV2{
		storage:           storage,
		profileManager:    NewProfileManager(config),
		preferenceManager: NewPreferenceManager(config),
		config:            config,
	}, nil
}

// Load reloads configuration from file
func (m *ManagerV2) Load() error {
	config, err := m.storage.Load()
	if err != nil {
		return err
	}

	m.config = config
	m.profileManager = NewProfileManager(config)
	m.preferenceManager = NewPreferenceManager(config)
	return nil
}

// Save saves configuration to file
func (m *ManagerV2) Save() error {
	return m.storage.Save(m.config)
}

// GetConfig returns the current configuration
func (m *ManagerV2) GetConfig() *Config {
	return m.config
}

// === Profile Management (delegated to ProfileManager) ===

// GetCurrentProfile returns the current default profile
func (m *ManagerV2) GetCurrentProfile() (*Profile, error) {
	return m.profileManager.GetCurrentProfile()
}

// SetProfile adds or updates a profile
func (m *ManagerV2) SetProfile(name string, profile Profile) error {
	err := m.profileManager.SetProfile(name, profile)
	if err != nil {
		return err
	}
	return m.Save()
}

// RemoveProfile removes a profile
func (m *ManagerV2) RemoveProfile(name string) error {
	err := m.profileManager.RemoveProfile(name)
	if err != nil {
		return err
	}
	return m.Save()
}

// SetDefaultProfile sets the default profile
func (m *ManagerV2) SetDefaultProfile(name string) error {
	err := m.profileManager.SetDefaultProfile(name)
	if err != nil {
		return err
	}
	return m.Save()
}

// ListProfiles returns all profile names
func (m *ManagerV2) ListProfiles() []string {
	return m.profileManager.ListProfiles()
}

// GetProfile returns a specific profile by name
func (m *ManagerV2) GetProfile(name string) (*Profile, error) {
	return m.profileManager.GetProfile(name)
}

// === Preference Management (delegated to PreferenceManager) ===

// SetPreference sets a preference value with validation
func (m *ManagerV2) SetPreference(key, value string) error {
	err := m.preferenceManager.SetPreference(key, value)
	if err != nil {
		return err
	}
	return m.Save()
}

// GetPreference gets a preference value
func (m *ManagerV2) GetPreference(key string) (string, error) {
	return m.preferenceManager.GetPreference(key)
}

// GetAllPreferences returns all preferences as a map
func (m *ManagerV2) GetAllPreferences() map[string]string {
	return m.preferenceManager.GetAllPreferences()
}
