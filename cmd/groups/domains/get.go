package domains

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the get command
func NewGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <domain>",
		Short: "Get a domain",
		Long: `Get detailed information about a specific domain including DNS records,
verification status, and last verification check time.

This command shows complete domain configuration and status.`,
		Example: `  # Get domain details
  ahasend domains get example.com

  # Get domain details with JSON output
  ahasend domains get example.com --output json`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runDomainsGet,
		SilenceUsage: true,
	}

	return cmd
}

func runDomainsGet(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	domain := args[0]
	if err := validation.ValidatePathSegment("domain", domain); err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"domain": domain,
	}).Debug("Executing domain get command")

	// Get domain details
	response, err := client.GetDomain(domain)
	if err != nil {
		return err
	}

	// Handle successful domain response
	config := printer.SingleConfig{
		SuccessMessage: "Domain retrieved successfully",
		EmptyMessage:   "Domain not found",
	}

	return handler.HandleSingleDomain(response, config)
}
