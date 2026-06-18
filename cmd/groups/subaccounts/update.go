package subaccounts

import (
	"fmt"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewUpdateCommand creates the update command
func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <sub-account-id>",
		Short: "Update an existing sub-account",
		Long: `Update an existing sub-account under your AhaSend parent account.

Only the flags you provide are changed; omitted fields remain unchanged. At least
one of --name, --website, or --monthly-credit must be provided. An explicit
--monthly-credit 0 is honored and distinguished from an omitted flag.`,
		Example: `  # Rename a sub-account
  ahasend subaccounts update 123e4567-e89b-12d3-a456-426614174000 --name "New Name"

  # Update website and monthly credit
  ahasend subaccounts update 123e4567-e89b-12d3-a456-426614174000 --website https://acme.example --monthly-credit 1000`,
		Args:         cobra.ExactArgs(1),
		RunE:         runSubAccountsUpdate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "New sub-account name")
	cmd.Flags().String("website", "", "New sub-account website")
	cmd.Flags().Int64("monthly-credit", 0, "New monthly credit allocation (0-1000000000)")

	return cmd
}

func runSubAccountsUpdate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	subAccountID := args[0]

	// Validate the ID before anything else, before auth.
	if err := validation.ValidateUUIDField("sub-account ID", subAccountID); err != nil {
		return err
	}

	// Build the request from changed flags only, distinguishing omitted fields
	// from explicit values.
	req := requests.UpdateSubAccountRequest{}

	if cmd.Flags().Changed("name") {
		v, _ := cmd.Flags().GetString("name")
		v = strings.TrimSpace(v)
		if v == "" {
			return errors.NewValidationError("--name cannot be empty", nil)
		}
		req.Name = &v
	}

	if cmd.Flags().Changed("website") {
		v, _ := cmd.Flags().GetString("website")
		if err := validateWebsite(v); err != nil {
			return err
		}
		req.Website = &v
	}

	if cmd.Flags().Changed("monthly-credit") {
		v, _ := cmd.Flags().GetInt64("monthly-credit")
		if err := validateMonthlyCredit(v); err != nil {
			return err
		}
		req.MonthlyCredit = &v
	}

	// Require at least one changed flag, before auth.
	if req.Name == nil && req.Website == nil && req.MonthlyCredit == nil {
		return errors.NewValidationError("at least one of --name, --website, or --monthly-credit must be provided", nil)
	}

	// SDK backstop validation after local validation and before auth.
	if err := req.Validate(); err != nil {
		return err
	}

	// Only authenticate after local validation passes
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"sub_account_id": subAccountID,
		"request":        fmt.Sprintf("%+v", req),
	}).Debug("Executing subaccounts update command")

	response, err := client.UpdateSubAccount(subAccountID, req)
	if err != nil {
		return err
	}

	if response == nil {
		return errors.NewNotFoundError(fmt.Sprintf("sub-account '%s' not found", subAccountID), nil)
	}

	config := printer.SingleConfig{
		SuccessMessage: fmt.Sprintf("Sub-account '%s' updated successfully", subAccountID),
		EmptyMessage:   "Sub-account not found",
		FieldOrder:     []string{"name", "id", "parent_account_id", "status", "website", "monthly_credit", "domain_count", "member_count", "created_at", "last_activity_at"},
	}

	return handler.HandleSingleSubAccount(response, config)
}
