package domains

import (
	"github.com/spf13/cobra"

	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
)

// NewCheckDNSCommand creates the check-dns command
func NewCheckDNSCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-dns <domain>",
		Short: "Trigger a DNS validation check for a domain",
		Long: `Trigger a fresh DNS validation check for a domain. If the domain was checked
within the last 60 seconds, the cached result is returned instead of performing
a new lookup.

Use it after DNS changes to check that the records have propagated. The
records are always listed. When the DNS is not valid, troubleshooting tips and
setup instructions for each record follow on stderr. --show-records prints the
setup instructions for a valid domain too.`,
		Example: `  # Check DNS for a domain
  ahasend domains check-dns example.com

  # Also print the DNS setup instructions of a valid domain
  ahasend domains check-dns example.com --show-records`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runDomainsCheckDNS,
		SilenceUsage: true,
	}

	cmd.Flags().Bool("show-records", false, "Print the DNS setup instructions even when the DNS is valid")

	return cmd
}

func runDomainsCheckDNS(cmd *cobra.Command, args []string) error {
	return runDNSCheck(cmd, args[0], client.AhaSendClient.CheckDomainDNS)
}
