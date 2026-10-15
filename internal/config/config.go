// Package config provides configuration management for the AhaSend CLI.
//
// This package handles:
//
//   - Multi-profile authentication with API keys and account IDs
//   - Configuration file loading and saving (YAML format)
//   - Profile switching and management operations
//
// All configuration is stored in ~/.ahasend/config.yaml following standard
// CLI conventions.
package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Config represents the CLI configuration. Each level keeps the keys the CLI
// does not know in Extra, so saving the file never drops them; comments are
// not kept.
type Config struct {
	DefaultProfile string               `yaml:"default_profile"`
	Profiles       map[string]Profile   `yaml:"profiles"`
	Extra          map[string]yaml.Node `yaml:",inline"`
}

// Profile represents an AhaSend account profile
type Profile struct {
	APIKey    string               `yaml:"api_key"`
	APIURL    string               `yaml:"api_url"`
	AccountID string               `yaml:"account_id"`
	Extra     map[string]yaml.Node `yaml:",inline"`
}

// Manager handles configuration loading and saving
type Manager struct {
	configDir  string
	configFile string
	config     *Config

	profileManager *ProfileManager
}

// dirMode keeps the CLI's directory, whose files hold API keys, readable by
// its owner only.
const dirMode os.FileMode = 0o700

// EnsureDir creates the CLI's directory, ~/.ahasend, readable by its owner
// only, tightens an existing one that other users can read, and returns its
// path.
func EnsureDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	dir := filepath.Join(homeDir, ".ahasend")
	if err := ensureDir(dir); err != nil {
		return "", err
	}
	return dir, nil
}

// ensureDir creates dir readable by its owner only, or tightens it when it
// exists.
func ensureDir(dir string) error {
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("failed to create the %s directory: %w", dir, err)
	}
	return tighten(dir, dirMode)
}

// tighten sets path's mode to mode when group or other users have any
// permission on it.
func tighten(path string, mode os.FileMode) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to read the permissions of %s: %w", path, err)
	}
	if info.Mode().Perm()&0o077 == 0 {
		return nil
	}
	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf("failed to restrict the permissions of %s: %w", path, err)
	}
	return nil
}

// NewManager creates a new configuration manager
func NewManager() (*Manager, error) {
	configDir, err := EnsureDir()
	if err != nil {
		return nil, err
	}
	configFile := filepath.Join(configDir, "config.yaml")

	config := &Config{
		DefaultProfile: "default",
		Profiles:       make(map[string]Profile),
	}

	manager := &Manager{
		configDir:      configDir,
		configFile:     configFile,
		config:         config,
		profileManager: newProfileManager(config),
	}

	return manager, nil
}

// ProfileNames returns the names of the profiles in the configuration file,
// sorted, or none when there is no file or it cannot be read. It creates
// nothing and writes nothing, so shell completion can call it.
func ProfileNames() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	manager := &Manager{configFile: filepath.Join(homeDir, ".ahasend", "config.yaml")}
	if err := manager.Load(); err != nil || manager.config == nil {
		return nil
	}
	names := manager.ListProfiles()
	slices.Sort(names)
	return names
}

// LoadManager returns a Manager with the configuration file loaded, or the
// configuration error that stopped it. Commands that change the
// configuration use Update instead.
func LoadManager() (*Manager, error) {
	manager, err := NewManager()
	if err != nil {
		return nil, errors.NewConfigError("failed to initialize configuration", err)
	}
	if err := manager.Load(); err != nil {
		return nil, errors.NewConfigError("failed to load configuration", err)
	}
	return manager, nil
}

// lockFileName is the name of the file in the configuration directory that
// Update locks.
const lockFileName = "config.lock"

// lockWaitNotice is how long Update waits for the lock of the configuration
// directory before it says on stderr what it waits for; a test shortens it.
var lockWaitNotice = time.Second

