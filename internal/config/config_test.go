package config

import (
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestNewManager(t *testing.T) {
	tests := []struct {
		name      string
		setupHome func() string
		wantErr   bool
	}{
		{
			name: "valid home directory",
			setupHome: func() string {
				tmpDir, _ := os.MkdirTemp("", "ahasend-test-*")
				return tmpDir
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupHome != nil {
				homeDir := tt.setupHome()
				defer os.RemoveAll(homeDir)
				testenv.SetHome(t, homeDir)
			}

			mgr, err := NewManager()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, mgr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, mgr)
			}
		})
	}
}

func TestManager_SetAndGetProfile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ahasend-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testenv.SetHome(t, tmpDir)

	mgr, err := NewManager()
	require.NoError(t, err)

	profile := Profile{
		APIKey:    "test-key",
		APIURL:    "https://api.ahasend.com",
		AccountID: "test-account",
	}

	// Test setting profile
	err = mgr.SetProfile("test", profile)
	assert.NoError(t, err)

	// Test getting profile
	retrievedProfile, err := mgr.GetProfile("test")
	require.NoError(t, err)
	assert.Equal(t, profile, retrievedProfile)

	// Test getting non-existent profile
	_, err = mgr.GetProfile("nonexistent")
	require.Error(t, err)
	assert.Equal(t, "profile 'nonexistent' not found", err.Error())
	assert.Equal(t, 3, errors.GetExitCode(err))
}

func TestManager_ListProfiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ahasend-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testenv.SetHome(t, tmpDir)

	mgr, err := NewManager()
	require.NoError(t, err)

	// Initially empty
	profiles := mgr.ListProfiles()
	assert.Empty(t, profiles)

	// Add profiles
	profile1 := Profile{APIKey: "key1", AccountID: "account1"}
	profile2 := Profile{APIKey: "key2", AccountID: "account2"}

	err = mgr.SetProfile("profile1", profile1)
	require.NoError(t, err)
	err = mgr.SetProfile("profile2", profile2)
	require.NoError(t, err)

	profiles = mgr.ListProfiles()
	assert.Len(t, profiles, 2)
	assert.Contains(t, profiles, "profile1")
	assert.Contains(t, profiles, "profile2")
}

func TestManager_DefaultProfile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ahasend-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testenv.SetHome(t, tmpDir)

	mgr, err := NewManager()
	require.NoError(t, err)

	// Default profile should be "default" initially
	defaultProfile := mgr.GetConfig().DefaultProfile
	assert.Equal(t, "default", defaultProfile)

	// Set default
	err = mgr.SetProfile("test", Profile{APIKey: "test"})
	require.NoError(t, err)
	err = mgr.SetDefaultProfile("test")
	assert.NoError(t, err)

	defaultProfile = mgr.GetConfig().DefaultProfile
	assert.Equal(t, "test", defaultProfile)
}

func TestManager_LoadAndSave(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ahasend-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testenv.SetHome(t, tmpDir)

	// Create first manager and save data
	mgr1, err := NewManager()
	require.NoError(t, err)

	profile := Profile{
		APIKey:    "test-key",
		APIURL:    "https://api.ahasend.com",
		AccountID: "test-account",
	}

	err = mgr1.SetProfile("test", profile)
	require.NoError(t, err)
	err = mgr1.SetDefaultProfile("test")
	require.NoError(t, err)
	err = mgr1.Save()
	require.NoError(t, err)

	// Create second manager and load data
	mgr2, err := NewManager()
	require.NoError(t, err)
	err = mgr2.Load()
	require.NoError(t, err)

	// Verify data was loaded correctly
	retrieved := mgr2.GetConfig().Profiles["test"]
	assert.Equal(t, profile, retrieved)

	defaultProfile := mgr2.GetConfig().DefaultProfile
	assert.Equal(t, "test", defaultProfile)
}

func TestManager_SaveCreatesDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ahasend-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testenv.SetHome(t, tmpDir)

	// NewManager creates the directory during initialization
	mgr, err := NewManager()
	require.NoError(t, err)

	// Verify directory was created by NewManager
	configDir := filepath.Join(tmpDir, ".ahasend")
	_, err = os.Stat(configDir)
	assert.NoError(t, err)

	// Save should work with existing directory
	err = mgr.Save()
	assert.NoError(t, err)

	// Verify config file was created
	configFile := filepath.Join(configDir, "config.yaml")
	_, err = os.Stat(configFile)
	assert.NoError(t, err)
}

