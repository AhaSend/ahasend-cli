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
	"fmt"
	"sync"

	"github.com/spf13/cobra"

	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/config"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
)

// ClientResolver resolves an authenticated AhaSend client for a command.
type ClientResolver func(*cobra.Command) (client.AhaSendClient, error)

var (
	authenticatedClientResolverMu sync.RWMutex
	authenticatedClientResolver   ClientResolver = defaultAuthenticatedClientResolver
)

// GetAuthenticatedClient returns an authenticated AhaSend client
// It checks for global flags first, then falls back to profiles
func GetAuthenticatedClient(cmd *cobra.Command) (client.AhaSendClient, error) {
	authenticatedClientResolverMu.RLock()
	resolver := authenticatedClientResolver
	authenticatedClientResolverMu.RUnlock()

	return resolver(cmd)
}

// SetAuthenticatedClientResolverForTesting overrides authenticated client resolution for tests.
func SetAuthenticatedClientResolverForTesting(resolver ClientResolver) func() {
	authenticatedClientResolverMu.Lock()
	previous := authenticatedClientResolver
	authenticatedClientResolver = resolver
	authenticatedClientResolverMu.Unlock()

	return func() {
		authenticatedClientResolverMu.Lock()
		authenticatedClientResolver = previous
		authenticatedClientResolverMu.Unlock()
	}
}

func defaultAuthenticatedClientResolver(cmd *cobra.Command) (client.AhaSendClient, error) {
	// Check for global API key flags
	apiKey, _ := cmd.Flags().GetString("api-key")
	accountID, _ := cmd.Flags().GetString("account-id")
	profileName, _ := cmd.Flags().GetString("profile")

	// The root command made sure --api-key comes with --account-id
	if apiKey != "" {
		logger.ConfigOperation("global_auth", "", map[string]interface{}{
			"method":     "api-key",
			"account_id": accountID,
		})
		return client.NewClient(apiKey, accountID)
	}

	// Fall back to profile-based authentication
	configMgr, err := config.LoadManager()
	if err != nil {
		return nil, err
	}

	// Use specified profile or default
	name, profile, err := configMgr.ResolveProfile(profileName)
	if err != nil {
		return nil, err
	}
	method := "specified_profile"
	if profileName == "" {
		method = "default_profile"
	}
	logger.ConfigOperation("profile_auth", name, map[string]interface{}{
		"method": method,
	})

	WarnIfUnencryptedAPIKey(cmd, profile.APIURL)
	return client.NewClient(profile.APIKey, profile.AccountID, profile.APIURL)
}

// apiKeyWarnedAnnotation marks the root command of a run that warned that
// its API key is sent unencrypted.
const apiKeyWarnedAnnotation = "ahasend.api-key-warned"

// WarnIfUnencryptedAPIKey writes a warning to stderr, once per run, when
// apiURL, the API URL of a profile or of auth login, sends the API key over
// plain HTTP to a host that is not a loopback host.
func WarnIfUnencryptedAPIKey(cmd *cobra.Command, apiURL string) {
	root := cmd.Root()
	if !validation.SendsInClear(apiURL) || root.Annotations[apiKeyWarnedAnnotation] != "" {
		return
	}
	if root.Annotations == nil {
		root.Annotations = map[string]string{}
	}
	root.Annotations[apiKeyWarnedAnnotation] = "true"
	fmt.Fprintln(printer.DisplayWriter(cmd.ErrOrStderr()), validation.PlainHTTPWarning("the API URL", apiURL, "the API key"))
}
