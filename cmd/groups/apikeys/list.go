package apikeys

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// NewListCommand creates the apikeys list command
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all API keys",
		Long: `List all API keys for your account with pagination support.

API keys are displayed with their ID, label, scopes, creation date, and status.
Use pagination flags to handle large numbers of keys.

The secret value of API keys is never displayed for security reasons.`,
		Example: `  # List all API keys
  ahasend apikeys list

  # List with pagination
  ahasend apikeys list --limit 10

  # Continue with pagination cursor
  ahasend apikeys list --cursor "next-page-token"

  # JSON output for automation
  ahasend apikeys list --output json`,
		RunE: runAPIKeysList,
	}

	// Pagination flags
	cmd.Flags().Int32("limit", 20, "Maximum number of API keys to return (1-100)")
	cmd.Flags().String("cursor", "", "Pagination cursor for next page")

	return cmd
}

func runAPIKeysList(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Get flags
	limit, _ := cmd.Flags().GetInt32("limit")
	cursor, _ := cmd.Flags().GetString("cursor")

	// Prepare parameters
	var limitPtr *int32
	if limit > 0 {
		limitPtr = &limit
	}

	var cursorPtr *string
	if cursor != "" {
		cursorPtr = &cursor
	}

	// Get API keys
	response, err := client.ListAPIKeys(limitPtr, cursorPtr)
	if err != nil {
		return handler.HandleError(err)
	}

	// Handle successful response
	return handler.HandleAPIKeyList(response, printer.ListConfig{
		SuccessMessage: "API Keys Retrieved Successfully",
		EmptyMessage:   "No API keys found",
		ShowPagination: true,
		FieldOrder:     []string{"id", "label", "scopes", "created_at", "updated_at"},
	})
}
