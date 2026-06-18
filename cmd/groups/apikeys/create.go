package apikeys

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewCreateCommand creates the apikeys create command
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new API key",
		Long: `Create a new API key with specified scopes and label.

API keys provide programmatic access to the AhaSend API. Each key has:
- A unique identifier and secret for authentication
- Configurable scopes that define what actions the key can perform
- An optional label for identification and organization

Choose scopes carefully based on your application's needs. Follow the
principle of least privilege by granting only the minimum required scopes.

The secret will be displayed once after creation and cannot be retrieved again.
Store it securely immediately after creation.

Available Scopes:
  Messages: messages:send:all, messages:cancel:all, messages:read:all
  Domains: domains:read, domains:write, domains:delete:all
  Accounts: accounts:read, accounts:write, accounts:billing
  Webhooks: webhooks:read:all, webhooks:write:all, webhooks:delete:all
  Routes: routes:read:all, routes:write:all, routes:delete:all
  Suppressions: suppressions:read, suppressions:write, suppressions:delete, suppressions:wipe
  SMTP: smtp-credentials:read:all, smtp-credentials:write:all, smtp-credentials:delete:all
  Statistics: statistics-transactional:read:all
  API Keys: api-keys:read, api-keys:write, api-keys:delete
  Sub-Accounts: sub-accounts:read, sub-accounts:write, sub-accounts:delete, sub-accounts:suspend, sub-accounts:usage
  Sub-Account API Keys: sub-account-api-keys:read, sub-account-api-keys:write, sub-account-api-keys:delete

Domain-restricted scopes can be created by appending {domain} to certain prefixes:
  messages:send:{example.com}, webhooks:read:{example.com}, etc.

Note: The domain must be verified and exist in your account for domain-restricted scopes to work.`,
		Example: `  # Create with specific scopes and label
  ahasend apikeys create \
    --label "Production API" \
    --scope messages:send:all \
    --scope domains:read

  # Create a read-only key for analytics
  ahasend apikeys create \
    --label "Analytics Dashboard" \
    --scope statistics-transactional:read:all \
    --scope messages:read:all

  # Create a domain-restricted key
  ahasend apikeys create \
    --label "Domain-specific API" \
    --scope messages:send:{example.com} \
    --scope webhooks:read:{example.com}`,
		RunE: runAPIKeyCreate,
	}

	// Configuration flags
	cmd.Flags().String("label", "", "Label for the API key (required)")
	cmd.Flags().StringSlice("scope", []string{}, "Scopes to grant (required, can be used multiple times)")

	// Mark required flags
	cmd.MarkFlagRequired("label")
	cmd.MarkFlagRequired("scope")

	return cmd
}

func runAPIKeyCreate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Get flag values
	label, _ := cmd.Flags().GetString("label")
	scopes, _ := cmd.Flags().GetStringSlice("scope")

	// Validate all scopes before making the API call
	for _, scope := range scopes {
		if err := validation.ValidateScope(scope); err != nil {
			return errors.NewValidationError(err.Error(), nil)
		}
	}

	// Log the operation
	logger.Get().WithFields(map[string]interface{}{
		"label":  label,
		"scopes": scopes,
	}).Debug("Creating API key")

	// Create the API key request
	req := requests.CreateAPIKeyRequest{
		Label:  label,
		Scopes: scopes,
	}

	// Create the API key
	apiKey, err := client.CreateAPIKey(req)
	if err != nil {
		return err
	}

	// Handle successful response
	return handler.HandleCreateAPIKey(apiKey, printer.CreateConfig{
		SuccessMessage: "✅ API Key Created Successfully",
		ItemName:       "API key",
		FieldOrder:     []string{"id", "label", "public_key", "secret_key", "scopes", "created_at"},
	})
}
