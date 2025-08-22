package suppressions

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
)

// NewCheckCommand creates the suppressions check command
func NewCheckCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check <email>",
		Short: "Check if an email address is suppressed",
		Long: `Check if an email address is suppressed and cannot receive emails.

This command verifies whether a specific email address is in your suppression list.
You can check for global suppressions or domain-specific suppressions.`,
		Example: `  # Check if email is suppressed globally
  ahasend suppressions check user@example.com

  # Check if email is suppressed for specific domain
  ahasend suppressions check user@example.com --domain mydomain.com

  # Check with JSON output for automation
  ahasend suppressions check user@example.com --output json`,
		Args: cobra.ExactArgs(1),
		RunE: runSuppressionsCheck,
	}

	// Add flags
	cmd.Flags().String("domain", "", "Check suppression for specific domain only")

	return cmd
}

func runSuppressionsCheck(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	email := args[0]

	// Validate email format
	if email == "" {
		return handler.HandleError(errors.NewValidationError("email address is required", nil))
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Get flag values
	domain, _ := cmd.Flags().GetString("domain")

	var domainPtr *string
	if domain != "" {
		domainPtr = &domain
	}

	logger.Get().WithFields(map[string]interface{}{
		"email":  email,
		"domain": domain,
	}).Debug("Checking suppression status")

	// Check suppression
	suppressions, err := client.ListSuppressions(requests.GetSuppressionsParams{
		Email:  &email,
		Domain: domainPtr,
	})
	if err != nil {
		return handler.HandleError(err)
	}

	// Use the new ResponseHandler to display suppression check results
	found := len(suppressions.Data) > 0
	var suppression *responses.Suppression
	if found {
		suppression = &suppressions.Data[0]
	}

	return handler.HandleCheckSuppression(suppression, found, printer.CheckConfig{
		FoundMessage:    fmt.Sprintf("Email %s is suppressed", email),
		NotFoundMessage: fmt.Sprintf("Email %s is not suppressed", email),
		FieldOrder:      []string{"email", "domain", "reason", "created_at", "expires_at"},
	})
}
