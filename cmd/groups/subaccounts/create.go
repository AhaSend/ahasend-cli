package subaccounts

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// monthlyCreditMin and monthlyCreditMax bound the optional --monthly-credit
// flag locally, mirroring the SDK's CreateSubAccountRequest.Validate() backstop
// so out-of-range values are rejected before authentication.
const (
	monthlyCreditMin = int64(0)
	monthlyCreditMax = int64(1000000000)
)

// NewCreateCommand creates the create command
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new sub-account",
		Long: `Create a new sub-account under your AhaSend parent account.

A sub-account requires a name and a website. You can optionally set a monthly
credit allocation. Creation is idempotent: provide your own --idempotency-key to
make retries safe, or one is generated for you.`,
		Example: `  # Create a sub-account
  ahasend subaccounts create --name "Acme Inc" --website https://acme.example

  # Create with a monthly credit allocation
  ahasend subaccounts create --name "Acme Inc" --website https://acme.example --monthly-credit 5000

  # Create with a custom idempotency key for safe retries
  ahasend subaccounts create --name "Acme Inc" --website https://acme.example --idempotency-key my-unique-key`,
		Args:         cobra.NoArgs,
		RunE:         runSubAccountsCreate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "Sub-account name (required)")
	cmd.Flags().String("website", "", "Sub-account website (required)")
	cmd.Flags().Int64("monthly-credit", 0, "Monthly credit allocation (0-1000000000)")
	cmd.Flags().String("idempotency-key", "", "Idempotency key for safe retries (auto-generated if not provided)")

	return cmd
}

func runSubAccountsCreate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	// Get flags
	name, _ := cmd.Flags().GetString("name")
	website, _ := cmd.Flags().GetString("website")
	idempotencyKey, _ := cmd.Flags().GetString("idempotency-key")

	// Validate name locally (trim, require non-empty)
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.NewValidationError("--name is required", nil)
	}

	// Validate the website host but send the original website string verbatim.
	if err := validateWebsite(website); err != nil {
		return err
	}

	// Build the request, distinguishing an omitted --monthly-credit from an
	// explicit zero via Changed().
	req := requests.CreateSubAccountRequest{
		Name:    name,
		Website: website,
	}

	if cmd.Flags().Changed("monthly-credit") {
		credit, _ := cmd.Flags().GetInt64("monthly-credit")
		if err := validateMonthlyCredit(credit); err != nil {
			return err
		}
		req.MonthlyCredit = &credit
	}

	// Generate an idempotency key when the user did not provide one so the
	// create call is always safe to retry.
	if idempotencyKey == "" {
		idempotencyKey = uuid.NewString()
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
		"name":            name,
		"website":         website,
		"monthly_credit":  req.MonthlyCredit,
		"idempotency_key": idempotencyKey,
	}).Debug("Executing subaccounts create command")

	response, err := client.CreateSubAccount(req, idempotencyKey)
	if err != nil {
		return err
	}

	if response == nil {
		return errors.NewAPIError("received nil response from API", nil)
	}

	config := printer.SingleConfig{
		SuccessMessage: fmt.Sprintf("Sub-account '%s' created successfully", name),
		EmptyMessage:   "No sub-account created",
		FieldOrder:     []string{"name", "id", "parent_account_id", "status", "website", "monthly_credit", "domain_count", "member_count", "created_at", "last_activity_at"},
	}

	return handler.HandleSingleSubAccount(response, config)
}

// validateWebsite parses the website and requires a host, leaving the original
// string untouched so it is sent to the API verbatim.
func validateWebsite(website string) error {
	if strings.TrimSpace(website) == "" {
		return errors.NewValidationError("--website is required", nil)
	}

	parsed, err := url.Parse(website)
	if err != nil || parsed.Host == "" {
		return errors.NewValidationError("invalid website: "+website+" (must include a host, e.g. https://example.com)", nil)
	}

	return nil
}

// validateMonthlyCredit enforces the 0..1000000000 range locally, before auth.
func validateMonthlyCredit(credit int64) error {
	if credit < monthlyCreditMin || credit > monthlyCreditMax {
		return errors.NewValidationError(fmt.Sprintf("invalid monthly credit: %d (must be between %d and %d)", credit, monthlyCreditMin, monthlyCreditMax), nil)
	}
	return nil
}
