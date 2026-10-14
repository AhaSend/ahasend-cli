package lists

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

// NewDeleteCommand creates the lists delete command.
func NewDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <list-id>",
		Short: "Delete a contact list",
		Long: `Delete a contact list. The API refuses while a campaign that still needs the
list refers to it. Delete asks for confirmation unless --force is set.`,
		Example: `  # Delete a contact list
  ahasend lists delete 123e4567-e89b-12d3-a456-426614174000

  # Delete a contact list without confirmation
  ahasend lists delete 123e4567-e89b-12d3-a456-426614174000 --force`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runListsDelete,
		SilenceUsage: true,
	}
	prompt.AddForceFlag(cmd)
	return cmd
}

func runListsDelete(cmd *cobra.Command, args []string) error {
	listID := args[0]
	listUUID, err := validation.ParseUUIDField("list ID", listID)
	if err != nil {
		return err
	}

	if err := prompt.Confirm(cmd, fmt.Sprintf("Are you sure you want to delete list %s?", listID)); err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"list_id": listID,
	}).Debug("Executing lists delete command")

	response, err := apiClient.DeleteList(listUUID)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSuccessResponse(response, printer.ResultConfig{
		SuccessMessage: fmt.Sprintf("List '%s' deleted successfully", listID),
	})
}
