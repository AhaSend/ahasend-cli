package auth

import (
	"fmt"
	"slices"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/config"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go"
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
- Available profiles, with --all

The key is checked by reading the account. Valid is Yes when the API accepts
the key for the profile's account, No when it refuses it (HTTP 401), as it
does a key of another account, or refuses the key from this IP address
(HTTP 403), and empty (null in JSON output) when the check
could not tell, such as when the API cannot be reached. Error says why the
check failed, and the command exits with the code of that failure. A key
without the scope to read the account is valid; Note says so.`,
		Example: `  # Show current authentication status
  ahasend auth status

  # Show status for specific profile
  ahasend auth status --profile production

  # List the names of all profiles
  ahasend auth status --all`,
		Args:         cmdutil.NoArgs,
		RunE:         runStatus,
		SilenceUsage: true,
	}

	cmd.Flags().String("profile", "", "Show status for specific profile")
	_ = cmd.RegisterFlagCompletionFunc("profile", cmdutil.CompleteProfileNames)
	cmd.Flags().Bool("all", false, "List the names of all profiles")

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

	configMgr, err := config.LoadManager()
	if err != nil {
		return err
	}

	profiles := configMgr.ListProfiles()
	logger.Get().WithField("profile_count", len(profiles)).Debug("Checking authentication status")

	if showAll {
		logger.Debug("Listing all profiles")
		names := slices.Clone(profiles)
		slices.Sort(names)
		return handler.HandleAuthProfiles(names)
	}

	if len(profiles) == 0 && profileName == "" {
		message := printer.NoProfilesMessage
		if handler.GetFormat() != "json" {
			// The message of JSON output stays the one sentence
			message += "\nRun 'ahasend auth login' to add one."
		}
		return handler.HandleEmpty(message)
	}

	// Determine which profile to show
	profileName, profile, err := configMgr.ResolveProfile(profileName)
	if err != nil {
		return err
	}
	logger.Get().WithField("profile", profileName).Debug("Showing the status of the profile")

	// Create AuthStatus for the specific profile
	auth.WarnIfUnencryptedAPIKey(cmd, profile.APIURL)
	status, checkErr := createAuthStatus(profileName, profile)

	if err := handler.HandleAuthStatus(status); err != nil {
		return err
	}

	// The status shows that the check failed; the exit code says why, such
	// as 2 when the API rejected the credentials. JSON output holds the
	// failure in its error field; other formats print it as the error line,
	// with what to do about it.
	if checkErr != nil {
		message := fmt.Sprintf("credentials of profile '%s' could not be verified", profileName)
		if handler.GetFormat() == "json" {
			return errors.NewResultPrintedError(fmt.Sprintf("%s: %v", message, checkErr), errors.GetExitCode(checkErr))
		}
		return fmt.Errorf("%s: %w", message, checkErr)
	}
	return nil
}

// createAuthStatus checks the credentials of the named profile with the API
// and fetches its account. It always returns the status; the error is why
// the check failed, if it did. The credentials are invalid only when the API
// refused them; any other failure, such as an API that cannot be reached,
// leaves their validity unknown.
func createAuthStatus(profileName string, profile config.Profile) (*printer.AuthStatus, error) {
	status := &printer.AuthStatus{
		Profile:   profileName,
		AccountID: profile.AccountID,
	}

	// Test if the credentials are valid
	testClient, err := client.NewClient(profile.APIKey, profile.AccountID, profile.APIURL)
	if err != nil {
		logger.Get().WithError(err).Debug("Failed to create client for status check")
		err = errors.NewConfigError("invalid profile", err)
		status.Error = err.Error()
		return status, err
	}
	check, err := auth.CheckKey(testClient)
	if err != nil {
		logger.Get().WithError(err).Debug("API key validation failed")
		if auth.IsKeyRefused(err) {
			status.Valid = ahasend.Bool(false)
		}
		status.Error = err.Error()
		return status, err
	}

	status.Valid = ahasend.Bool(true)
	status.Account = check.Account
	if check.Unreadable != nil {
		status.Note = "the API key cannot read the account details: " + check.Unreadable.Error()
	}
	return status, nil
}
