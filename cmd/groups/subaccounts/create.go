package subaccounts

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
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

A sub-account requires a name and a website domain, such as example.com. A URL
with only a scheme and host, such as https://example.com, is also accepted and
sent as its domain. You can optionally set a monthly
credit allocation. Creation is idempotent: provide your own --idempotency-key to
make retries safe, or one is generated for you.`,
		Example: `  # Create a sub-account
  ahasend subaccounts create --name "Acme Inc" --website acme.example.com

  # Create with a monthly credit allocation
  ahasend subaccounts create --name "Acme Inc" --website acme.example.com --monthly-credit 5000

  # Create with a custom idempotency key for safe retries
  ahasend subaccounts create --name "Acme Inc" --website acme.example.com --idempotency-key my-unique-key`,
		Args:         cobra.NoArgs,
		RunE:         runSubAccountsCreate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "Sub-account name (required)")
	cmd.Flags().String("website", "", "Sub-account website domain, e.g. example.com (required)")
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

	website, err := normalizeWebsite(website)
	if err != nil {
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

// normalizeWebsite returns the bare domain the API expects for a sub-account
// website. It accepts a domain such as "example.com", or an http(s) URL made of
// only a scheme and host, such as "https://example.com/", which is reduced to
// its host. URLs with credentials, a port, a path, a query, or a fragment are
// rejected rather than silently trimmed.
func normalizeWebsite(website string) (string, error) {
	website = strings.TrimSpace(website)
	if website == "" {
		return "", errors.NewValidationError("--website is required", nil)
	}

	invalid := func(reason string) error {
		return errors.NewValidationError(fmt.Sprintf("invalid website: %s (%s; use a domain such as example.com)", website, reason), nil)
	}

	host := website
	if strings.Contains(website, "://") {
		parsed, err := url.Parse(website)
		if err != nil {
			return "", invalid("not a valid URL")
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return "", invalid("only http and https URLs are accepted")
		}
		if parsed.User != nil {
			return "", invalid("must not include credentials")
		}
		if parsed.Port() != "" {
			return "", invalid("must not include a port")
		}
		if (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
			return "", invalid("must not include a path, query, or fragment")
		}
		host = parsed.Hostname()
	}

	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if validation.ValidateDomainName(host) != nil {
		return "", invalid("not a valid domain")
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return "", invalid("must be a fully qualified domain")
	}
	if tld := labels[len(labels)-1]; tld[0] < 'a' || tld[0] > 'z' {
		return "", invalid("must end in a valid top-level domain")
	}

	return host, nil
}

// validateMonthlyCredit enforces the 0..1000000000 range locally, before auth.
func validateMonthlyCredit(credit int64) error {
	if credit < monthlyCreditMin || credit > monthlyCreditMax {
		return errors.NewValidationError(fmt.Sprintf("invalid monthly credit: %d (must be between %d and %d)", credit, monthlyCreditMin, monthlyCreditMax), nil)
	}
	return nil
}
