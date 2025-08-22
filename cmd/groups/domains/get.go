package domains

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the get command
func NewGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <domain>",
		Short: "Get detailed information about a domain",
		Long: `Get detailed information about a specific domain including DNS records,
verification status, and last verification check time.

This command shows complete domain configuration and status.`,
		Example: `  # Get domain details
  ahasend domains get example.com

  # Get domain details with JSON output
  ahasend domains get example.com --output json`,
		Args:         cobra.ExactArgs(1),
		RunE:         runDomainsGet,
		SilenceUsage: true,
	}

	return cmd
}

func runDomainsGet(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	domain := args[0]

	logger.Get().WithFields(map[string]interface{}{
		"domain": domain,
	}).Debug("Executing domain get command")

	// Get domain details
	response, err := client.GetDomain(domain)
	if err != nil {
		return handler.HandleError(err)
	}

	if response == nil {
		return handler.HandleError(errors.NewNotFoundError(fmt.Sprintf("domain '%s' not found", domain), nil))
	}

	// Handle successful domain response
	config := printer.SingleConfig{
		SuccessMessage: fmt.Sprintf("Domain details for '%s'", domain),
		EmptyMessage:   "Domain not found",
		FieldOrder:     []string{"domain", "id", "dns_valid", "created_at", "updated_at", "last_dns_check_at"},
	}

	return handler.HandleSingleDomain(response, config)
}
