package domains

import (
	"fmt"

	"github.com/AhaSend/ahasend-go/models/responses"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewVerifyCommand creates the verify command
func NewVerifyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify <domain>",
		Short: "Show a domain's DNS status",
		Long: `Show the DNS status of a domain as of its last DNS check, with
troubleshooting information. It does not check the DNS again: run
"ahasend domains check-dns" for a new check after changing the records.

The command shows whether DNS records are properly configured and lists them.
When the DNS is not valid, troubleshooting tips and setup instructions for
each record follow on stderr. --show-records prints the setup instructions
for a valid domain too.`,
		Example: `  # Show the domain's DNS status
  ahasend domains verify example.com

  # Also print the DNS setup instructions of a valid domain
  ahasend domains verify example.com --show-records`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runDomainsVerify,
		SilenceUsage: true,
	}

	cmd.Flags().Bool("show-records", false, "Print the DNS setup instructions even when the DNS is valid")

	return cmd
}

func runDomainsVerify(cmd *cobra.Command, args []string) error {
	return runDNSCheck(cmd, args[0], client.AhaSendClient.GetDomain)
}

// runDNSCheck fetches domain with fetch and prints it under a line that says
// whether its DNS is valid, followed by the DNS help on stderr. verify and
// check-dns differ only in how they fetch the domain.
func runDNSCheck(cmd *cobra.Command, domain string, fetch func(client.AhaSendClient, string) (*responses.Domain, error)) error {
	if err := validation.ValidatePathSegment("domain", domain); err != nil {
		return err
	}
	showRecords, _ := cmd.Flags().GetBool("show-records")

	handler := printer.GetResponseHandlerFromCommand(cmd)
	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"domain":       domain,
		"show_records": showRecords,
	}).Debug("Executing domain " + cmd.Name() + " command")

	response, err := fetch(apiClient, domain)
	if err != nil {
		return err
	}

	successMessage := fmt.Sprintf("Domain '%s' DNS configuration needs attention", domain)
	if response.DNSValid {
		successMessage = fmt.Sprintf("Domain '%s' DNS is properly configured", domain)
	}
	if err := handler.HandleSingleDomain(response, printer.SingleConfig{
		SuccessMessage: successMessage,
		EmptyMessage:   "Domain not found",
	}); err != nil {
		return err
	}
	writeDNSHelp(cmd, response, showRecords)
	return nil
}
