package auth

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/config"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// NewLogoutCommand creates the logout command
func NewLogoutCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout [profile]",
		Short: "Log out and remove stored credentials",
		Long: `Remove stored API credentials for the current profile or a specific profile.
This will delete the profile from your local configuration.`,
		Example: `  # Logout from current default profile
  ahasend auth logout

  # Logout from specific profile
  ahasend auth logout production

  # Logout from all profiles
  ahasend auth logout --all`,
		Args:         cobra.MaximumNArgs(1),
		RunE:         runLogout,
		SilenceUsage: true,
	}

	cmd.Flags().Bool("all", false, "Logout from all profiles")

	return cmd
}

func runLogout(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	all, _ := cmd.Flags().GetBool("all")

	// Log logout operation
	logger.ConfigOperation("logout_start", "", map[string]interface{}{
		"all_profiles": all,
		"profile_arg":  len(args) > 0,
	})

	configMgr, err := config.NewManager()
	if err != nil {
		return handler.HandleError(errors.NewConfigError("failed to initialize configuration", err))
	}

	if err := configMgr.Load(); err != nil {
		return handler.HandleError(errors.NewConfigError("failed to load configuration", err))
	}

	// Logout from all profiles
	if all {
		profiles := configMgr.ListProfiles()
		logger.Get().WithField("profile_count", len(profiles)).Debug("Executing logout from all profiles")

		if len(profiles) == 0 {
			return handler.HandleEmpty("No profiles found")
		}

		// Remove all profiles
		for _, profileName := range profiles {
			logger.Get().WithField("profile", profileName).Debug("Removing profile")
			// Can't use RemoveProfile as it prevents removing default profile
			delete(configMgr.GetConfig().Profiles, profileName)
		}

		// Reset default profile
		configMgr.GetConfig().DefaultProfile = ""

		if err := configMgr.Save(); err != nil {
			return handler.HandleError(errors.NewConfigError("failed to save configuration", err))
		}

		logger.ConfigOperation("logout_complete", "", map[string]interface{}{
			"operation": "all_profiles",
			"count":     len(profiles),
		})

		// Handle successful logout from all profiles
		return handler.HandleAuthLogout(true, printer.AuthConfig{
			SuccessMessage: fmt.Sprintf("Logged out from all profiles (%d removed)", len(profiles)),
		})
	}

	// Determine which profile to logout from
	var profileName string
	if len(args) > 0 {
		profileName = args[0]
		logger.Get().WithField("profile", profileName).Debug("Using specified profile for logout")
	} else {
		// Use current default profile
		profileName = configMgr.GetConfig().DefaultProfile
		if profileName == "" {
			return handler.HandleError(errors.NewValidationError("no default profile set and no profile specified", nil))
		}
		logger.Get().WithField("profile", profileName).Debug("Using default profile for logout")
	}

	// Check if profile exists
	profiles := configMgr.ListProfiles()
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

	// If this is the default profile, we need to handle it specially
	if profileName == configMgr.GetConfig().DefaultProfile {
		// Remove the profile manually since RemoveProfile prevents removing default
		delete(configMgr.GetConfig().Profiles, profileName)

		// Set new default profile if others exist
		remainingProfiles := configMgr.ListProfiles()
		if len(remainingProfiles) > 0 {
			newDefaultProfile := remainingProfiles[0]
			configMgr.GetConfig().DefaultProfile = newDefaultProfile
			// Note: Info about new default will be included in the success message
		} else {
			configMgr.GetConfig().DefaultProfile = ""
		}

		if err := configMgr.Save(); err != nil {
			return handler.HandleError(errors.NewConfigError("failed to save configuration", err))
		}
	} else {
		// Use RemoveProfile for non-default profiles
		if err := configMgr.RemoveProfile(profileName); err != nil {
			return handler.HandleError(errors.NewConfigError("failed to remove profile", err))
		}
	}

	logger.ConfigOperation("logout_complete", profileName, map[string]interface{}{
		"operation":   "single_profile",
		"was_default": profileName == configMgr.GetConfig().DefaultProfile,
	})

	// Handle successful logout
	return handler.HandleAuthLogout(true, printer.AuthConfig{
		SuccessMessage: fmt.Sprintf("Successfully logged out from profile '%s'", profileName),
	})
}
