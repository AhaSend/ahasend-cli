package subaccounts

import (
	"fmt"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
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
		Short: "Update a sub-account",
		Long: `Update a sub-account under your AhaSend parent account.

Only the flags you provide are changed; omitted fields remain unchanged. At
least one of --name, --website or --monthly-credit must be provided. An
explicit --monthly-credit 0 is honored and distinguished from an omitted flag.
--website is sent as the API stores it, as for 'subaccounts create'.`,
		Example: `  # Rename a sub-account
  ahasend subaccounts update 123e4567-e89b-12d3-a456-426614174000 --name "New Name"

  # Update website and monthly credit
  ahasend subaccounts update 123e4567-e89b-12d3-a456-426614174000 --website acme.example.com --monthly-credit 1000`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runSubAccountsUpdate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "New sub-account name")
	cmd.Flags().String("website", "", "New sub-account website domain, e.g. example.com")
	cmd.Flags().Int64("monthly-credit", 0, "New monthly credit allocation (0-1000000000)")

	return cmd
}

func runSubAccountsUpdate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	subAccountID := args[0]

	// Validate the ID before anything else, before auth.
	subAccountUUID, err := validation.ParseUUIDField("sub-account ID", subAccountID)
	if err != nil {
		return err
	}

	// Build the request from changed flags only, distinguishing omitted fields
	// from explicit values.
	req := requests.UpdateSubAccountRequest{}

	name, err := cmdutil.ReadNonEmpty(cmd, "name", "")
	if err != nil {
		return err
	}
	// The name is sent trimmed, as the API stores it, since the SDK refuses
	// one over the limit before the API trims it
	if name != nil {
		trimmed := strings.TrimSpace(*name)
		name = &trimmed
	}
	if err := cmdutil.CheckMaxLength("name", name, cmdutil.MaxNameLength); err != nil {
		return err
	}
	req.Name = name

	website, err := cmdutil.ReadNonEmpty(cmd, "website", "")
	if err != nil {
		return err
	}
	if website != nil {
		normalized, err := normalizeWebsite(*website)
		if err != nil {
			return err
		}
		req.Website = &normalized
	}

	if req.MonthlyCredit, err = readMonthlyCredit(cmd); err != nil {
		return err
	}

	// Require at least one changed flag, before auth.
	if req.Name == nil && req.Website == nil && req.MonthlyCredit == nil {
		return errors.NewValidationError("at least one of --name, --website or --monthly-credit must be provided", nil)
	}

	// SDK backstop validation after local validation and before auth.
	if err := req.Validate(); err != nil {
		return errors.NewValidationError(err.Error(), nil)
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

	response, err := client.UpdateSubAccount(subAccountUUID, req)
	if err != nil {
		return err
	}

	config := printer.SingleConfig{
		SuccessMessage: "Sub-account updated successfully",
		EmptyMessage:   "Sub-account not found",
	}

	return handler.HandleSingleSubAccount(response, config)
}
