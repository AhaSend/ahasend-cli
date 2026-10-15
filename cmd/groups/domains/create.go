package domains

import (
	"fmt"
	"io"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/dns"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewCreateCommand creates the create command
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [domain]",
		Short: "Create a domain for email sending",
		Long: `Create a domain in your AhaSend account for email sending. After creating
the domain, configure its DNS records and verify it.

The domain must be a valid domain name that you own and can configure DNS
records for. When the domain argument is missing and stdin is a terminal,
the command asks for it; --non-interactive never asks.

Use --sending-type to create a marketing domain (the default is
transactional). The subdomain flags and --dkim-rotation-interval set what
domains update sets; --dkim-selector and --dkim-private-key-file set a
custom DKIM selector and key (Platform Partner accounts only).

The records the domain needs go to stderr: as instructions, or in the
provider format --format names. Table output puts a header line above the
records; plain and CSV output write the records alone, so
'--format bind --output plain 2> example.com.zone' saves just the records.
--format cannot be used with --no-dns-help or --output json, which print no
records.`,
		Example: `  # Create a domain
  ahasend domains create example.com

  # Create a domain with DNS record format output
  ahasend domains create example.com --format bind

  # Skip DNS instructions
  ahasend domains create example.com --no-dns-help

  # Create a marketing domain with a custom DKIM selector
  ahasend domains create news.example.com --sending-type marketing --dkim-selector news

  # Create a domain with custom tracking and return-path subdomains
  ahasend domains create example.com --tracking-subdomain click --return-path-subdomain bounce`,
		Args:         cmdutil.MaximumNArgs(1),
		RunE:         runDomainsCreate,
		SilenceUsage: true,
	}

	cmdutil.AddOneOfFlag(cmd, "format", "", "DNS record format: %s (terraform for Cloudflare provider v5); not with --no-dns-help or --output json", dns.Formats)
	cmd.Flags().Bool("no-dns-help", false, "Skip DNS configuration instructions")
	cmdutil.AddOneOfFlag(cmd, "sending-type", "", "Sending type: %s (default transactional)", sendingTypes)
	cmd.Flags().String("dkim-selector", "", "Custom DKIM selector (Platform Partner accounts only)")
	cmd.Flags().String("dkim-private-key-file", "", "File with the domain's DKIM private key, an RSA key in PEM form of at least 1024 bits (Platform Partner accounts only)")
	cmdutil.MarkFileFlag(cmd, "dkim-private-key-file")
	addSettingsFlags(cmd)
	cmdutil.AddIdempotencyKeyFlag(cmd)
	prompt.AddNonInteractiveFlag(cmd)

	return cmd
}

func runDomainsCreate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	settings, err := readSettings(cmd)
	if err != nil {
		return err
	}
	dkimPrivateKey, err := readDKIMPrivateKey(cmd)
	if err != nil {
		return err
	}
	format, _ := cmd.Flags().GetString("format")
	if format != "" {
		if format, err = cmdutil.MatchOneOf(cmd, "format", format); err != nil {
			return err
		}
	}
	if err := cmdutil.RejectTogether(cmd, "format", "no-dns-help"); err != nil {
		return err
	}
	if format != "" && handler.GetFormat() == "json" {
		return errors.NewValidationError("--format and --output json cannot be used together: JSON output prints the API's response unchanged", nil)
	}
	noDNSHelp, _ := cmd.Flags().GetBool("no-dns-help")

	// Get domain name
	var domain string
	if len(args) > 0 {
		domain = args[0]
	}
	if domain, err = cmdutil.AskIfEmpty(cmd, domain, "Domain name", "the domain argument is required"); err != nil {
		return err
	}

	// Validate domain name
	logger.Get().WithField("domain", domain).Debug("Validating domain name")
	if err := validation.ValidateDomainName("domain", domain); err != nil {
		return err
	}

	idempotency, err := cmdutil.IdempotencyOptions(cmd)
	if err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"domain":      domain,
		"format":      format,
		"no_dns_help": noDNSHelp,
	}).Debug("Executing domain add command")

	req := requests.CreateDomainRequest{
		Domain:                   domain,
		DKIMPrivateKey:           dkimPrivateKey,
		TrackingSubdomain:        settings.TrackingSubdomain,
		ReturnPathSubdomain:      settings.ReturnPathSubdomain,
		SubscriptionSubdomain:    settings.SubscriptionSubdomain,
		MediaSubdomain:           settings.MediaSubdomain,
		DKIMRotationIntervalDays: settings.DKIMRotationIntervalDays,
		DKIMSelector:             settings.DKIMSelector,
		SendingType:              settings.SendingType,
	}

	// Create the domain
	response, err := client.CreateDomain(req, idempotency...)
	if err != nil {
		return err
	}

	config := printer.SingleConfig{
		SuccessMessage: "Domain created successfully",
		EmptyMessage:   "No domain created",
	}
	if err := handler.HandleSingleDomain(response, config); err != nil {
		return err
	}

	// DNS configuration instructions are help text: they go to stderr, and
	// JSON output skips them because the API body holds the records.
	if !noDNSHelp && response != nil && handler.GetFormat() != "json" {
		writeCreateDNSHelp(printer.DisplayWriter(cmd.ErrOrStderr()), dns.FormatDNSRecords(response), format, handler.GetFormat() == "table")
	}
	return nil
}

// writeCreateDNSHelp writes the records a new domain needs to w: as
// instructions, or in the provider format given by --format, each record
// after a comment that marks it optional or says how to add it, under a
// header when header is set.
func writeCreateDNSHelp(w io.Writer, recordSet *dns.DNSRecordSet, format string, header bool) {
	if format == "" {
		dns.PrintDNSInstructions(w, recordSet)
		return
	}
	if header {
		fmt.Fprintf(w, "\nDNS Records (%s format):\n", strings.ToUpper(format))
		fmt.Fprintln(w, strings.Repeat("-", 40))
	}
	for i, line := range dns.FormatDNSRecordsForProvider(recordSet.Records, format) {
		if comment := dns.ProviderComment(recordSet.Records[i], format); comment != "" {
			fmt.Fprintln(w, comment)
		}
		fmt.Fprintln(w, line)
	}
}

// sendingTypes are the values of --sending-type.
var sendingTypes = []string{"transactional", "marketing"}
