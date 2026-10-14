package members

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewSetCommand creates the lists members set command.
func NewSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <list-id> <id-or-email>",
		Short: "Add a contact to a list or change its membership",
		Long: `Add one existing contact to a list, or change its membership, by contact ID
or raw email.

Without --subscription-status a new member is confirmed and an existing one
keeps its status. A complained membership cannot be changed.`,
		Example: `  # Add a contact to a list
  ahasend lists members set 123e4567-e89b-12d3-a456-426614174000 person@example.com

  # Unsubscribe a contact from a list
  ahasend lists members set 123e4567-e89b-12d3-a456-426614174000 person@example.com --subscription-status unsubscribed`,
		Args:         cmdutil.ExactArgs(2),
		RunE:         runMembersSet,
		SilenceUsage: true,
	}

	cmdutil.AddOneOfFlag(cmd, "subscription-status", "", "Membership status to write: %s", subscriptionStatuses)
	return cmd
}

// subscriptionStatuses are the values of --subscription-status. The API
// refuses complained, which only the mailbox provider sets.
var subscriptionStatuses = []string{"confirmed", "unconfirmed", "unsubscribed"}

func runMembersSet(cmd *cobra.Command, args []string) error {
	listID, idOrEmail := args[0], args[1]
	listUUID, err := validation.ParseUUIDField("list ID", listID)
	if err != nil {
		return err
	}
	if err := validation.ValidatePathSegment("contact ID or email", idOrEmail); err != nil {
		return err
	}

	subscriptionStatus, err := cmdutil.ReadOneOf(cmd, "subscription-status")
	if err != nil {
		return err
	}
	request := requests.UpsertListContactRequest{
		SubscriptionStatus: subscriptionStatus,
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"list_id": listID,
		"contact": idOrEmail,
	}).Debug("Executing lists members set command")

	member, err := apiClient.UpsertListContact(listUUID, idOrEmail, request)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleListMember(member, printer.SingleConfig{
		SuccessMessage: "List member saved successfully",
		EmptyMessage:   "No member returned",
	})
}
