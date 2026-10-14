package stats

import (
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
)

// NewBouncesCommand creates the stats bounces command
func NewBouncesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bounces",
		Short: "View email bounce statistics and analysis",
		Long: `View email bounce statistics by classification for each time period.

Bounce statistics show how many messages bounced, categorized by specific
bounce reasons, to help identify and troubleshoot delivery issues faster.

Bounce classifications include:
- AuthenticationFailed: Message rejected due to DMARC or authentication issues
- BadConnection: The connection to the recipient server or its TLS failed
- BadDomain: The recipient domain doesn't exist
- DNSFailure: The domain's MX record is invalid
- InactiveMailbox: The mailbox provider has deactivated the email address
- InvalidRecipient: The email address doesn't exist
- PolicyRelated: Blocked due to recipient server policies (spam/blocklists)
- ProtocolErrors: SMTP communication issues with recipient mail server
- QuotaIssues: The recipient's mailbox is full
- RoutingErrors: The recipient mail server couldn't route the email
- TransientFailure: The recipient server temporarily rejected the message
- Uncategorized: Other bounce types not specifically categorized

The API may return other classifications too; the output shows each as the
API names it.`,
		Example: `  # View bounces of the last 7 days
  ahasend stats bounces --from-time 7d

  # Export the data to CSV for further analysis
  ahasend stats bounces --from-time 30d --output csv

  # View bounces with hourly grouping
  ahasend stats bounces --from-time 24h --group-by hour

  # Filter by sender domain
  ahasend stats bounces --sender-domain example.com --from-time 7d`,
		Args:         cmdutil.NoArgs,
		RunE:         runBounceStats,
		SilenceUsage: true,
	}

	addFilterFlags(cmd)
	for _, name := range []string{"raw", "classification", "trends", "show-domains", "show-totals"} {
		addNoEffectFlag(cmd, name)
	}

	return cmd
}

func runBounceStats(cmd *cobra.Command, args []string) error {
	return runStats(cmd, "Bounce Statistics", func(c client.AhaSendClient, filters statsFilters) (*responses.BounceStatisticsResponse, error) {
		return c.GetBounceStatistics(requests.GetBounceStatisticsParams(filters))
	}, printer.ResponseHandler.HandleBounceStats)
}