func TestManager_SaveCreatesTheDirectoryAndFileReadableByTheOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows files have no POSIX modes")
	}
	home := t.TempDir()
	testenv.SetHome(t, home)

	mgr, err := NewManager()
	require.NoError(t, err)
	require.NoError(t, mgr.Save())

	dirInfo, err := os.Stat(filepath.Join(home, ".ahasend"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())
	fileInfo, err := os.Stat(filepath.Join(home, ".ahasend", "config.yaml"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm())
}

func TestManager_SaveTightensAnExistingDirectoryAndFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows files have no POSIX modes")
	}
	home := t.TempDir()
	testenv.SetHome(t, home)
	configDir := filepath.Join(home, ".ahasend")
	require.NoError(t, os.Mkdir(configDir, 0o755))
	configFile := filepath.Join(configDir, "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte("default_profile: default\n"), 0o644))
	// Mkdir and WriteFile apply the umask; set the loose modes exactly.
	require.NoError(t, os.Chmod(configDir, 0o755))
	require.NoError(t, os.Chmod(configFile, 0o644))

	mgr, err := NewManager()
	require.NoError(t, err)
	require.NoError(t, mgr.Load())
	require.NoError(t, mgr.SetProfile("default", Profile{APIKey: "aha-sk-test"}))
	require.NoError(t, mgr.Save())

	dirInfo, err := os.Stat(configDir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())
	fileInfo, err := os.Stat(configFile)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm())
}

// TestLoadReadsConfigFilesOfEarlierVersions checks that a config file an
// earlier version wrote still loads: lowercased profile names, profile keys
// and top-level keys the CLI no longer uses.
func TestLoadReadsConfigFilesOfEarlierVersions(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".ahasend"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".ahasend", "config.yaml"), []byte(`default_profile: work
legacy_setting: true
profiles:
  work:
    account_id: 11111111-1111-4111-8111-111111111111
    account_name: Acme
    account_updated: 2025-01-02T03:04:05Z
    api_key: aha-sk-work
    api_url: https://api.ahasend.com
    name: AhaSend work
preferences:
  output_format: json
`), 0o600))

	mgr, err := NewManager()
	require.NoError(t, err)
	require.NoError(t, mgr.Load())

	assert.Equal(t, "work", mgr.GetConfig().DefaultProfile)
	require.Equal(t, []string{"work"}, mgr.ListProfiles())
	work := mgr.GetConfig().Profiles["work"]
	assert.Equal(t, "aha-sk-work", work.APIKey)
	assert.Equal(t, "https://api.ahasend.com", work.APIURL)
	assert.Equal(t, "11111111-1111-4111-8111-111111111111", work.AccountID)
	assert.Contains(t, work.Extra, "name", "the name v0.2.0 saved is kept")
	assert.Contains(t, mgr.GetConfig().Extra, "preferences", "a block the CLI does not read is kept")
}

// loadProfiles loads a configuration file holding a profile for each of names,
// the first one the default, as v0.2.0 saved them: with the case the user
// typed.
func loadProfiles(t *testing.T, names ...string) *Manager {
	t.Helper()
	home := t.TempDir()
	testenv.SetHome(t, home)
	content := "default_profile: " + names[0] + "\nprofiles:\n"
	for _, name := range names {
		content += fmt.Sprintf("  %s:\n    account_id: 11111111-1111-4111-8111-111111111111\n    api_key: aha-sk-%s\n    api_url: http://127.0.0.1:1\n    name: AhaSend %s\n",
			name, name, name)
	}
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".ahasend"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".ahasend", "config.yaml"), []byte(content), 0o600))
	mgr, err := NewManager()
	require.NoError(t, err)
	require.NoError(t, mgr.Load())
	return mgr
}

// TestResolveProfileFindsAProfileNamedInAnotherCase checks that --profile
// prod finds the profile v0.2.0 saved as Prod, which v0.2.0 matched
// ignoring case.
func TestResolveProfileFindsAProfileNamedInAnotherCase(t *testing.T) {
	mgr := loadProfiles(t, "staging", "Prod")

	name, profile, err := mgr.ResolveProfile("prod")

	require.NoError(t, err)
	assert.Equal(t, "Prod", name)
	assert.Equal(t, "aha-sk-Prod", profile.APIKey)
}

