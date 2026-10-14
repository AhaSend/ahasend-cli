package contacts

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// contactUpdateFlags are the flags that change an update request.
var contactUpdateFlags = []string{"email", "first-name", "last-name", "status", "status-reason", "unsubscribed", "attributes"}

// NewUpdateCommand creates the contact update command.
func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <id-or-email>",
		Short: "Update a contact",
		Long: `Partially update one contact by ID or raw email. Only explicitly supplied
flags are sent. --first-name "", --last-name "" and --status-reason "" clear
those fields. A null attribute member removes that attribute.`,
		Example: `  # Unsubscribe a contact
  ahasend contacts update person@example.com --unsubscribed=true

  # Remove an attribute from a contact
  ahasend contacts update person@example.com --attributes '{"old_key":null}'`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runContactsUpdate,
		SilenceUsage: true,
	}
	addUpdateFieldFlags(cmd)
	return cmd
}

func runContactsUpdate(cmd *cobra.Command, args []string) error {
	idOrEmail := args[0]
	if err := validation.ValidatePathSegment("contact ID or email", idOrEmail); err != nil {
		return err
	}

	hasChange := false
	for _, name := range contactUpdateFlags {
		if cmd.Flags().Changed(name) {
			hasChange = true
			break
		}
	}
	if !hasChange {
		return errors.NewValidationError("at least one of --email, --first-name, --last-name, --status, --status-reason, --unsubscribed or --attributes must be provided", nil)
	}

	email, err := cmdutil.ReadNonEmpty(cmd, "email", "")
	if err != nil {
		return err
	}
	if err := validation.ValidateOptionalEmail("--email", email); err != nil {
		return err
	}
	status, err := cmdutil.ReadOneOf(cmd, "status")
	if err != nil {
		return err
	}
	attributes, err := readAttributes(cmd)
	if err != nil {
		return err
	}
	request := requests.UpdateContactRequest{
		Email:        email,
		FirstName:    cmdutil.ChangedString(cmd, "first-name"),
		LastName:     cmdutil.ChangedString(cmd, "last-name"),
		Status:       status,
		StatusReason: cmdutil.ChangedString(cmd, "status-reason"),
		Unsubscribed: cmdutil.ChangedBool(cmd, "unsubscribed"),
		Attributes:   attributes,
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"contact": idOrEmail,
	}).Debug("Executing contacts update command")

	contact, err := apiClient.UpdateContact(idOrEmail, request)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleContact(contact, printer.SingleConfig{
		SuccessMessage: "Contact updated successfully",
		EmptyMessage:   "No contact updated",
	})
}
