package smtp

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the smtp get command
func NewGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <credential-id>",
		Short: "Get details of a specific SMTP credential",
		Long: `Get detailed information about a specific SMTP credential.

Shows all credential details including name, username, scope, domains,
and timestamps. Note that passwords are never displayed for security reasons.`,
		Example: `  # Get SMTP credential details
  ahasend smtp get 550e8400-e29b-41d4-a716-446655440000

  # Get as JSON
  ahasend smtp get 550e8400-e29b-41d4-a716-446655440000 --output json`,
		Args: cobra.ExactArgs(1),
		RunE: runSMTPGet,
	}

	return cmd
}

func runSMTPGet(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	credentialID := args[0]

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"credential_id": credentialID,
	}).Debug("Getting SMTP credential details")

	// Get SMTP credential
	credential, err := client.GetSMTPCredential(credentialID)
	if err != nil {
		return handler.HandleError(err)
	}

	if credential == nil {
		return handler.HandleError(errors.NewAPIError("received nil response from API", nil))
	}

	// Use the new ResponseHandler to display SMTP credential details
	return handler.HandleSingleSMTP(credential, printer.SingleConfig{
		SuccessMessage: fmt.Sprintf("SMTP credential details for %s", credentialID),
		EmptyMessage:   "SMTP credential not found",
		FieldOrder:     []string{"id", "name", "username", "scope", "domains", "sandbox", "created_at", "updated_at"},
	})
}
