package campaigns

import (
	"fmt"
	"net/mail"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
)

// NewTestCommand creates the campaigns test command.
func NewTestCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test <campaign-id> --to <address>",
		Short: "Send a test of a draft campaign",
		Long: `Send a test of a draft campaign to up to 5 addresses, with placeholders for
the contact's fields. Addresses that differ only in case count once. --to can
be used multiple times and splits on commas; it takes an address with or
without a display name. A display name with a comma must be in double quotes,
and the whole value too, with the inner quotes doubled, so the comma does not
split it: --to '"""Doe, Jane"" <jane@example.com>"'.

The output lists the addresses the mail server accepted and the ones it
refused, with the reason, and the command exits 1 when any address was
refused. When the server's answer could not be read, the test may have gone
out: check the inboxes before sending another. A campaign with issues is
refused, and each issue is listed.

The API caps the test emails of an account at 50 a UTC day, one per
address of each test, and refuses a test that would pass the cap with
"daily test send limit reached" and exit code 8. The command does not send
such a test again.`,
		Example: `  # Send a test to yourself
  ahasend campaigns test 123e4567-e89b-12d3-a456-426614174000 --to me@example.com

  # Send a test to a named address and a team inbox
  ahasend campaigns test 123e4567-e89b-12d3-a456-426614174000 --to '"""Doe, Jane"" <jane@example.com>"' --to team@example.com`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runCampaignsTest,
		SilenceUsage: true,
	}

	cmdutil.AddRepeatableFlag(cmd, "to", "Address to send the test to (required; can be used multiple times)")
	cmdutil.AddIdempotencyKeyFlag(cmd)
	return cmd
}

func runCampaignsTest(cmd *cobra.Command, args []string) error {
	campaignID, err := validation.ParseUUIDField("campaign ID", args[0])
	if err != nil {
		return err
	}
	to, err := cmdutil.ReadRepeatable(cmd, "to", "")
	if err != nil {
		return err
	}
	if to == nil {
		return errors.NewValidationError("--to is required", nil)
	}
	if err := checkTestAddresses(to); err != nil {
		return err
	}
	if count := countTestAddresses(to); count > maxTestAddresses {
		return errors.NewValidationError(fmt.Sprintf("--to takes at most %d addresses, got %d", maxTestAddresses, count), nil)
	}

	request := requests.TestCampaignRequest{To: to}

	idempotency, err := cmdutil.IdempotencyOptions(cmd)
	if err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"campaign_id": args[0],
		"recipients":  len(to),
	}).Debug("Executing campaigns test command")

	test, err := apiClient.TestCampaign(campaignID, request, idempotency...)
	if err != nil {
		return campaignNotReadyError(cmd, err, "test")
	}
	if err := printer.GetResponseHandlerFromCommand(cmd).HandleCampaignTest(test, printer.SingleConfig{
		SuccessMessage: campaignTestSummary(test),
		EmptyMessage:   "No test result returned",
	}); err != nil {
		return err
	}
	if test != nil && len(test.Failed) > 0 {
		return errors.NewItemsFailedError(len(test.Failed), len(test.SentTo)+len(test.Failed), "addresses")
	}
	return nil
}

// maxTestAddresses is the most addresses a campaign test goes to.
const maxTestAddresses = 5

// checkTestAddresses checks that each --to entry is an address, with or
// without a display name, as the API parses it.
func checkTestAddresses(to []string) error {
	for _, entry := range to {
		if _, err := mail.ParseAddress(entry); err != nil {
			return validation.InvalidValueError("--to", entry,
				`not an email address; a display name with a comma must be in double quotes, such as "Doe, Jane" <jane@example.com>`)
		}
	}
	return nil
}

// countTestAddresses counts the --to addresses, which checkTestAddresses
// checked, as the API does: addresses that differ only in case count once,
// and a display name does not count.
func countTestAddresses(to []string) int {
	addresses := map[string]bool{}
	for _, entry := range to {
		if parsed, err := mail.ParseAddress(entry); err == nil {
			addresses[strings.ToLower(parsed.Address)] = true
		}
	}
	return len(addresses)
}

// campaignTestSummary is the first line of the test result, which says
// whether the mail server accepted every address, some or none.
func campaignTestSummary(test *responses.CampaignTest) string {
	if test == nil || len(test.Failed) == 0 {
		return "Campaign test sent"
	}
	if len(test.SentTo) == 0 {
		return "Campaign test failed for every address"
	}
	return fmt.Sprintf("Campaign test sent to %d of %d addresses", len(test.SentTo), len(test.SentTo)+len(test.Failed))
}
