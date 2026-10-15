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

// NewDeleteCommand creates the apikeys delete command
func NewDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <key-id>",
		Short: "Delete an API key",
		Long: `Delete an API key permanently.

WARNING: This action is irreversible. Once an API key is deleted:
- The key can no longer be used for authentication
- Any applications using this key will lose access immediately
- The key cannot be recovered or restored

Before deleting an API key, ensure:
- No applications or scripts are actively using it
- You have alternative authentication methods configured
- You have documented any systems that might be affected

Delete asks for confirmation unless --force is set.`,
		Example: `  # Delete an API key (with confirmation)
  ahasend apikeys delete fcb3f3bc-4ac8-4330-948d-1671fcf9a768

  # Force delete without confirmation (for automation)
  ahasend apikeys delete fcb3f3bc-4ac8-4330-948d-1671fcf9a768 --force

  # JSON output for automation
  ahasend apikeys delete fcb3f3bc-4ac8-4330-948d-1671fcf9a768 --force --output json`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runAPIKeyDelete,
		SilenceUsage: true,
	}

	prompt.AddForceFlag(cmd)

	return cmd
}

func runAPIKeyDelete(cmd *cobra.Command, args []string) error {
	keyID := args[0]

	keyUUID, err := validation.ParseUUIDField("API key ID", keyID)
	if err != nil {
		return err
	}

	if err := prompt.Confirm(cmd, fmt.Sprintf("Are you sure you want to delete API key %s?", keyID)); err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"key_id": keyID,
	}).Debug("Deleting API key")

	response, err := client.DeleteAPIKey(keyUUID)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleSuccessResponse(response, printer.ResultConfig{
		SuccessMessage: fmt.Sprintf("API key '%s' deleted successfully", keyID),
	})
}
