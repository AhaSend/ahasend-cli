package domains

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/dns"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// NewVerifyCommand creates the verify command
func NewVerifyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify <domain>",
		Short: "Check domain DNS configuration",
		Long: `Check the DNS configuration status for a domain and provide troubleshooting information.

This command shows whether DNS records are properly configured and provides
helpful guidance for fixing DNS issues.`,
		Example: `  # Check domain DNS status
  ahasend domains verify example.com

  # Show detailed DNS information
  ahasend domains verify example.com --verbose`,
		Args:         cobra.ExactArgs(1),
		RunE:         runDomainsVerify,
		SilenceUsage: true,
	}

	cmd.Flags().Bool("verbose", false, "Show detailed DNS information")

	return cmd
}

func runDomainsVerify(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	domain := args[0]
	verbose, _ := cmd.Flags().GetBool("verbose")

	logger.Get().WithFields(map[string]interface{}{
		"domain":  domain,
		"verbose": verbose,
	}).Debug("Executing domain verify command")

	// Get current domain status
	response, err := client.GetDomain(domain)
	if err != nil {
		return err
	}

	if response == nil {
		return errors.NewNotFoundError(fmt.Sprintf("domain '%s' not found", domain), nil)
	}

	// Show troubleshooting tips and DNS configuration instructions
	// TODO: This logic should ideally be moved to the printer implementations
	// to maintain format consistency, but for now keeping it here
	if !response.DNSValid {
		fmt.Println("💡 Troubleshooting tips:")
		fmt.Println("• DNS propagation can take up to 48 hours")
		fmt.Println("• Check that DNS records are configured exactly as shown")
		fmt.Println("• Use 'dig' or online DNS lookup tools to verify record propagation")
		fmt.Printf("• Run 'ahasend domains get %s --show-dns-records' to see required records\n", domain)

		// Show DNS configuration instructions
		recordSet := dns.FormatDNSRecords(response)
		dns.PrintDNSInstructions(recordSet)
		fmt.Println()
	}

	// Show DNS records if verbose mode is enabled
	if verbose && response != nil {
		recordSet := dns.FormatDNSRecords(response)
		if len(recordSet.Records) > 0 {
			fmt.Println("📋 DNS Records Details:")
			dns.PrintDNSInstructions(recordSet)
		}
	}

	// Handle successful domain verification response
	var successMessage string
	if response.DNSValid {
		successMessage = fmt.Sprintf("✅ Domain '%s' DNS is properly configured", domain)
	} else {
		successMessage = fmt.Sprintf("⚠️ Domain '%s' DNS configuration needs attention", domain)
	}

	config := printer.SingleConfig{
		SuccessMessage: successMessage,
		EmptyMessage:   "Domain not found",
		FieldOrder:     []string{"domain", "dns_valid", "created_at", "updated_at", "last_dns_check_at"},
	}

	return handler.HandleSingleDomain(response, config)
}
