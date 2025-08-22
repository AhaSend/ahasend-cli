package suppressions

import (
	"fmt"
	"regexp"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/output"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewCreateCommand creates the suppressions create command
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <email>",
		Short: "Create a new suppression for an email address",
		Long: `Create a new suppression entry to prevent sending emails to an address.

This command creates a new suppression entry for the specified email address.
You can specify a reason for suppression (up to 255 characters) and must set an expiration time.

Use --domain to create domain-specific suppressions.
Without --domain, creates a global suppression for all domains.

The --expires flag is required and can accept:
- Relative time: 30d, 24h, 1w, 3mo, 1y
- Absolute time: 2024-12-31T23:59:59Z`,
		Example: `  # Create global suppression with reason that expires in 30 days
  ahasend suppressions create user@example.com --reason "User requested unsubscribe" --expires 30d

  # Create domain-specific suppression that expires in 1 year  
  ahasend suppressions create user@example.com --domain mydomain.com --reason "Email bounced" --expires 1y

  # Create suppression with specific expiration date
  ahasend suppressions create user@example.com --reason "Holiday pause" --expires 2024-12-31T23:59:59Z

  # Create suppression with JSON output
  ahasend suppressions create user@example.com --reason "Manually added" --expires 90d --output json`,
		Args: cobra.ExactArgs(1),
		RunE: runSuppressionsCreate,
	}

	// Add flags
	cmd.Flags().String("reason", "", "Suppression reason (up to 255 characters)")
	cmd.Flags().String("domain", "", "Domain for domain-specific suppression (optional)")
	cmd.Flags().String("expires", "", "Expiration time (e.g., '30d', '2024-12-31T23:59:59Z') [required]")
	cmd.MarkFlagRequired("expires")

	return cmd
}

func runSuppressionsCreate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	email := args[0]

	// Validate email format
	if !isValidEmail(email) {
		return handler.HandleError(errors.NewValidationError(fmt.Sprintf("invalid email format: %s", email), nil))
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Get flag values
	reason, _ := cmd.Flags().GetString("reason")
	domain, _ := cmd.Flags().GetString("domain")
	expiresStr, _ := cmd.Flags().GetString("expires")

	// Validate reason length (per OpenAPI spec: maxLength 255)
	if len(reason) > 255 {
		return handler.HandleError(errors.NewValidationError(fmt.Sprintf("reason exceeds maximum length of 255 characters (got %d characters)", len(reason)), nil))
	}

	// Parse expiration time (required)
	parsedTime, err := output.ParseTimeFuture(expiresStr)
	if err != nil {
		return handler.HandleError(err)
	}
	expiresAt := parsedTime

	// Prepare suppression request
	req := requests.CreateSuppressionRequest{
		Email:     email,
		ExpiresAt: expiresAt, // Zero time for permanent, or user-specified time
	}

	if reason != "" {
		req.Reason = &reason
	}

	if domain != "" {
		req.Domain = &domain
	}

	logger.Get().WithFields(map[string]interface{}{
		"email":      email,
		"reason":     reason,
		"domain":     domain,
		"expires_at": expiresAt,
	}).Debug("Adding suppression")

	// Create suppression
	response, err := client.CreateSuppression(req)
	if err != nil {
		return handler.HandleError(err)
	}

	// Use the new ResponseHandler to display created suppression
	return handler.HandleCreateSuppression(response, printer.CreateConfig{
		SuccessMessage: fmt.Sprintf("Suppression added successfully for %s", email),
		ItemName:       "suppression",
		FieldOrder:     []string{"email", "domain", "reason", "created_at", "expires_at"},
	})
}

// isValidEmail validates email format using regex
func isValidEmail(email string) bool {
	// Basic email validation regex
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}
