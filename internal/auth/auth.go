// Package auth provides authentication utilities for the AhaSend CLI.
//
// This package handles client authentication through multiple methods:
//   - Global API key flags (--api-key and --account-id)
//   - Profile-based authentication from configuration files
//   - Profile switching and validation
//
// The main function GetAuthenticatedClient creates authenticated clients
// for use by CLI commands, handling the authentication precedence and
// validation automatically.
package auth

import (
	"github.com/spf13/cobra"

	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/config"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
)

// GetAuthenticatedClient returns an authenticated AhaSend client
// It checks for global flags first, then falls back to profiles
func GetAuthenticatedClient(cmd *cobra.Command) (client.AhaSendClient, error) {
	// Check for global API key flags
	apiKey, _ := cmd.Flags().GetString("api-key")
	accountID, _ := cmd.Flags().GetString("account-id")
	profileName, _ := cmd.Flags().GetString("profile")

	if apiKey != "" {
		if accountID == "" {
			return nil, errors.NewValidationError("--account-id is required when using --api-key", nil)
		}
		logger.ConfigOperation("global_auth", "", map[string]interface{}{
			"method":     "api-key",
			"account_id": accountID,
		})
		return client.NewClient(apiKey, accountID)
	}

	// Fall back to profile-based authentication
	configMgr, err := config.NewManager()
	if err != nil {
		return nil, errors.NewConfigError("failed to initialize configuration", err)
	}

	if err := configMgr.Load(); err != nil {
		return nil, errors.NewConfigError("failed to load configuration", err)
	}

	// Use specified profile or default
	var profile *config.Profile
	if profileName != "" {
		profiles := configMgr.GetConfig().Profiles
		if p, exists := profiles[profileName]; exists {
			profile = &p
		} else {
			return nil, errors.NewNotFoundError("profile not found: "+profileName, nil)
		}
		logger.ConfigOperation("profile_auth", profileName, map[string]interface{}{
			"method": "specified_profile",
		})
	} else {
		var err error
		profile, err = configMgr.GetCurrentProfile()
		if err != nil {
			return nil, errors.NewAuthError("no default profile found. Run 'ahasend auth login' to authenticate", err)
		}
		logger.ConfigOperation("profile_auth", profile.Name, map[string]interface{}{
			"method": "default_profile",
		})
	}

	return client.NewClient(profile.APIKey, profile.AccountID)
}

// RequireAuth validates that authentication is available
func RequireAuth(cmd *cobra.Command) error {
	_, err := GetAuthenticatedClient(cmd)
	return err
}
