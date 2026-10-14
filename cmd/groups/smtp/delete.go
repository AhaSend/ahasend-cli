package smtp

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

// NewDeleteCommand creates the smtp delete command
func NewDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <credential-id>",
		Short: "Delete an SMTP credential",
		Long: `Delete an SMTP credential permanently.

This action cannot be undone. Any applications or services using this
credential will immediately lose access to send emails through SMTP.

Delete asks for confirmation unless --force is set.`,
		Example: `  # Delete with confirmation prompt
  ahasend smtp delete 550e8400-e29b-41d4-a716-446655440000

  # Delete without confirmation (for automation)
  ahasend smtp delete 550e8400-e29b-41d4-a716-446655440000 --force

  # Delete with JSON output
  ahasend smtp delete 550e8400-e29b-41d4-a716-446655440000 --force --output json`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runSMTPDelete,
		SilenceUsage: true,
	}

	prompt.AddForceFlag(cmd)

	return cmd
}

func runSMTPDelete(cmd *cobra.Command, args []string) error {
	credentialID := args[0]
	credentialUUID, err := validation.ParseUUIDField("credential ID", credentialID)
	if err != nil {
		return err
	}
	if err := prompt.Confirm(cmd, fmt.Sprintf("Are you sure you want to delete SMTP credential %s?", credentialID)); err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"credential_id": credentialID,
	}).Info("Deleting SMTP credential")

	response, err := client.DeleteSMTPCredential(credentialUUID)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleSuccessResponse(response, printer.ResultConfig{
		SuccessMessage: fmt.Sprintf("SMTP credential '%s' deleted successfully", credentialID),
	})
}
