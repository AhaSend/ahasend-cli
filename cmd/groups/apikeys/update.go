package apikeys

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewUpdateCommand creates the apikeys update command
func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <key-id>",
		Short: "Update an existing API key",
		Long: `Update an existing API key's label and scopes.

You can update the following properties of an API key:
- Label: Change the descriptive name for identification
- Scopes: Modify the permissions granted to the key

When updating scopes, the new scopes completely replace the existing ones.
If you want to add a scope, include all existing scopes plus the new one.

The API key secret cannot be changed. If you need a new secret, create a new
API key and delete the old one.`,
		Example: `  # Update API key label
  ahasend apikeys update ak_1234567890abcdef --label "Updated Label"

  # Update API key scopes
  ahasend apikeys update ak_1234567890abcdef \
    --scope messages:read,messages:write,statistics:read

  # Update both label and scopes
  ahasend apikeys update ak_1234567890abcdef \
    --label "Production API v2" \
    --scope full

  # JSON output for automation
  ahasend apikeys update ak_1234567890abcdef \
    --label "New Label" \
    --output json`,
		Args: cobra.ExactArgs(1),
		RunE: runAPIKeyUpdate,
	}

	// Update flags
	cmd.Flags().String("label", "", "New label for the API key")
	cmd.Flags().StringSlice("scope", []string{}, "New scopes to grant (replaces existing scopes)")

	return cmd
}

func runAPIKeyUpdate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	keyID := args[0]

	// Get flag values
	label, _ := cmd.Flags().GetString("label")
	scopes, _ := cmd.Flags().GetStringSlice("scope")

	// Validate that at least one field is being updated
	if label == "" && len(scopes) == 0 {
		return errors.NewValidationError("at least one of --label or --scope must be provided", nil)
	}

	// Validate all scopes if provided
	if len(scopes) > 0 {
		for _, scope := range scopes {
			if err := validateScope(scope); err != nil {
				return errors.NewValidationError(err.Error(), nil)
			}
		}
	}

	// Log the operation
	logger.Get().WithFields(map[string]interface{}{
		"key_id": keyID,
		"label":  label,
		"scopes": scopes,
	}).Debug("Updating API key")

	// Create the update request
	req := requests.UpdateAPIKeyRequest{}

	// Only include fields that were provided
	if label != "" {
		req.Label = &label
	}
	if len(scopes) > 0 {
		req.Scopes = &scopes
	}

	// Update the API key
	apiKey, err := client.UpdateAPIKey(keyID, req)
	if err != nil {
		return handler.HandleError(err)
	}

	// Handle successful response
	return handler.HandleUpdateAPIKey(apiKey, printer.UpdateConfig{
		SuccessMessage: "✅ API Key Updated Successfully",
		ItemName:       "API key",
		FieldOrder:     []string{"id", "label", "scopes", "created_at", "updated_at"},
	})
}
