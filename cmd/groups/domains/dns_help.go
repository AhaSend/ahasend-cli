package domains

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/dns"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
)

// writeDNSHelp writes troubleshooting tips and the setup instructions of the
// DNS records domain needs to stderr when its DNS is not valid, and the setup
// instructions alone when it is valid and showRecords is set. JSON output
// skips it: the API body already holds the records, and stdout carries
// nothing else.
func writeDNSHelp(cmd *cobra.Command, domain *responses.Domain, showRecords bool) {
	if printer.GetResponseHandlerFromCommand(cmd).GetFormat() == "json" {
		return
	}
	out := printer.DisplayWriter(cmd.ErrOrStderr())
	recordSet := dns.FormatDNSRecords(domain)

	if !domain.DNSValid {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Troubleshooting tips:")
		fmt.Fprintln(out, "• DNS propagation can take up to 48 hours")
		fmt.Fprintln(out, "• Check that DNS records are configured exactly as shown")
		fmt.Fprintln(out, "• Use 'dig' or online DNS lookup tools to verify record propagation")
		fmt.Fprintf(out, "• Run 'ahasend domains check-dns %s' to check the records again\n", domain.Domain)
		dns.PrintDNSInstructions(out, recordSet)
		return
	}

	if showRecords && len(recordSet.Records) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "DNS Records Details:")
		dns.PrintDNSInstructions(out, recordSet)
	}
}
