package smtp

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// NewListCommand creates the smtp list command
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all SMTP credentials",
		Long: `List all SMTP credentials in your account with pagination support.

SMTP credentials are displayed with their name, username, scope, and creation date.
Passwords are never shown for security reasons. Use pagination flags to navigate
through large lists of credentials.`,
		Example: `  # List all SMTP credentials
  ahasend smtp list

  # List with pagination
  ahasend smtp list --limit 10

  # Continue from cursor
  ahasend smtp list --cursor "next-page-token"

  # Export to JSON
  ahasend smtp list --output json`,
		RunE: runSMTPList,
	}

	// Add flags
	cmd.Flags().Int32("limit", 50, "Maximum number of credentials to return (1-100)")
	cmd.Flags().String("cursor", "", "Pagination cursor for continued results")

	return cmd
}

func runSMTPList(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Get flag values
	limit, _ := cmd.Flags().GetInt32("limit")
	cursor, _ := cmd.Flags().GetString("cursor")

	// Validate limit
	if limit < 1 || limit > 100 {
		return errors.NewValidationError("limit must be between 1 and 100", nil)
	}

	logger.Get().WithFields(map[string]interface{}{
		"limit":  limit,
		"cursor": cursor,
	}).Debug("Listing SMTP credentials")

	// Build request parameters
	limitPtr := &limit
	var cursorPtr *string
	if cursor != "" {
		cursorPtr = &cursor
	}

	// List SMTP credentials
	response, err := client.ListSMTPCredentials(limitPtr, cursorPtr)
	if err != nil {
		return err
	}

	if response == nil {
		return errors.NewAPIError("received nil response from API", nil)
	}

	// Use the new ResponseHandler to display SMTP credentials list
	return handler.HandleSMTPList(response, printer.ListConfig{
		SuccessMessage: "SMTP credentials retrieved successfully",
		EmptyMessage:   "No SMTP credentials found",
		ShowPagination: true,
		FieldOrder:     []string{"id", "name", "username", "scope", "domains", "sandbox", "created_at", "updated_at"},
	})
}
