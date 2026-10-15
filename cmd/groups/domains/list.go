package domains

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewListCommand creates the list command
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List domains",
		Long: `List all domains in your AhaSend account with their verification status,
DNS record status, and other details.

The list can be filtered and paginated for large numbers of domains.`,
		Example: `  # List all domains
  ahasend domains list

  # List domains with JSON output
  ahasend domains list --output json

  # List domains with pagination
  ahasend domains list --limit 10

  # Filter by DNS status
  ahasend domains list --status verified

  # List only marketing domains
  ahasend domains list --sending-type marketing`,
		Args:         cmdutil.NoArgs,
		RunE:         runDomainsList,
		SilenceUsage: true,
	}

	cmdutil.AddPaginationFlags(cmd)
	cmdutil.AddLegacyCursorAlias(cmd)
	cmdutil.AddOneOfFlag(cmd, "status", "", "Filter by DNS status: %s", dnsStatuses)
	cmdutil.AddOneOfFlag(cmd, "sending-type", "", "Filter by sending type: %s", sendingTypes)

	return cmd
}

func runDomainsList(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	// Get flags
	pagination, err := cmdutil.ReadPaginationParams(cmd)
	if err != nil {
		return err
	}
	status, _ := cmd.Flags().GetString("status")
	sendingType, err := cmdutil.ReadFilterOneOf(cmd, "sending-type")
	if err != nil {
		return err
	}

	// Handle pagination and filters
	params := requests.GetDomainsParams{PaginationParams: pagination}
	if status != "" {
		dnsValid, err := dnsValidForStatus(cmd, status)
		if err != nil {
			return err
		}
		params.DNSValid = &dnsValid
	}
	params.SendingType = sendingType

	// Only authenticate after local validation passes
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"status":       status,
		"sending_type": sendingType,
	}).Debug("Executing domains list command")

	// Fetch domains
	response, err := client.ListDomains(params)
	if err != nil {
		return err
	}

	emptyMessage := "No domains found"
	if status != "" {
		emptyMessage = fmt.Sprintf("No domains found with status '%s'", status)
	}

	// Handle successful domains list response
	config := printer.ListConfig{
		SuccessMessage: "Domains retrieved successfully",
		EmptyMessage:   emptyMessage,
	}

	return handler.HandleDomainList(response, config)
}

// dnsStatuses are the values of --status.
var dnsStatuses = []string{"verified", "pending", "failed"}

// dnsValidForStatus maps a --status value to the API's dns_valid filter.
// "pending" and "failed" both select domains whose DNS is not valid.
func dnsValidForStatus(cmd *cobra.Command, status string) (bool, error) {
	status, err := cmdutil.MatchOneOf(cmd, "status", status)
	if err != nil {
		return false, err
	}
	return status == "verified", nil
}
