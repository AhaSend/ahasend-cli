package contacts

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the contact get command.
func NewGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id-or-email>",
		Short: "Get a contact",
		Long: `Get one contact by ID or raw email address. Pass email addresses exactly as
written.`,
		Example: `  # Get a contact by email address
  ahasend contacts get 'user+tag@example.com'`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runContactsGet,
		SilenceUsage: true,
	}
}

func runContactsGet(cmd *cobra.Command, args []string) error {
	idOrEmail := args[0]
	if err := validation.ValidatePathSegment("contact ID or email", idOrEmail); err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"contact": idOrEmail,
	}).Debug("Executing contacts get command")

	contact, err := apiClient.GetContact(idOrEmail)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleContact(contact, printer.SingleConfig{
		SuccessMessage: "Contact retrieved successfully",
		EmptyMessage:   "Contact not found",
	})
}
