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

// NewUpdateCommand creates the `subaccounts api-keys update` command.
func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <sub-account-id> <key-id>",
		Short: "Update a sub-account API key",
		Long: `Update the label and/or scopes of an API key that belongs to a sub-account.

When updating scopes, the new scopes completely replace the existing ones. To
add a scope, include all existing scopes plus the new one.

At least one of --label or --scope must be provided.`,
		Example: `  # Update a sub-account API key's label
  ahasend subaccounts api-keys update 123e4567-e89b-12d3-a456-426614174000 223e4567-e89b-12d3-a456-426614174000 \
    --label "Updated Label"

  # Replace a sub-account API key's scopes
  ahasend subaccounts api-keys update 123e4567-e89b-12d3-a456-426614174000 223e4567-e89b-12d3-a456-426614174000 \
    --scope messages:send:all \
    --scope domains:read`,
		Args:         cobra.ExactArgs(2),
		RunE:         runSubAccountAPIKeyUpdate,
		SilenceUsage: true,
	}

	cmd.Flags().String("label", "", "New label for the API key")
	cmd.Flags().StringSlice("scope", []string{}, "New scopes to grant (replaces existing scopes)")

	return cmd
}

func runSubAccountAPIKeyUpdate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	subAccountID := args[0]
	keyID := args[1]

	// Get flag values
	label, _ := cmd.Flags().GetString("label")
	scopes, _ := cmd.Flags().GetStringSlice("scope")

	// Validate before auth: both positional UUIDs and the at-least-one-of guard.
	if err := validation.ValidateUUIDField("sub-account ID", subAccountID); err != nil {
		return err
	}
	if err := validation.ValidateUUIDField("API key ID", keyID); err != nil {
		return err
	}
	if label == "" && len(scopes) == 0 {
		return errors.NewValidationError("at least one of --label or --scope must be provided", nil)
	}

	// Validate all scopes if provided
	if len(scopes) > 0 {
		for _, scope := range scopes {
			if err := validation.ValidateScope(scope); err != nil {
				return errors.NewValidationError(err.Error(), nil)
			}
		}
	}

	// Build the update request, including only fields that were provided.
	req := requests.UpdateAPIKeyRequest{}
	if label != "" {
		req.Label = &label
	}
	if len(scopes) > 0 {
		req.Scopes = &scopes
	}

	// SDK backstop validation after local validation and before auth.
	if err := req.Validate(); err != nil {
		return err
	}

	// Only authenticate after local validation passes
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"sub_account_id": subAccountID,
		"key_id":         keyID,
		"label":          label,
		"scopes":         scopes,
	}).Debug("Executing subaccounts api-keys update command")

	apiKey, err := client.UpdateSubAccountAPIKey(subAccountID, keyID, req)
	if err != nil {
		return err
	}

	// Reuse the shared update renderer for output parity with the top-level
	// apikeys command.
	return handler.HandleUpdateAPIKey(apiKey, printer.UpdateConfig{
		SuccessMessage: "✅ API Key Updated Successfully",
		ItemName:       "API key",
		FieldOrder:     []string{"id", "label", "scopes", "created_at", "updated_at"},
	})
}
