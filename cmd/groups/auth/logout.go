package auth

import (
	"fmt"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/config"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/spf13/cobra"
)

// NewLogoutCommand creates the logout command
func NewLogoutCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout [profile]",
		Short: "Log out and remove stored credentials",
		Long: `Remove stored API credentials for the current profile or a specific profile.
This will delete the profile from your local configuration. Name the profile
as the argument or with --profile; without either, the default profile logs
out.

--all removes every profile and its stored API key, and asks for confirmation
unless --force is set.`,
		Example: `  # Logout from current default profile
  ahasend auth logout

  # Logout from specific profile
  ahasend auth logout production

  # Logout from specific profile, named with --profile
  ahasend auth logout --profile production

  # Logout from all profiles
  ahasend auth logout --all

  # Logout from all profiles without the confirmation prompt
  ahasend auth logout --all --force`,
		Args:              cmdutil.MaximumNArgs(1),
		ValidArgsFunction: cmdutil.CompleteProfileArgument,
		RunE:              runLogout,
		SilenceUsage:      true,
	}

	cmd.Flags().Bool("all", false, "Logout from all profiles")
	prompt.AddForceFlag(cmd)

	return cmd
}

func runLogout(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	all, _ := cmd.Flags().GetBool("all")
	if err := checkLogoutTarget(cmd, args, all); err != nil {
		return err
	}
	argument := ""
	if len(args) > 0 {
		argument = args[0]
	}
	profileFlag, _ := cmd.Flags().GetString("profile")
	if profileFlag != "" {
		args = []string{profileFlag}
	}

	// Log logout operation
	logger.ConfigOperation("logout_start", "", map[string]interface{}{
		"all_profiles": all,
		"profile_arg":  len(args) > 0,
	})

	configMgr, err := config.LoadManager()
	if err != nil {
		return err
	}
	if argument != "" && profileFlag != "" && argument != profileFlag {
		if err := checkSameProfile(configMgr, argument, profileFlag); err != nil {
			return err
		}
	}

	// Without profiles, there is nothing to log out of unless one is named
	if len(configMgr.ListProfiles()) == 0 && len(args) == 0 {
		return handler.HandleAuthResult(printer.AuthResult{Message: printer.NoProfilesMessage})
	}
	if all {
		question := fmt.Sprintf("Are you sure you want to log out of all %d profiles and delete their stored API keys?", len(configMgr.ListProfiles()))
		if err := prompt.Confirm(cmd, question); err != nil {
			return err
		}
	}

	var result printer.AuthResult
	err = config.Update(cmd.ErrOrStderr(), func(configMgr *config.Manager) error {
		if all {
			result = logoutAll(configMgr)
			return nil
		}
		var err error
		result, err = logoutProfile(configMgr, args)
		return err
	})
	if err != nil {
		return err
	}
	return handler.HandleAuthResult(result)
}

// checkLogoutTarget checks that the command names at most one profile to log
// out: the argument and --profile do not name two profiles when both are
// given, and --all comes with neither. Two names that differ only in case
// are checked by checkSameProfile once the configuration is loaded.
func checkLogoutTarget(cmd *cobra.Command, args []string, all bool) error {
	profileSet := cmd.Flags().Changed("profile")
	profileFlag, _ := cmd.Flags().GetString("profile")
	switch {
	case all && len(args) > 0:
		return errors.NewValidationError("--all and a profile argument cannot be used together", nil)
	case all && profileSet:
		return errors.NewValidationError("--all and --profile cannot be used together", nil)
	case !all && cmd.Flags().Changed("force"):
		return errors.NewValidationError("--force needs --all", nil)
	case len(args) > 0 && profileSet && !strings.EqualFold(args[0], profileFlag):
		return twoProfilesError(args[0], profileFlag)
	}
	return nil
}

// checkSameProfile checks that argument and flag, the profile argument and
// --profile of a logout, which differ only in case, name the same profile.
func checkSameProfile(configMgr *config.Manager, argument, flag string) error {
	fromArgument, err := configMgr.ProfileName(argument)
	if err != nil {
		return err
	}
	fromFlag, err := configMgr.ProfileName(flag)
	if err != nil {
		return err
	}
	if fromArgument != fromFlag {
		return twoProfilesError(argument, flag)
	}
	return nil
}

// twoProfilesError is the error of a logout whose profile argument and
// --profile name two profiles.
func twoProfilesError(argument, flag string) error {
	return errors.NewValidationError(fmt.Sprintf("the profile argument %q and --profile %q cannot be used together", argument, flag), nil)
}

// logoutAll removes every profile and the default.
func logoutAll(configMgr *config.Manager) printer.AuthResult {
	profiles := configMgr.ListProfiles()
	logger.Get().WithField("profile_count", len(profiles)).Debug("Executing logout from all profiles")
	for _, profileName := range profiles {
		logger.Get().WithField("profile", profileName).Debug("Removing profile")
		delete(configMgr.GetConfig().Profiles, profileName)
	}
	configMgr.GetConfig().DefaultProfile = ""

	logger.ConfigOperation("logout_complete", "", map[string]interface{}{
		"operation": "all_profiles",
		"count":     len(profiles),
	})
	return printer.AuthResult{
		Message: fmt.Sprintf("Logged out from all profiles (%d removed)", len(profiles)),
	}
}

// logoutProfile removes the profile named in args, or the default profile.
// When the default goes and exactly one profile remains, that profile becomes
// the default; when several remain, none does, and the message says how to
// choose one.
func logoutProfile(configMgr *config.Manager, args []string) (printer.AuthResult, error) {
	profileName := configMgr.GetConfig().DefaultProfile
	if len(args) > 0 {
		profileName = args[0]
	} else if profileName == "" {
		return printer.AuthResult{}, errors.NewConfigError("no default profile set and no profile specified", nil)
	}
	logger.Get().WithField("profile", profileName).Debug("Logging out of the profile")

	profileName, err := configMgr.ProfileName(profileName)
	if err != nil {
		return printer.AuthResult{}, err
	}

	message := fmt.Sprintf("Logged out from profile '%s'", profileName)
	wasDefault := profileName == configMgr.GetConfig().DefaultProfile
	delete(configMgr.GetConfig().Profiles, profileName)
	if wasDefault {
		remaining := configMgr.ListProfiles()
		switch len(remaining) {
		case 0:
			configMgr.GetConfig().DefaultProfile = ""
		case 1:
			configMgr.GetConfig().DefaultProfile = remaining[0]
			message += fmt.Sprintf(". Profile '%s' is now the default", remaining[0])
		default:
			configMgr.GetConfig().DefaultProfile = ""
			message += ". No profile is the default now; run 'ahasend auth switch <profile>' to choose one"
		}
	}

	logger.ConfigOperation("logout_complete", profileName, map[string]interface{}{
		"operation":   "single_profile",
		"was_default": wasDefault,
	})
	return printer.AuthResult{Profile: profileName, Message: message}, nil
}
