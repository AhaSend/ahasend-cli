package auth

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/config"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// NewSwitchCommand creates the switch command
func NewSwitchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "switch <profile>",
		Short: "Switch to a different authentication profile",
		Long: `Switch the active authentication profile to use different AhaSend credentials.
This changes which API key and account will be used by default for all commands.`,
		Example: `  # Switch to production profile
  ahasend auth switch production

  # List available profiles to switch to
  ahasend auth status --all`,
		Args:         cobra.ExactArgs(1),
		RunE:         runSwitch,
		SilenceUsage: true,
	}

	return cmd
}

func runSwitch(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	profileName := args[0]

	// Log switch operation
	logger.ConfigOperation("switch_start", profileName, map[string]interface{}{
		"target_profile": profileName,
	})

	configMgr, err := config.NewManager()
	if err != nil {
		return handler.HandleError(errors.NewConfigError("failed to initialize configuration", err))
	}

	if err := configMgr.Load(); err != nil {
		return handler.HandleError(errors.NewConfigError("failed to load configuration", err))
	}

	// Check if profile exists
	profiles := configMgr.ListProfiles()
	logger.Get().WithFields(map[string]interface{}{
		"target_profile":     profileName,
		"available_profiles": len(profiles),
	}).Debug("Checking if profile exists")

	found := false
	for _, name := range profiles {
		if name == profileName {
			found = true
			break
		}
	}

	if !found {
		return handler.HandleError(errors.NewNotFoundError(fmt.Sprintf("profile '%s' not found", profileName), nil))
	}

	// Check if it's already the current profile
	if profileName == configMgr.GetConfig().DefaultProfile {
		logger.Get().WithField("profile", profileName).Debug("Profile is already current")
		return handler.HandleSimpleSuccess(fmt.Sprintf("Already using profile '%s'", profileName))
	}

	// Validate the profile by testing its API key
	profiles_map := configMgr.GetConfig().Profiles
	profile := profiles_map[profileName]

	logger.Get().WithFields(map[string]interface{}{
		"profile":    profileName,
		"account_id": profile.AccountID,
	}).Debug("Validating profile credentials")

	testClient, err := client.NewClient(profile.APIKey, profile.AccountID)
	if err != nil {
		return handler.HandleError(errors.NewAuthError("failed to create API client for profile", err))
	}

	if err := testClient.Ping(); err != nil {
		// Still allow the switch but note validation failed
		logger.Get().WithFields(map[string]interface{}{
			"profile": profileName,
			"error":   err.Error(),
		}).Debug("Profile validation failed but continuing with switch")
	} else {
		logger.Get().WithField("profile", profileName).Debug("Profile validation successful")
	}

	// Switch to the profile
	if err := configMgr.SetDefaultProfile(profileName); err != nil {
		return handler.HandleError(errors.NewConfigError("failed to switch profile", err))
	}

	logger.ConfigOperation("switch_complete", profileName, map[string]interface{}{
		"from_profile": configMgr.GetConfig().DefaultProfile,
		"to_profile":   profileName,
		"account_id":   profile.AccountID,
	})

	// Handle successful switch
	return handler.HandleAuthSwitch(profileName, printer.AuthConfig{
		SuccessMessage: fmt.Sprintf("Switched to profile '%s'", profileName),
	})
}
