package smtp

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// NewListCommand creates the smtp list command
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List SMTP credentials",
		Long: `List the SMTP credentials in your account, with cursor pagination.

Each credential shows its name, username, scope and creation date. Passwords
are never shown.`,
		Example: `  # List all SMTP credentials
  ahasend smtp list

  # List with pagination
  ahasend smtp list --limit 10

  # Fetch the next page
  ahasend smtp list --after "next-page-token"

  # Export to JSON
  ahasend smtp list --output json`,
		Args:         cmdutil.NoArgs,
		RunE:         runSMTPList,
		SilenceUsage: true,
	}

	// Add flags
	cmdutil.AddPaginationFlags(cmd)
	cmdutil.AddLegacyCursorAlias(cmd)

	return cmd
}

func runSMTPList(cmd *cobra.Command, args []string) error {
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

	logger.Get().Debug("Listing SMTP credentials")

	// List SMTP credentials
	response, err := client.ListSMTPCredentials(pagination)
	if err != nil {
		return err
	}

	// Use the new ResponseHandler to display SMTP credentials list
	return handler.HandleSMTPList(response, printer.ListConfig{
		SuccessMessage: "SMTP credentials retrieved successfully",
		EmptyMessage:   "No SMTP credentials found",
	})
}
