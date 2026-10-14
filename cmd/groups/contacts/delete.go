package contacts

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

const contactDeleteWarning = `WARNING: Permanently deleting this contact:
- cancels all active workflows; cancelled workflow audit rows are retained
- erases unsubscribe history
- erases membership history
- erases campaign history
- erases open history
- erases click history

To stop mail without erasing history, use contacts update --unsubscribed=true
instead.`

// NewDeleteCommand creates the contact delete command.
func NewDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <id-or-email>",
		Short: "Permanently delete a contact",
		Long: `Permanently delete one contact by ID or raw email. Delete asks for
confirmation unless --force is set.

` + contactDeleteWarning,
		Example: `  # Delete a contact
  ahasend contacts delete person@example.com

  # Delete a contact without confirmation
  ahasend contacts delete person@example.com --force`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runContactsDelete,
		SilenceUsage: true,
	}
	prompt.AddForceFlag(cmd)
	return cmd
}

func runContactsDelete(cmd *cobra.Command, args []string) error {
	idOrEmail := args[0]
	if err := validation.ValidatePathSegment("contact ID or email", idOrEmail); err != nil {
		return err
	}

	question := fmt.Sprintf("%s\n\nAre you sure you want to delete contact %s?", contactDeleteWarning, idOrEmail)
	if err := prompt.Confirm(cmd, question); err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"contact": idOrEmail,
	}).Debug("Executing contacts delete command")

	response, err := apiClient.DeleteContact(idOrEmail)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSuccessResponse(response, printer.ResultConfig{
		SuccessMessage: fmt.Sprintf("Contact '%s' deleted successfully", idOrEmail),
	})
}
