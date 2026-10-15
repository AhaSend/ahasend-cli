package members

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

// NewRemoveCommand creates the lists members remove command.
func NewRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <list-id> <id-or-email>",
		Short: "Remove a contact from a list",
		Long: `Remove one contact from a list by contact ID or raw email. This deletes the
membership and any unsubscribe it records. To stop mail on the list and keep
the record, use "lists members set --subscription-status unsubscribed" instead.
A complained membership cannot be removed. Remove asks for confirmation unless
--force is set.`,
		Example: `  # Remove a contact from a list
  ahasend lists members remove 123e4567-e89b-12d3-a456-426614174000 person@example.com

  # Remove a contact without confirmation
  ahasend lists members remove 123e4567-e89b-12d3-a456-426614174000 person@example.com --force`,
		Args:         cmdutil.ExactArgs(2),
		RunE:         runMembersRemove,
		SilenceUsage: true,
	}
	prompt.AddForceFlag(cmd)
	return cmd
}

func runMembersRemove(cmd *cobra.Command, args []string) error {
	listID, idOrEmail := args[0], args[1]
	listUUID, err := validation.ParseUUIDField("list ID", listID)
	if err != nil {
		return err
	}
	if err := validation.ValidatePathSegment("contact ID or email", idOrEmail); err != nil {
		return err
	}

	question := fmt.Sprintf("Are you sure you want to remove contact %s from list %s?", idOrEmail, listID)
	if err := prompt.Confirm(cmd, question); err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"list_id": listID,
		"contact": idOrEmail,
	}).Debug("Executing lists members remove command")

	response, err := apiClient.DeleteListContact(listUUID, idOrEmail)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSuccessResponse(response, printer.ResultConfig{
		SuccessMessage: fmt.Sprintf("Contact '%s' removed from list '%s' successfully", idOrEmail, listID),
	})
}