func TestResolveProfileFindsADefaultNamedInAnotherCase(t *testing.T) {
	mgr := loadProfiles(t, "Prod")
	mgr.GetConfig().DefaultProfile = "prod"

	name, _, err := mgr.ResolveProfile("")

	require.NoError(t, err)
	assert.Equal(t, "Prod", name)
}

// TestLoadNamesTheDefaultAsItsProfileIsNamed checks that a default_profile
// in another case than its profile, as v0.2.0 could save it, is the
// profile's own name once loaded, and that Save writes it so; a default of
// no profile, or of several, stays as written.
func TestLoadNamesTheDefaultAsItsProfileIsNamed(t *testing.T) {
	mgr := loadProfiles(t, "staging", "prod")
	mgr.GetConfig().DefaultProfile = "Prod"
	require.NoError(t, mgr.Save())

	require.NoError(t, mgr.Load())
	assert.Equal(t, "prod", mgr.GetConfig().DefaultProfile)
	require.NoError(t, mgr.Save())
	data, err := os.ReadFile(mgr.configFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "default_profile: prod\n")

	for _, name := range []string{"missing", "PROD"} {
		mgr := loadProfiles(t, "Prod", "prod")
		mgr.GetConfig().DefaultProfile = name
		require.NoError(t, mgr.Save())
		require.NoError(t, mgr.Load())
		assert.Equal(t, name, mgr.GetConfig().DefaultProfile)
	}
}

func TestResolveProfilePrefersTheProfileOfTheExactName(t *testing.T) {
	mgr := loadProfiles(t, "Prod", "prod")

	name, profile, err := mgr.ResolveProfile("prod")

	require.NoError(t, err)
	assert.Equal(t, "prod", name)
	assert.Equal(t, "aha-sk-prod", profile.APIKey)
}

// TestResolveProfileRefusesANameSeveralProfilesMatchIgnoringCase checks that
// a name several profiles match only ignoring case is not found, and the
// error lists them.
func TestResolveProfileRefusesANameSeveralProfilesMatchIgnoringCase(t *testing.T) {
	mgr := loadProfiles(t, "Prod", "PROD")

	_, _, err := mgr.ResolveProfile("prod")

	require.Error(t, err)
	assert.Equal(t, "profile 'prod' not found; the profiles 'PROD', 'Prod' differ from it only in case, so give one exactly", err.Error())
	assert.Equal(t, 3, errors.GetExitCode(err))
}

// TestResolveProfileRefusesAKeyThatIsNotPrintableASCII checks that a
// profile whose api_key holds a control or non-ASCII character, such as one
// an arrow key put there, is a configuration error, before any request.
func TestResolveProfileRefusesAKeyThatIsNotPrintableASCII(t *testing.T) {
	mgr := loadProfiles(t, "work")
	profile := mgr.GetConfig().Profiles["work"]
	profile.APIKey = "aha-sk-\x1b[Dkey"
	mgr.GetConfig().Profiles["work"] = profile

	_, _, err := mgr.ResolveProfile("work")

	require.Error(t, err)
	assert.Equal(t, "the api_key of profile 'work' must be printable ASCII, without control or non-ASCII characters. "+
		"Run 'ahasend auth login --profile work' to log in again", err.Error())
	assert.Equal(t, 3, errors.GetExitCode(err))
}

// TestSaveKeepsKeysTheCLIDoesNotKnow checks that saving the configuration
// keeps the top-level and profile keys the CLI does not know, and the blocks
// it does not read, with their values.
func TestSaveKeepsKeysTheCLIDoesNotKnow(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	path := filepath.Join(home, ".ahasend", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(`default_profile: work
legacy_setting: true
plugins:
  - name: one
    enabled: false
profiles:
  work:
    account_id: 11111111-1111-4111-8111-111111111111
    account_name: Acme
    api_key: aha-sk-work
    api_url: https://api.ahasend.com
    name: work
preferences:
  output_format: json
  pager: less
`), 0o600))

	require.NoError(t, Update(io.Discard, func(mgr *Manager) error {
		return mgr.SetProfile("personal", Profile{APIKey: "aha-sk-personal"})
	}))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var saved map[string]interface{}
	require.NoError(t, yaml.Unmarshal(data, &saved))
	assert.Equal(t, true, saved["legacy_setting"])
	assert.Equal(t, []interface{}{map[string]interface{}{"name": "one", "enabled": false}}, saved["plugins"])
	profiles := saved["profiles"].(map[string]interface{})
	assert.Equal(t, "Acme", profiles["work"].(map[string]interface{})["account_name"])
	assert.Contains(t, profiles, "personal")
	assert.Equal(t, "less", saved["preferences"].(map[string]interface{})["pager"])
}

