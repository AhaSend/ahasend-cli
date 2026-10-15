package members

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewAddCommand creates the lists members add command.
func NewAddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <list-id>",
		Short: "Add existing contacts to a list",
		Long: `Add up to 1,000 existing contacts to a list in one request.

Name contacts with --email and --contact-id, which can be used multiple times,
split on commas and combine, or with --file, a JSON file holding
{"data":[...]} whose items are {"email": "..."} or {"id": "..."}. The API
reports each contact's outcome: contacts already on the list are skipped, and
unknown or invalid ones fail with a reason. The whole result is printed, and
the command exits 1 when any contact failed.`,
		Example: `  # Add contacts by email address
  ahasend lists members add 123e4567-e89b-12d3-a456-426614174000 --email a@example.com --email b@example.com

  # Add a contact by ID
  ahasend lists members add 123e4567-e89b-12d3-a456-426614174000 --contact-id 9b2f8c3e-1d4a-4e5b-8c6d-7e8f9a0b1c2d

  # Add the contacts in a file
  ahasend lists members add 123e4567-e89b-12d3-a456-426614174000 --file members.json`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runMembersAdd,
		SilenceUsage: true,
	}

	cmdutil.AddRepeatableFlag(cmd, "email", "Email address of a contact to add (can be used multiple times)")
	cmdutil.AddRepeatableFlag(cmd, "contact-id", "ID of a contact to add (can be used multiple times)")
	cmd.Flags().String("file", "", `JSON file containing {"data":[...]}`)
	cmdutil.MarkFileFlag(cmd, "file")
	cmdutil.AddIdempotencyKeyFlag(cmd)
	return cmd
}

func runMembersAdd(cmd *cobra.Command, args []string) error {
	listID := args[0]
	listUUID, err := validation.ParseUUIDField("list ID", listID)
	if err != nil {
		return err
	}

	emails, err := cmdutil.ReadRepeatable(cmd, "email", "")
	if err != nil {
		return err
	}
	contactIDs, err := cmdutil.ReadRepeatable(cmd, "contact-id", "")
	if err != nil {
		return err
	}
	for _, flag := range []string{"email", "contact-id"} {
		if err := cmdutil.RejectTogether(cmd, "file", flag); err != nil {
			return err
		}
	}
	path, _ := cmd.Flags().GetString("file")
	if path == "" && emails == nil && contactIDs == nil {
		return errors.NewValidationError("one of --email, --contact-id or --file must be provided", nil)
	}
	if count := len(emails) + len(contactIDs); count > cmdutil.MaxDataFileItems {
		return errors.NewValidationError(fmt.Sprintf(
			"--email and --contact-id take at most %d members in total, got %d", cmdutil.MaxDataFileItems, count), nil)
	}

	var request requests.BatchAddListContactsRequest
	if path != "" {
		request, err = readMembersFile(path)
	} else {
		request, err = membersFromFlags(emails, contactIDs)
	}
	if err != nil {
		return err
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
		"list_id": listID,
		"members": len(request.Data),
	}).Debug("Executing lists members add command")

	response, err := apiClient.BatchAddListContacts(listUUID, request, idempotency...)
	if err != nil {
		return err
	}
	if err := printer.GetResponseHandlerFromCommand(cmd).HandleBatchListMembers(response, printer.SingleConfig{
		SuccessMessage: "List member batch completed",
		EmptyMessage:   "No member results returned",
	}); err != nil {
		return err
	}
	if response != nil && response.Failed > 0 {
		return errors.NewItemsFailedError(response.Failed, len(response.Data), "contacts")
	}
	return nil
}

// membersFromFlags builds one item per --email value, in order, followed by
// one item per --contact-id value.
func membersFromFlags(emails, contactIDs []string) (requests.BatchAddListContactsRequest, error) {
	request := requests.BatchAddListContactsRequest{
		Data: make([]requests.BatchAddListContactInput, 0, len(emails)+len(contactIDs)),
	}
	for _, email := range emails {
		request.Data = append(request.Data, requests.BatchAddListContactInput{Email: &email})
	}
	ids, err := validation.ParseUUIDs("contact ID", contactIDs)
	if err != nil {
		return requests.BatchAddListContactsRequest{}, err
	}
	for _, id := range ids {
		request.Data = append(request.Data, requests.BatchAddListContactInput{ID: &id})
	}
	return request, nil
}

// readMembersFile reads a members file. An item with both or neither of
// email and id is sent as written, and the API reports it as invalid.
func readMembersFile(path string) (requests.BatchAddListContactsRequest, error) {
	items, err := cmdutil.ReadDataItems[requests.BatchAddListContactInput](path, "members", "member")
	if err != nil {
		return requests.BatchAddListContactsRequest{}, err
	}
	return requests.BatchAddListContactsRequest{Data: items}, nil
}
