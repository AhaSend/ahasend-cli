package suppressions

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
)

// NewListCommand creates the suppressions list command
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all suppressed email addresses",
		Long: `List suppressed email addresses with filtering and pagination support.

Suppressions are email addresses that should not receive emails from your account.
They can be filtered by email address, domain, creation time, and exported to JSON format.`,
		Example: `  # List all suppressions
  ahasend suppressions list

  # Search for specific email suppression
  ahasend suppressions list --email user@example.com

  # Filter by domain
  ahasend suppressions list --domain example.com

  # Export to JSON
  ahasend suppressions list --output json`,
		RunE: runSuppressionsList,
	}

	// Add flags
	cmd.Flags().String("email", "", "Email address to search for (optional)")
	cmd.Flags().Int32("limit", 50, "Maximum number of suppressions to return")
	cmd.Flags().String("cursor", "", "Pagination cursor for continued results")
	cmd.Flags().String("domain", "", "Filter by specific domain")

	return cmd
}

func runSuppressionsList(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Parse flags
	email, _ := cmd.Flags().GetString("email")
	limit, _ := cmd.Flags().GetInt32("limit")
	cursor, _ := cmd.Flags().GetString("cursor")
	domain, _ := cmd.Flags().GetString("domain")

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"email":  email,
		"limit":  limit,
		"cursor": cursor,
		"domain": domain,
	}).Debug("Executing suppressions list command")

	// Fetch suppressions
	response, err := fetchSuppressions(client, email, &limit, &cursor, &domain)
	if err != nil {
		return err
	}

	// Use the new ResponseHandler to display suppressions list
	emptyMessage := "No suppressions found"
	if email != "" {
		emptyMessage = "No suppressions found for the specified email address"
	}

	return handler.HandleSuppressionList(response, printer.ListConfig{
		EmptyMessage: emptyMessage,
		FieldOrder:   []string{"email", "domain", "reason", "created_at", "expires_at"},
	})
}

func fetchSuppressions(ahaSendClient client.AhaSendClient, email string, limit *int32, cursor, domain *string) (*responses.PaginatedSuppressionsResponse, error) {
	params := requests.GetSuppressionsParams{
		Limit:  limit,
		Cursor: cursor,
		Domain: domain,
	}

	// Only add email parameter if it's provided
	if email != "" {
		params.Email = &email
	}

	suppressions, err := ahaSendClient.ListSuppressions(params)
	if err != nil {
		return nil, err
	}
	return suppressions, nil
}
