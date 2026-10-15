package subaccounts

import (
	"fmt"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// suspensionReasonMaxLength bounds the required --reason flag locally, mirroring
// the SDK's SuspendSubAccountRequest.Validate() backstop so an over-long reason
// is rejected before authentication.
const suspensionReasonMaxLength = 500

// NewSuspendCommand creates the suspend command
func NewSuspendCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "suspend <sub-account-id>",
		Aliases: []string{"pause"},
		Short:   "Suspend a sub-account",
		Long: `Suspend a sub-account under your AhaSend parent account.

A suspended sub-account cannot send email until it is unsuspended. A reason is
required and is recorded with the suspension. "pause" is an alias of this
command.

Suspend asks for confirmation unless --force is set.`,
		Example: `  # Suspend a sub-account
  ahasend subaccounts suspend 123e4567-e89b-12d3-a456-426614174000 --reason "Payment overdue"

  # Force suspend without confirmation (for automation)
  ahasend subaccounts suspend 123e4567-e89b-12d3-a456-426614174000 --reason "Payment overdue" --force`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runSubAccountsSuspend,
		SilenceUsage: true,
	}

	cmd.Flags().String("reason", "", "Reason for the suspension (required)")
	prompt.AddForceFlag(cmd)

	return cmd
}

func runSubAccountsSuspend(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	subAccountID := args[0]

	// Validate the ID before anything else, before auth and before confirmation.
	subAccountUUID, err := validation.ParseUUIDField("sub-account ID", subAccountID)
	if err != nil {
		return err
	}

	// Validate the reason locally before auth: not blank, and within the
	// maximum length once trimmed, as it is sent.
	reason, err := cmdutil.ReadRequiredText(cmd, "reason")
	if err != nil {
		return err
	}
	reason = strings.TrimSpace(reason)
	if err := cmdutil.CheckMaxLength("reason", &reason, suspensionReasonMaxLength); err != nil {
		return err
	}

	req := requests.SuspendSubAccountRequest{Reason: reason}

	// SDK backstop validation after local validation and before auth.
	if err := req.Validate(); err != nil {
		return errors.NewValidationError(err.Error(), nil)
	}

	if err := prompt.Confirm(cmd, fmt.Sprintf("Are you sure you want to suspend sub-account %s?", subAccountID)); err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"sub_account_id": subAccountID,
	}).Debug("Executing subaccounts suspend command")

	response, err := client.SuspendSubAccount(subAccountUUID, req)
	if err != nil {
		return err
	}

	config := printer.SingleConfig{
		SuccessMessage: "Sub-account suspended successfully",
		EmptyMessage:   "Sub-account not found",
	}

	return handler.HandleSingleSubAccount(response, config)
}
