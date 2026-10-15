package apikeys

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewDeleteCommand creates the `subaccounts api-keys delete` command.
func NewDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <sub-account-id> <key-id>",
		Short: "Delete a sub-account API key",
		Long: `Delete an API key that belongs to a sub-account permanently.

WARNING: This action is irreversible. Once the API key is deleted:
- The key can no longer be used for authentication
- Any applications using this key will lose access immediately
- The key cannot be recovered or restored

Delete asks for confirmation unless --force is set.`,
		Example: `  # Delete a sub-account API key (with confirmation)
  ahasend subaccounts api-keys delete 123e4567-e89b-12d3-a456-426614174000 223e4567-e89b-12d3-a456-426614174000

  # Force delete without confirmation (for automation)
  ahasend subaccounts api-keys delete 123e4567-e89b-12d3-a456-426614174000 223e4567-e89b-12d3-a456-426614174000 --force`,
		Args:         cmdutil.ExactArgs(2),
		RunE:         runSubAccountAPIKeyDelete,
		SilenceUsage: true,
	}

	prompt.AddForceFlag(cmd)

	return cmd
}

func runSubAccountAPIKeyDelete(cmd *cobra.Command, args []string) error {
	subAccountID := args[0]
	keyID := args[1]
	subAccountUUID, err := validation.ParseUUIDField("sub-account ID", subAccountID)
	if err != nil {
		return err
	}
	keyUUID, err := validation.ParseUUIDField("API key ID", keyID)
	if err != nil {
		return err
	}

	if err := prompt.Confirm(cmd, fmt.Sprintf("Are you sure you want to delete API key %s of sub-account %s?", keyID, subAccountID)); err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"sub_account_id": subAccountID,
		"key_id":         keyID,
	}).Debug("Executing subaccounts api-keys delete command")

	response, err := client.DeleteSubAccountAPIKey(subAccountUUID, keyUUID)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleSuccessResponse(response, printer.ResultConfig{
		SuccessMessage: fmt.Sprintf("API key '%s' deleted successfully", keyID),
	})
}
