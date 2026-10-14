package suppressions

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/timeparse"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// maxReasonLength is the most characters the API accepts in a suppression's
// reason.
const maxReasonLength = 255

// NewCreateCommand creates the suppressions create command
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <email> --expires <time>",
		Short: "Create a suppression for an email address",
		Long: `Create a suppression entry to prevent sending emails to an address.

--reason (up to 255 characters) records why the address is suppressed.
Use --domain to suppress the address for one domain. Without it, the API
creates a suppression for each domain the account has now; domains added
later are not covered.

--expires is required and takes a time after now; another time fails before
the request. It accepts:
- Relative time: 30d, 24h, 1w, 3mo, 1y
- Absolute time in the future: 2027-12-31T23:59:59Z`,
		Example: `  # Suppress an address on every current domain for 30 days, with a reason
  ahasend suppressions create user@example.com --reason "User requested unsubscribe" --expires 30d

  # Create domain-specific suppression that expires in 1 year
  ahasend suppressions create user@example.com --domain mydomain.com --reason "Email bounced" --expires 1y

  # Create suppression with specific expiration date
  ahasend suppressions create user@example.com --reason "Holiday pause" --expires 2027-12-31T23:59:59Z

  # Create suppression with JSON output
  ahasend suppressions create user@example.com --reason "Manually added" --expires 90d --output json`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runSuppressionsCreate,
		SilenceUsage: true,
	}

	// Add flags
	cmd.Flags().String("reason", "", "Suppression reason (up to 255 characters)")
	cmd.Flags().String("domain", "", "Suppress the address for this domain only (default: every current domain)")
	cmd.Flags().String("expires", "", "Expiration time (e.g., '30d', '2027-12-31T23:59:59Z') (required)")
	cmdutil.AddIdempotencyKeyFlag(cmd)

	return cmd
}

func runSuppressionsCreate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	email := args[0]

	if err := validation.ValidateEmail("email", email); err != nil {
		return err
	}

	// Get flag values
	reason, _ := cmd.Flags().GetString("reason")
	domain, err := cmdutil.ReadNonEmpty(cmd, "domain", "")
	if err != nil {
		return err
	}
	if err := validation.ValidateOptionalDomainName("--domain", domain); err != nil {
		return err
	}
	expiresStr, _ := cmd.Flags().GetString("expires")
	if expiresStr == "" {
		return errors.NewValidationError("--expires is required", nil)
	}

	if err := cmdutil.CheckMaxLength("reason", &reason, maxReasonLength); err != nil {
		return err
	}

	// Parse expiration time (required)
	parsedTime, err := timeparse.ParseTimeFuture("expires", expiresStr, "30d or 1y")
	if err != nil {
		return err
	}
	expiresAt := parsedTime

	// Prepare suppression request
	req := requests.CreateSuppressionRequest{
		Email:     email,
		ExpiresAt: expiresAt, // Zero time for permanent, or user-specified time
		Domain:    domain,
	}

	if reason != "" {
		req.Reason = &reason
	}

	idempotency, err := cmdutil.IdempotencyOptions(cmd)
	if err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"email":      email,
		"reason":     reason,
		"domain":     cmd.Flag("domain").Value.String(),
		"expires_at": expiresAt,
	}).Debug("Adding suppression")

	// Create suppression
	response, err := client.CreateSuppression(req, idempotency...)
	if err != nil {
		return err
	}

	// Use the new ResponseHandler to display created suppression
	return handler.HandleCreateSuppression(response, printer.ResultConfig{
		SuccessMessage: "Suppression created successfully",
	})
}
