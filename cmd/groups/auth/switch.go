package auth

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
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
		Long: `Switch the active authentication profile to use different AhaSend
credentials. This changes which API key and account all commands use by
default. The command checks the profile's key against its API URL first;
when the check fails, it warns on stderr and switches anyway.`,
		Example: `  # Switch to production profile
  ahasend auth switch production

  # List available profiles to switch to
  ahasend auth status --all`,
		Args:              cmdutil.ExactArgs(1),
		ValidArgsFunction: cmdutil.CompleteProfileArgument,
		RunE:              runSwitch,
		SilenceUsage:      true,
	}

	return cmd
}

func runSwitch(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	profileName := args[0]
	if cmd.Flags().Changed("profile") {
		return errors.NewValidationError("--profile cannot be used with auth switch; give the profile as the argument", nil)
	}

	// Log switch operation
	logger.ConfigOperation("switch_start", profileName, map[string]interface{}{
		"target_profile": profileName,
	})

	configMgr, err := config.LoadManager()
	if err != nil {
		return err
	}

	profileName, profile, err := configMgr.ResolveProfile(profileName)
	if err != nil {
		return err
	}

	// Check if it's already the current profile
	previousProfile := configMgr.GetConfig().DefaultProfile
	if profileName == previousProfile {
		logger.Get().WithField("profile", profileName).Debug("Profile is already current")
		return handler.HandleAuthResult(printer.AuthResult{
			Profile: profileName,
			Message: fmt.Sprintf("Already using profile '%s'", profileName),
		})
	}

	// Validate the profile by testing its API key
	logger.Get().WithFields(map[string]interface{}{
		"profile":    profileName,
		"account_id": profile.AccountID,
	}).Debug("Validating profile credentials")

	auth.WarnIfUnencryptedAPIKey(cmd, profile.APIURL)
	testClient, err := client.NewClient(profile.APIKey, profile.AccountID, profile.APIURL)
	if err != nil {
		return errors.NewAuthError("failed to create API client for profile", err)
	}

	// A key that fails the check does not stop the switch: the key may be
	// replaced next, or the API may be out of reach for now.
	if _, err := auth.CheckKey(testClient); err != nil {
		fmt.Fprintf(printer.DisplayWriter(cmd.ErrOrStderr()), "Warning: checking the key of profile '%s' failed: %v%s\n",
			profileName, err, printer.ErrorHint(err))
	} else {
		logger.Get().WithField("profile", profileName).Debug("Profile validation successful")
	}

	// Switch to the profile
	err = config.Update(cmd.ErrOrStderr(), func(configMgr *config.Manager) error {
		if err := configMgr.SetDefaultProfile(profileName); err != nil {
			return errors.NewConfigError("failed to switch profile", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	logger.ConfigOperation("switch_complete", profileName, map[string]interface{}{
		"from_profile": previousProfile,
		"to_profile":   profileName,
		"account_id":   profile.AccountID,
	})

	// Handle successful switch
	return handler.HandleAuthResult(printer.AuthResult{
		Profile: profileName,
		Message: fmt.Sprintf("Switched to profile '%s'", profileName),
	})
}
