package apikeys

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// NewListCommand creates the apikeys list command
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List API keys",
		Long: `List all API keys for your account with pagination support.

API keys are displayed with their ID, label, scopes, creation date, and status.
Use pagination flags to handle large numbers of keys.

The secret value of API keys is never displayed for security reasons.`,
		Example: `  # List all API keys
  ahasend apikeys list

  # List with pagination
  ahasend apikeys list --limit 10

  # Fetch the next page
  ahasend apikeys list --after "next-page-token"

  # JSON output for automation
  ahasend apikeys list --output json`,
		Args:         cmdutil.NoArgs,
		RunE:         runAPIKeysList,
		SilenceUsage: true,
	}

	// Pagination flags
	cmdutil.AddPaginationFlags(cmd)
	cmdutil.AddLegacyCursorAlias(cmd)

	return cmd
}

func runAPIKeysList(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	pagination, err := cmdutil.ReadPaginationParams(cmd)
	if err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Get API keys
	response, err := client.ListAPIKeys(pagination)
	if err != nil {
		return err
	}

	// Handle successful response
	return handler.HandleAPIKeyList(response, printer.ListConfig{
		SuccessMessage: "API keys retrieved successfully",
		EmptyMessage:   "No API keys found",
	})
}