// Update loads the configuration, calls change with it and saves it when
// change returns nil, all under the exclusive lock of the configuration
// directory, so two commands that change the configuration at once never
// lose each other's changes. When another command holds the lock for more
// than lockWaitNotice, Update says on stderr that it waits for the lock
// file. change's error is returned as it is.
func Update(stderr io.Writer, change func(*Manager) error) error {
	manager, err := NewManager()
	if err != nil {
		return errors.NewConfigError("failed to initialize configuration", err)
	}
	noticed := make(chan struct{})
	notice := time.AfterFunc(lockWaitNotice, func() {
		defer close(noticed)
		fmt.Fprintf(stderr, "Waiting for %s: another ahasend command is changing the configuration\n",
			filepath.Join(manager.configDir, lockFileName))
	})
	unlock, err := lockDir(manager.configDir)
	if !notice.Stop() {
		<-noticed
	}
	if err != nil {
		return errors.NewConfigError("failed to lock the configuration", err)
	}
	defer unlock()

	if err := manager.Load(); err != nil {
		return errors.NewConfigError("failed to load configuration", err)
	}
	if err := change(manager); err != nil {
		return err
	}
	if err := manager.Save(); err != nil {
		return errors.NewConfigError("failed to save configuration", err)
	}
	return nil
}

// Load loads the configuration file. A missing file leaves the defaults.
// Profile names are kept exactly as written; keys the CLI does not know are
// kept for Save. A default_profile that names a profile in another case, as
// v0.2.0, which matched names ignoring case, could save it, becomes the name
// of that profile, as ProfileName finds it, so every comparison with it is
// exact, and Save writes it so.
func (m *Manager) Load() error {
	data, err := os.ReadFile(m.configFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	config := &Config{DefaultProfile: "default"}
	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to parse config file %s: %w", m.configFile, err)
	}
	if config.Profiles == nil {
		config.Profiles = make(map[string]Profile)
	}

	m.config = config
	m.profileManager = newProfileManager(config)
	if stored, err := m.ProfileName(config.DefaultProfile); err == nil {
		config.DefaultProfile = stored
	}
	return nil
}

// Save writes the configuration file. The file holds API keys, so it is
// readable by its owner only, and so is its directory. The new content goes
// to a temporary file in the same directory that then replaces the file, so
// a crash never leaves a partly written configuration. When the file is a
// symlink, the file it points to is replaced, so the symlink stays.
func (m *Manager) Save() error {
	data, err := yaml.Marshal(m.config)
	if err != nil {
		return fmt.Errorf("failed to encode the configuration: %w", err)
	}
	if err := ensureDir(m.configDir); err != nil {
		return err
	}
	target := m.configFile
	if resolved, err := filepath.EvalSymlinks(m.configFile); err == nil {
		target = resolved
	}

	// CreateTemp creates the file readable by its owner only.
	tmp, err := os.CreateTemp(filepath.Dir(target), "config-*.yaml.tmp")
	if err != nil {
		return fmt.Errorf("failed to create a temporary configuration file: %w", err)
	}
	// Once the file is renamed there is nothing left to remove
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write %s: %w", tmp.Name(), err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write %s: %w", tmp.Name(), err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to write %s: %w", tmp.Name(), err)
	}
	if err := replaceFile(tmp.Name(), target); err != nil {
		return fmt.Errorf("failed to replace %s: %w", target, err)
	}
	return nil
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *Config {
	return m.config
}

// ProfileName returns the name of the profile that name refers to: the
// profile called name or, when there is none, the one profile whose name
// matches name ignoring case. v0.2.0 matched profile names ignoring case, so
// a file it saved with "Prod" was used with --profile prod. Without such a
// profile, the error is the configuration error "profile '<name>' not
// found", which lists the profiles that match name ignoring case when there
// are several.
func (m *Manager) ProfileName(name string) (string, error) {
	if _, exists := m.config.Profiles[name]; exists {
		return name, nil
	}
	var matches []string
	for stored := range m.config.Profiles {
		if strings.EqualFold(stored, name) {
			matches = append(matches, stored)
		}
	}
	switch len(matches) {
	case 0:
		return "", errors.NewConfigError(fmt.Sprintf("profile '%s' not found", name), nil)
	case 1:
		return matches[0], nil
	default:
		slices.Sort(matches)
		return "", errors.NewConfigError(fmt.Sprintf("profile '%s' not found; the profiles '%s' differ from it only in case, so give one exactly",
			name, strings.Join(matches, "', '")), nil)
	}
}