// TestANewConfigurationFileHoldsOnlyTheProfiles checks that a file the CLI
// creates holds the default profile and the profiles, and no block no
// command reads.
func TestANewConfigurationFileHoldsOnlyTheProfiles(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	require.NoError(t, Update(io.Discard, func(mgr *Manager) error {
		return mgr.SetProfile("work", Profile{APIKey: "aha-sk-work"})
	}))

	data, err := os.ReadFile(filepath.Join(home, ".ahasend", "config.yaml"))
	require.NoError(t, err)
	var saved map[string]interface{}
	require.NoError(t, yaml.Unmarshal(data, &saved))
	assert.ElementsMatch(t, []string{"default_profile", "profiles"}, slices.Collect(maps.Keys(saved)))
}

// TestProfileNamesRoundTripExactly checks that profile names with capitals
// and dots are saved and loaded unchanged.
func TestProfileNamesRoundTripExactly(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	names := []string{"Work", "prod.eu", "a.b.C"}
	require.NoError(t, Update(io.Discard, func(mgr *Manager) error {
		for _, name := range names {
			require.NoError(t, mgr.SetProfile(name, Profile{APIKey: "aha-sk-" + name}))
		}
		return mgr.SetDefaultProfile("prod.eu")
	}))
	// A second save keeps them too
	require.NoError(t, Update(io.Discard, func(*Manager) error { return nil }))

	mgr, err := LoadManager()
	require.NoError(t, err)
	assert.ElementsMatch(t, names, mgr.ListProfiles())
	assert.Equal(t, "prod.eu", mgr.GetConfig().DefaultProfile)
	assert.Equal(t, "aha-sk-a.b.C", mgr.GetConfig().Profiles["a.b.C"].APIKey)
}

// TestConcurrentUpdatesKeepEveryChange checks that commands saving
// different profiles at the same time never lose one another's profile.
func TestConcurrentUpdatesKeepEveryChange(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	const count = 20

	var wg sync.WaitGroup
	for i := range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, Update(io.Discard, func(mgr *Manager) error {
				return mgr.SetProfile(fmt.Sprintf("profile-%d", i), Profile{APIKey: "aha-sk"})
			}))
		}()
	}
	wg.Wait()

	mgr, err := LoadManager()
	require.NoError(t, err)
	assert.Len(t, mgr.ListProfiles(), count)
}

// TestSaveLeavesNoTemporaryFiles checks that saving replaces the config
// file and leaves only it and the lock file in the directory.
func TestSaveLeavesNoTemporaryFiles(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)

	for range 2 {
		require.NoError(t, Update(io.Discard, func(mgr *Manager) error {
			return mgr.SetProfile("work", Profile{APIKey: "aha-sk"})
		}))
	}

	entries, err := os.ReadDir(filepath.Join(home, ".ahasend"))
	require.NoError(t, err)
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	assert.ElementsMatch(t, []string{"config.lock", "config.yaml"}, names)
}

// TestSaveKeepsASymlinkedConfigFile checks that saving a config file that is
// a symlink writes the file it points to and keeps the symlink.
func TestSaveKeepsASymlinkedConfigFile(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	dotfiles := t.TempDir()
	target := filepath.Join(dotfiles, "ahasend.yaml")
	require.NoError(t, os.WriteFile(target, []byte("default_profile: personal\nprofiles: {}\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".ahasend"), 0o700))
	link := filepath.Join(home, ".ahasend", "config.yaml")
	require.NoError(t, os.Symlink(target, link))

	require.NoError(t, Update(io.Discard, func(mgr *Manager) error {
		return mgr.SetProfile("work", Profile{APIKey: "aha-sk-work"})
	}))

	info, err := os.Lstat(link)
	require.NoError(t, err)
	assert.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink, "the config file stays a symlink")
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Contains(t, string(data), "aha-sk-work")
	entries, err := os.ReadDir(dotfiles)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temporary file is left next to the target")
}
