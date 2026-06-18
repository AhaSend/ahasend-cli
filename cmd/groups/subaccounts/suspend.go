package subaccounts

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
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
		Use:   "suspend <sub-account-id>",
		Short: "Suspend a sub-account",
		Long: `Suspend a sub-account under your AhaSend parent account.

A suspended sub-account cannot send email until it is unsuspended. A reason is
required and is recorded with the suspension.

Use the --force flag to skip the confirmation prompt for automation.`,
		Example: `  # Suspend a sub-account
  ahasend subaccounts suspend 123e4567-e89b-12d3-a456-426614174000 --reason "Payment overdue"

  # Force suspend without confirmation (for automation)
  ahasend subaccounts suspend 123e4567-e89b-12d3-a456-426614174000 --reason "Payment overdue" --force`,
		Args:         cobra.ExactArgs(1),
		RunE:         runSubAccountsSuspend,
		SilenceUsage: true,
	}

	cmd.Flags().String("reason", "", "Reason for the suspension (required)")
	cmd.Flags().Bool("force", false, "Skip confirmation prompt")

	return cmd
}

func runSubAccountsSuspend(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	subAccountID := args[0]

	// Validate the ID before anything else, before auth and before confirmation.
	if err := validation.ValidateUUIDField("sub-account ID", subAccountID); err != nil {
		return err
	}

	// Validate the reason locally before auth: trimmed, non-empty, and within
	// the maximum length.
	reason, _ := cmd.Flags().GetString("reason")
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errors.NewValidationError("--reason is required", nil)
	}
	if len(reason) > suspensionReasonMaxLength {
		return errors.NewValidationError(fmt.Sprintf("--reason too long: %d characters (max %d)", len(reason), suspensionReasonMaxLength), nil)
	}

	req := requests.SuspendSubAccountRequest{Reason: reason}

	// SDK backstop validation after local validation and before auth.
	if err := req.Validate(); err != nil {
		return err
	}

	force, _ := cmd.Flags().GetBool("force")

	// Confirm only after all local validation passes.
	if !force {
		if err := confirmSubAccountStateChange("suspend", subAccountID); err != nil {
			return err
		}
	}

	// Only authenticate after local validation passes.
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"sub_account_id": subAccountID,
		"force":          force,
	}).Debug("Executing subaccounts suspend command")

	response, err := client.SuspendSubAccount(subAccountID, req)
	if err != nil {
		return err
	}

	if response == nil {
		return errors.NewNotFoundError(fmt.Sprintf("sub-account '%s' not found", subAccountID), nil)
	}

	config := printer.SingleConfig{
		SuccessMessage: fmt.Sprintf("Sub-account '%s' suspended successfully", subAccountID),
		EmptyMessage:   "Sub-account not found",
		FieldOrder:     []string{"name", "id", "parent_account_id", "status", "website", "monthly_credit", "domain_count", "member_count", "created_at", "last_activity_at"},
	}

	return handler.HandleSingleSubAccount(response, config)
}

func confirmSubAccountStateChange(action, subAccountID string) error {
	fmt.Printf("⚠️  You are about to %s sub-account: %s\n", action, subAccountID)
	fmt.Print("Are you sure you want to continue? (y/N): ")

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		response := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if response == "y" || response == "yes" {
			return nil
		}
	}

	return errors.NewValidationError("operation cancelled", nil)
}
