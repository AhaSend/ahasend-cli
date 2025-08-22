package auth

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/config"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// NewLoginCommand creates the login command
func NewLoginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to AhaSend by providing an API key",
		Long: `Authenticate with AhaSend by providing your API key and account ID.
This command will validate your credentials and store them securely for future use.

You can create API keys in your AhaSend dashboard at https://app.ahasend.com`,
		Example: `  # Interactive login
  ahasend auth login

  # Login with specific profile name
  ahasend auth login --profile production

  # Login with API key directly (not recommended for production)
  ahasend auth login --api-key your-api-key --account-id your-account-id`,
		RunE:         runLogin,
		SilenceUsage: true,
	}

	cmd.Flags().String("profile", "", "Profile name to save credentials under")
	cmd.Flags().String("api-key", "", "AhaSend API key (not recommended, use interactive prompt)")
	cmd.Flags().String("account-id", "", "AhaSend Account ID")
	cmd.Flags().String("api-url", "https://api.ahasend.com", "AhaSend API URL")

	return cmd
}

func runLogin(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	profileName, _ := cmd.Flags().GetString("profile")
	apiKey, _ := cmd.Flags().GetString("api-key")
	accountID, _ := cmd.Flags().GetString("account-id")
	apiURL, _ := cmd.Flags().GetString("api-url")

	// Create configuration manager
	configMgr, err := config.NewManager()
	if err != nil {
		return handler.HandleError(errors.NewConfigError("failed to initialize configuration", err))
	}

	// Load existing configuration
	if err := configMgr.Load(); err != nil {
		return handler.HandleError(errors.NewConfigError("failed to load configuration", err))
	}

	// Use default profile name if not specified
	if profileName == "" {
		profileName = "default"
	}

	// Default API URL
	if apiURL == "" {
		apiURL = "https://api.ahasend.com"
	}

	// Log login attempt
	logger.ConfigOperation("login_start", profileName, map[string]interface{}{
		"interactive": apiKey == "",
		"api_url":     apiURL,
	})

	// Validate partial flag usage - if one credential flag is provided, both must be provided
	apiKeyProvided := cmd.Flags().Changed("api-key")
	accountIDProvided := cmd.Flags().Changed("account-id")

	if apiKeyProvided && !accountIDProvided {
		return handler.HandleError(errors.NewValidationError("account ID is required when API key is provided", nil))
	}
	if accountIDProvided && !apiKeyProvided {
		return handler.HandleError(errors.NewValidationError("API key is required when account ID is provided", nil))
	}

	// Interactive prompts if values not provided
	// Note: The handler will manage whether to show prompts based on format
	if apiKey == "" {
		// TODO: Handler should manage interactive mode, but for now keep prompts
		apiKey, err = promptAPIKey()
		if err != nil {
			return handler.HandleError(errors.NewValidationError("failed to read API key", err))
		}
	}

	if accountID == "" {
		// TODO: Handler should manage interactive mode, but for now keep prompts
		accountID, err = promptAccountID()
		if err != nil {
			return handler.HandleError(errors.NewValidationError("failed to read account ID", err))
		}
	}

	// Validate inputs
	if apiKey == "" {
		return handler.HandleError(errors.NewValidationError("API key is required", nil))
	}
	if accountID == "" {
		return handler.HandleError(errors.NewValidationError("account ID is required", nil))
	}

	// Test the credentials
	testClient, err := client.NewClient(apiKey, accountID)
	if err != nil {
		return handler.HandleError(errors.NewAuthError("failed to create API client", err))
	}

	if err := testClient.Ping(); err != nil {
		return handler.HandleError(err)
	}

	// Fetch account information
	var accountName string
	var accountUpdated time.Time

	account, err := testClient.GetAccount()
	if err != nil {
		// If we can't get account info, log the error but don't fail the login
		logger.Get().WithError(err).Warn("Failed to fetch account information, continuing with login")
		accountName = ""
		accountUpdated = time.Time{}
	} else {
		accountName = account.Name
		accountUpdated = time.Now()
	}

	// Save the profile
	profile := config.Profile{
		APIKey:         apiKey,
		APIURL:         apiURL,
		AccountID:      accountID,
		Name:           fmt.Sprintf("AhaSend %s", profileName),
		AccountName:    accountName,
		AccountUpdated: accountUpdated,
	}

	if err := configMgr.SetProfile(profileName, profile); err != nil {
		return handler.HandleError(errors.NewConfigError("failed to save profile", err))
	}

	// Set as default profile if it's the first one or explicitly requested
	currentProfiles := configMgr.ListProfiles()
	if len(currentProfiles) == 1 || profileName == "default" {
		if err := configMgr.SetDefaultProfile(profileName); err != nil {
			return handler.HandleError(errors.NewConfigError("failed to set default profile", err))
		}
	}

	// Handle successful login
	return handler.HandleAuthLogin(true, profileName, printer.AuthConfig{
		SuccessMessage: fmt.Sprintf("Successfully authenticated and saved profile '%s'", profileName),
	})
}

func promptAPIKey() (string, error) {
	fmt.Print("Enter your AhaSend API key: ")

	// Hide input for API key
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", err
	}
	fmt.Println() // Add newline after hidden input

	return strings.TrimSpace(string(bytePassword)), nil
}

func promptAccountID() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter your AhaSend Account ID: ")

	accountID, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(accountID), nil
}
