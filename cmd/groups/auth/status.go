package auth

import (
	"fmt"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/config"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
)

// NewStatusCommand creates the status command
func NewStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show authentication status and current profile information",
		Long: `Display information about the current authentication status, including:
- Current active profile
- API key validity
- Account information
- Available profiles`,
		Example: `  # Show current authentication status
  ahasend auth status

  # Show status for specific profile
  ahasend auth status --profile production

  # Show status for all profiles
  ahasend auth status --all`,
		RunE:         runStatus,
		SilenceUsage: true,
	}

	cmd.Flags().String("profile", "", "Show status for specific profile")
	cmd.Flags().Bool("all", false, "Show status for all profiles")

	return cmd
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	profileName, _ := cmd.Flags().GetString("profile")
	showAll, _ := cmd.Flags().GetBool("all")

	// Log status check
	logger.ConfigOperation("status_check", profileName, map[string]interface{}{
		"show_all":         showAll,
		"specific_profile": profileName != "",
	})

	configMgr, err := config.NewManager()
	if err != nil {
		return errors.NewConfigError("failed to initialize configuration", err)
	}

	if err := configMgr.Load(); err != nil {
		return errors.NewConfigError("failed to load configuration", err)
	}

	profiles := configMgr.ListProfiles()
	logger.Get().WithField("profile_count", len(profiles)).Debug("Checking authentication status")

	if len(profiles) == 0 {
		return handler.HandleEmpty("No authentication profiles found")
	}

	if showAll {
		logger.Debug("Showing status for all profiles")
		// For --all flag, we'll show a simple message since we don't have a specific multi-profile handler
		return handler.HandleSimpleSuccess(fmt.Sprintf("Found %d profiles: %v", len(profiles), profiles))
	}

	// Determine which profile to show
	if profileName == "" {
		profileName = configMgr.GetConfig().DefaultProfile
		if profileName == "" {
			return errors.NewValidationError("no default profile set and no profile specified", nil)
		}
		logger.Get().WithField("profile", profileName).Debug("Using default profile for status")
	} else {
		logger.Get().WithField("profile", profileName).Debug("Using specified profile for status")
	}

	// Create AuthStatus for the specific profile
	status, err := createAuthStatus(configMgr, profileName)
	if err != nil {
		return err
	}

	return handler.HandleAuthStatus(status, printer.AuthConfig{
		SuccessMessage: fmt.Sprintf("Authentication status for profile '%s'", profileName),
	})
}

// createAuthStatus creates an AuthStatus struct for the given profile
func createAuthStatus(configMgr *config.Manager, profileName string) (*printer.AuthStatus, error) {
	// Get the specific profile by name
	profiles := configMgr.GetConfig().Profiles
	profile, exists := profiles[profileName]
	if !exists {
		return nil, errors.NewNotFoundError(fmt.Sprintf("profile '%s' not found", profileName), nil)
	}

	// Create a copy of the profile to work with
	profileCopy := profile

	// Check if account info needs refreshing
	if shouldRefreshAccountInfo(&profileCopy) {
		logger.Get().WithField("profile", profileName).Debug("Refreshing account information")
		if err := refreshAccountInfo(configMgr, profileName, &profileCopy); err != nil {
			logger.Get().WithError(err).Debug("Failed to refresh account info, continuing with existing data")
		}
	}

	// Test if the credentials are valid
	testClient, err := client.NewClient(profile.APIKey, profile.AccountID)
	isValid := true
	var account *responses.Account

	if err != nil {
		logger.Get().WithError(err).Debug("Failed to create client for status check")
		isValid = false
	} else {
		if err := testClient.Ping(); err != nil {
			logger.Get().WithError(err).Debug("API key validation failed")
			isValid = false
		} else {
			// Try to get account info
			if acc, err := testClient.GetAccount(); err == nil {
				account = acc
			}
		}
	}

	// Create masked API key
	maskedAPIKey := fmt.Sprintf("%s...%s",
		profile.APIKey[:min(10, len(profile.APIKey))],
		profile.APIKey[max(0, len(profile.APIKey)-4):])

	return &printer.AuthStatus{
		Profile: profileName,
		APIKey:  maskedAPIKey,
		Account: account,
		Valid:   isValid,
	}, nil
}

// shouldRefreshAccountInfo checks if account information needs to be refreshed
func shouldRefreshAccountInfo(profile *config.Profile) bool {
	// Refresh if account name is not set
	if profile.AccountName == "" {
		return true
	}

	// Refresh if it's been more than 30 days since last update
	if profile.AccountUpdated.IsZero() {
		return true
	}

	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	return profile.AccountUpdated.Before(thirtyDaysAgo)
}

// refreshAccountInfo fetches fresh account information and updates the profile
func refreshAccountInfo(configMgr *config.Manager, profileName string, profile *config.Profile) error {
	client, err := client.NewClient(profile.APIKey, profile.AccountID)
	if err != nil {
		return err
	}

	account, err := client.GetAccount()
	if err != nil {
		logger.Get().WithError(err).Debug("Failed to refresh account information")
		return err
	}

	// Update the profile with fresh account info
	updatedProfile := *profile
	updatedProfile.AccountName = account.Name
	updatedProfile.AccountUpdated = time.Now()

	// Save the updated profile
	if err := configMgr.SetProfile(profileName, updatedProfile); err != nil {
		logger.Get().WithError(err).Debug("Failed to save updated profile")
		return err
	}

	*profile = updatedProfile
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
