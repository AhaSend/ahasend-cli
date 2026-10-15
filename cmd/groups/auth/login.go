package auth

import (
	"fmt"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/config"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// defaultAPIURL is the API URL a profile uses unless login is given another.
const defaultAPIURL = "https://api.ahasend.com"

// NewLoginCommand creates the login command
func NewLoginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to AhaSend by providing an API key",
		Long: `Authenticate with AhaSend by providing your API key and account ID.
The command checks that the API key belongs to the account by reading the
account, and that the API accepts the key from this IP address, and saves
the credentials only when both hold.
The profile becomes the default when there is no default profile yet;
otherwise the default stays, and 'ahasend auth switch <profile>' changes it.

You can create API keys in your AhaSend dashboard at https://app.ahasend.com`,
		Example: `  # Interactive login
  ahasend auth login

  # Login with specific profile name
  ahasend auth login --profile production

  # Login with API key directly (not recommended for production)
  ahasend auth login --api-key your-api-key --account-id your-account-id`,
		Args:         cmdutil.NoArgs,
		RunE:         runLogin,
		SilenceUsage: true,
	}

	cmd.Flags().String("profile", "", "Profile name to save credentials under")
	_ = cmd.RegisterFlagCompletionFunc("profile", cmdutil.CompleteProfileNames)
	cmd.Flags().String("api-key", "", "AhaSend API key (not recommended, use interactive prompt)")
	cmd.Flags().String("account-id", "", "AhaSend Account ID")
	cmd.Flags().String("api-url", defaultAPIURL, "AhaSend API URL")

	return cmd
}

func runLogin(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	profileName, _ := cmd.Flags().GetString("profile")
	apiKey, _ := cmd.Flags().GetString("api-key")
	accountID, _ := cmd.Flags().GetString("account-id")
	apiURL, _ := cmd.Flags().GetString("api-url")

	// A configuration that cannot be loaded fails before the prompts
	configMgr, err := config.LoadManager()
	if err != nil {
		return err
	}

	// Use default profile name if not specified
	if profileName == "" {
		profileName = "default"
	}
	// A name several profiles match ignoring case fails before the prompts
	if _, err := configMgr.NewProfileName(profileName); err != nil {
		return err
	}

	// Default API URL
	if apiURL == "" {
		apiURL = defaultAPIURL
	}

	// Log login attempt
	logger.ConfigOperation("login_start", profileName, map[string]interface{}{
		"interactive": apiKey == "",
		"api_url":     apiURL,
	})

	if err := cmdutil.CheckCredentialFlags(cmd); err != nil {
		return err
	}
	if err := validation.CheckAPIURL(apiURL); err != nil {
		return errors.NewValidationError("--api-url "+err.Error(), nil)
	}
	auth.WarnIfUnencryptedAPIKey(cmd, apiURL)

	// Ask for the credentials no flag gave; a key that cannot be used fails
	// before the Account ID is asked for
	if apiKey == "" {
		apiKey, err = prompt.PromptSecret(cmd, "Enter your AhaSend API key: ")
		if err != nil {
			return err
		}
		// API keys hold no spaces: drop the ones a paste brings along
		apiKey = strings.TrimSpace(apiKey)
	}
	if apiKey == "" {
		return errors.NewValidationError("--api-key is required", nil)
	}
	if !validation.IsPrintableASCII(apiKey) {
		return errors.NewValidationError("the API key "+validation.APIKeyNotPrintable, nil)
	}

	if accountID == "" {
		accountID, err = prompt.Prompt(cmd, "Enter your AhaSend Account ID: ")
		if err != nil {
			return err
		}
	}

	// Validate inputs
	if accountID == "" {
		return errors.NewValidationError("--account-id is required", nil)
	}
	if err := validation.ValidateUUIDField("account ID", accountID); err != nil {
		return err
	}

	// Test the credentials
	testClient, err := client.NewClient(apiKey, accountID, apiURL)
	if err != nil {
		return errors.NewValidationError("failed to create API client", err)
	}

	if _, err := auth.CheckKey(testClient); err != nil {
		return err
	}

	// Save the profile
	profile := config.Profile{
		APIKey:    apiKey,
		APIURL:    apiURL,
		AccountID: accountID,
	}

	var defaultProfile string
	err = config.Update(cmd.ErrOrStderr(), func(configMgr *config.Manager) error {
		// A profile logged in again keeps its name, which may differ from
		// profileName in case, and the keys the CLI does not know
		stored, err := configMgr.NewProfileName(profileName)
		if err != nil {
			return err
		}
		if stored != "" {
			profileName = stored
			existing, _ := configMgr.GetProfile(stored)
			profile.Extra = existing.Extra
		}
		if err := configMgr.SetProfile(profileName, profile); err != nil {
			return errors.NewConfigError("failed to save profile", err)
		}
		// The profile becomes the default only when there is none yet
		defaultProfile = configMgr.GetConfig().DefaultProfile
		if _, err := configMgr.GetProfile(defaultProfile); err != nil {
			defaultProfile = profileName
			if err := configMgr.SetDefaultProfile(profileName); err != nil {
				return errors.NewConfigError("failed to set default profile", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Handle successful login
	message := fmt.Sprintf("Profile '%s' authenticated and saved successfully", profileName)
	if defaultProfile != profileName {
		message += fmt.Sprintf(". Profile '%s' is the default; run 'ahasend auth switch %s' to use this one", defaultProfile, profileName)
	}
	return handler.HandleAuthResult(printer.AuthResult{
		Profile: profileName,
		Message: message,
	})
}
