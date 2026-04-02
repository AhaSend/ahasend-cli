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

// NewCheckDNSCommand creates the check-dns command
func NewCheckDNSCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-dns <domain>",
		Short: "Trigger a DNS validation check for a domain",
		Long: `Trigger a fresh DNS validation check for a domain. If the domain was checked
within the last 60 seconds, the cached result is returned instead of performing
a new lookup.

This is useful after making DNS changes to quickly verify that records have propagated.`,
		Example: `  # Check DNS for a domain
  ahasend domains check-dns example.com

  # Check DNS and show detailed records
  ahasend domains check-dns example.com --verbose`,
		Args:         cobra.ExactArgs(1),
		RunE:         runDomainsCheckDNS,
		SilenceUsage: true,
	}

	cmd.Flags().Bool("verbose", false, "Show detailed DNS information")

	return cmd
}

func runDomainsCheckDNS(cmd *cobra.Command, args []string) error {
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
	}).Debug("Executing domain check-dns command")

	response, err := client.CheckDomainDNS(domain)
	if err != nil {
		return err
	}

	if response == nil {
		return errors.NewNotFoundError(fmt.Sprintf("domain '%s' not found", domain), nil)
	}

	if !response.DNSValid {
		fmt.Println("💡 Troubleshooting tips:")
		fmt.Println("• DNS propagation can take up to 48 hours")
		fmt.Println("• Check that DNS records are configured exactly as shown")
		fmt.Println("• Use 'dig' or online DNS lookup tools to verify record propagation")
		fmt.Printf("• Run 'ahasend domains get %s --show-dns-records' to see required records\n", domain)

		recordSet := dns.FormatDNSRecords(response)
		dns.PrintDNSInstructions(recordSet)
		fmt.Println()
	}

	if verbose && response.DNSValid {
		recordSet := dns.FormatDNSRecords(response)
		if len(recordSet.Records) > 0 {
			fmt.Println("📋 DNS Records Details:")
			dns.PrintDNSInstructions(recordSet)
		}
	}

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