// NewProfileName returns the name of the profile that name refers to, as
// ProfileName finds it, or "" when there is none, so a profile of that name
// can be added. Several profiles that match name ignoring case are
// ProfileName's configuration error.
func (m *Manager) NewProfileName(name string) (string, error) {
	stored, err := m.ProfileName(name)
	if err == nil {
		return stored, nil
	}
	for stored := range m.config.Profiles {
		if strings.EqualFold(stored, name) {
			return "", err
		}
	}
	return "", nil
}

// GetProfile returns the profile that name refers to, as ProfileName finds
// it, or the error ProfileName returns.
func (m *Manager) GetProfile(name string) (Profile, error) {
	stored, err := m.ProfileName(name)
	if err != nil {
		return Profile{}, err
	}
	return m.config.Profiles[stored], nil
}

// ResolveProfile returns the name and the profile a command authenticates
// with: the profile that name refers to, as ProfileName finds it, or the
// default profile when name is empty.
// Without any profile, the error is the authentication error that asks to
// log in; a profile that does not exist, a missing default and a profile
// whose api_url is not an API URL, whose api_key is empty or whose account_id
// is not a UUID are configuration errors.
func (m *Manager) ResolveProfile(name string) (string, Profile, error) {
	if name == "" {
		if len(m.config.Profiles) == 0 {
			return "", Profile{}, errors.NewAuthError("no profiles found. Run 'ahasend auth login' to authenticate", nil)
		}
		name = m.config.DefaultProfile
		if name == "" {
			return "", Profile{}, errors.NewConfigError("no default profile set. Run 'ahasend auth switch <profile>' to choose one", nil)
		}
		stored, err := m.ProfileName(name)
		if err != nil {
			return "", Profile{}, errors.NewConfigError(fmt.Sprintf("default profile '%s' not found. Run 'ahasend auth switch <profile>' to choose one", name), nil)
		}
		name = stored
	}
	name, err := m.ProfileName(name)
	if err != nil {
		return "", Profile{}, err
	}
	profile := m.config.Profiles[name]
	if profile.APIURL != "" {
		if err := validation.CheckAPIURL(profile.APIURL); err != nil {
			return "", Profile{}, errors.NewConfigError(fmt.Sprintf("the api_url %q of profile '%s' %v", profile.APIURL, name, err), nil)
		}
	}
	if profile.APIKey == "" {
		return "", Profile{}, errors.NewConfigError(fmt.Sprintf("profile '%s' has no api_key. Run 'ahasend auth login --profile %s' to log in again", name, name), nil)
	}
	if !validation.IsPrintableASCII(profile.APIKey) {
		return "", Profile{}, errors.NewConfigError(fmt.Sprintf("the api_key of profile '%s' %s. Run 'ahasend auth login --profile %s' to log in again",
			name, validation.APIKeyNotPrintable, name), nil)
	}
	if _, err := uuid.Parse(profile.AccountID); err != nil {
		return "", Profile{}, errors.NewConfigError(fmt.Sprintf("the account_id %q of profile '%s' is not a UUID. Run 'ahasend auth login --profile %s' to log in again", profile.AccountID, name, name), nil)
	}
	return name, profile, nil
}

// SetProfile adds or updates a profile. Save writes the change.
func (m *Manager) SetProfile(name string, profile Profile) error {
	return m.profileManager.SetProfile(name, profile)
}

// SetDefaultProfile sets the default profile. Save writes the change.
func (m *Manager) SetDefaultProfile(name string) error {
	return m.profileManager.SetDefaultProfile(name)
}

// ListProfiles returns all profile names
func (m *Manager) ListProfiles() []string {
	return m.profileManager.ListProfiles()
}
