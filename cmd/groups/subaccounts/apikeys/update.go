package apikeys

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewUpdateCommand creates the `subaccounts api-keys update` command.
func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <sub-account-id> <key-id>",
		Short: "Update a sub-account API key",
		Long: `Update the label, scopes and IP allow list of an API key that belongs to a
sub-account.

When updating scopes, the new scopes completely replace the existing ones. To
add a scope, include all existing scopes plus the new one. --ip-allow-list
likewise replaces the whole list, and --clear-ip-allow-list removes it so the
key works from any IP.

At least one of --label, --scope, --ip-allow-list or --clear-ip-allow-list
must be provided.

` + cmdutil.ScopesHelp(cmdutil.SubAccountKey),
		Example: `  # Update a sub-account API key's label
  ahasend subaccounts api-keys update 123e4567-e89b-12d3-a456-426614174000 223e4567-e89b-12d3-a456-426614174000 \
    --label "Updated Label"

  # Replace a sub-account API key's scopes
  ahasend subaccounts api-keys update 123e4567-e89b-12d3-a456-426614174000 223e4567-e89b-12d3-a456-426614174000 \
    --scope messages:send:all \
    --scope domains:read

  # Allow a sub-account API key from any IP again
  ahasend subaccounts api-keys update 123e4567-e89b-12d3-a456-426614174000 223e4567-e89b-12d3-a456-426614174000 \
    --clear-ip-allow-list`,
		Args:         cmdutil.ExactArgs(2),
		RunE:         runSubAccountAPIKeyUpdate,
		SilenceUsage: true,
	}

	cmdutil.AddAPIKeyUpdateFlags(cmd, cmdutil.SubAccountKey)

	return cmd
}

func runSubAccountAPIKeyUpdate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	subAccountID := args[0]
	keyID := args[1]

	// Validate before auth: both positional UUIDs and the at-least-one-of guard.
	subAccountUUID, err := validation.ParseUUIDField("sub-account ID", subAccountID)
	if err != nil {
		return err
	}
	keyUUID, err := validation.ParseUUIDField("API key ID", keyID)
	if err != nil {
		return err
	}
	req, err := cmdutil.ReadAPIKeyUpdate(cmd, cmdutil.SubAccountKey)
	if err != nil {
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
	}).Debug("Executing subaccounts api-keys update command")

	apiKey, err := client.UpdateSubAccountAPIKey(subAccountUUID, keyUUID, req)
	if err != nil {
		return err
	}

	// Reuse the shared update renderer for output parity with the top-level
	// apikeys command.
	return handler.HandleUpdateAPIKey(apiKey, printer.ResultConfig{
		SuccessMessage: "API key updated successfully",
	})
}
