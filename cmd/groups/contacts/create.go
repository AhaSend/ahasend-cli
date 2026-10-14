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

// NewCreateCommand creates the contact create command.
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <email>",
		Short: "Create a contact",
		Long: `Create one account-global contact. Attribute keys must already exist in the
AhaSend dashboard, and attribute values cannot be null. Creation does not add
the contact to a list.`,
		Example: `  # Create a contact with a first name
  ahasend contacts create person@example.com --first-name Pat

  # Create a contact with attributes and an idempotency key
  ahasend contacts create person@example.com --attributes '{"plan":"pro"}' --idempotency-key import-42`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runContactsCreate,
		SilenceUsage: true,
	}
	addCreateFieldFlags(cmd)
	cmdutil.AddIdempotencyKeyFlag(cmd)
	return cmd
}

func runContactsCreate(cmd *cobra.Command, args []string) error {
	if err := validation.ValidateEmail("email", args[0]); err != nil {
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
	request := requests.CreateContactRequest{
		Email:        args[0],
		FirstName:    cmdutil.ChangedString(cmd, "first-name"),
		LastName:     cmdutil.ChangedString(cmd, "last-name"),
		Status:       status,
		StatusReason: cmdutil.ChangedString(cmd, "status-reason"),
		Unsubscribed: cmdutil.ChangedBool(cmd, "unsubscribed"),
		Attributes:   attributes,
	}
	if err := request.Validate(); err != nil {
		return errors.NewValidationError(err.Error(), nil)
	}

	idempotency, err := cmdutil.IdempotencyOptions(cmd)
	if err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"email": request.Email,
	}).Debug("Executing contacts create command")

	contact, err := apiClient.CreateContact(request, idempotency...)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleContact(contact, printer.SingleConfig{
		SuccessMessage: "Contact created successfully",
		EmptyMessage:   "No contact created",
	})
}
