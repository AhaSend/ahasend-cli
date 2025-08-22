package domains

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/dns"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewCreateCommand creates the create command
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <domain>",
		Short: "Create a new domain for email sending",
		Long: `Create a new domain in your AhaSend account for email sending.
After creating the domain, you'll need to configure DNS records and verify the domain.

The domain must be a valid domain name that you own and can configure DNS records for.`,
		Example: `  # Create a domain interactively
  ahasend domains create example.com

  # Create a domain with DNS record format output
  ahasend domains create example.com --format bind

  # Skip DNS instructions
  ahasend domains create example.com --no-dns-help`,
		Args:         cobra.MaximumNArgs(1),
		RunE:         runDomainsCreate,
		SilenceUsage: true,
	}

	cmd.Flags().String("format", "", "DNS record format (bind, cloudflare, terraform)")
	cmd.Flags().Bool("no-dns-help", false, "Skip DNS configuration instructions")

	return cmd
}

func runDomainsCreate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString("format")
	noDNSHelp, _ := cmd.Flags().GetBool("no-dns-help")

	// Get domain name
	var domain string
	if len(args) > 0 {
		domain = args[0]
		logger.Get().WithField("domain", domain).Debug("Using domain from arguments")
	} else {
		domain, err = promptDomainName()
		if err != nil {
			return handler.HandleError(errors.NewValidationError("failed to read domain name", err))
		}
		logger.Get().WithField("domain", domain).Debug("Using domain from prompt")
	}

	// Validate domain name
	logger.Get().WithField("domain", domain).Debug("Validating domain name")
	if err := validation.ValidateDomainName(domain); err != nil {
		return handler.HandleError(err)
	}

	logger.Get().WithFields(map[string]interface{}{
		"domain":      domain,
		"format":      format,
		"no_dns_help": noDNSHelp,
	}).Debug("Executing domain add command")

	// Create the domain
	response, err := client.CreateDomain(domain)
	if err != nil {
		return handler.HandleError(err)
	}

	// Handle successful domain creation
	// Note: DNS instructions will be handled by the printer implementation
	// The specific format and no-dns-help flags will need to be handled differently
	// For now, we'll use the SingleConfig to pass the domain
	config := printer.SingleConfig{
		SuccessMessage: fmt.Sprintf("Domain '%s' added successfully", domain),
		EmptyMessage:   "No domain created",
		FieldOrder:     []string{"domain", "id", "dns_valid", "created_at", "updated_at"},
	}

	// Show DNS configuration instructions unless disabled
	// TODO: This logic should ideally be moved to the printer implementations
	// to maintain format consistency, but for now keeping it here
	if !noDNSHelp && response != nil {
		recordSet := dns.FormatDNSRecords(response)

		if format == "" {
			dns.PrintDNSInstructions(recordSet)
		} else {
			// Print records in specific format
			fmt.Printf("\nDNS Records (%s format):\n", strings.ToUpper(format))
			fmt.Println(strings.Repeat("-", 40))

			for _, record := range recordSet.Records {
				fmt.Println(dns.FormatDNSRecordForProvider(record, format))
			}
			fmt.Println()
		}
	}

	return handler.HandleSingleDomain(response, config)
}

func promptDomainName() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter domain name: ")

	domain, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(domain), nil
}
