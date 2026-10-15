package campaigns

import (
	"fmt"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/api"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// maxListIDs is the most lists a campaign sends to, counting a list given
// twice once, as the API does.
const maxListIDs = 50

// readListIDs parses the --list-id values, or returns nil when the flag is not
// set. More than maxListIDs lists is a validation error.
func readListIDs(cmd *cobra.Command) ([]uuid.UUID, error) {
	values, err := cmdutil.ReadRepeatable(cmd, "list-id", "")
	if err != nil {
		return nil, err
	}
	ids, err := validation.ParseUUIDs("list ID", values)
	if err != nil {
		return nil, err
	}
	distinct := map[uuid.UUID]bool{}
	for _, id := range ids {
		distinct[id] = true
	}
	if len(distinct) > maxListIDs {
		return nil, errors.NewValidationError(fmt.Sprintf("--list-id takes at most %d lists, got %d", maxListIDs, len(distinct)), nil)
	}
	return ids, nil
}

// campaignNotReadyError replaces the error of a send or test that the API
// refused because the campaign has issues. Outside JSON mode it returns an
// error that lists each issue as "code: message" on its own line, after a line
// that starts with "failed to <action> campaign", and exits 4 like the API's
// 400 it replaces. JSON mode and every other error keep the SDK error, so the
// raw API body prints.
func campaignNotReadyError(cmd *cobra.Command, err error, action string) error {
	notReady, ok := api.AsCampaignNotReady(err)
	if !ok || printer.GetResponseHandlerFromCommand(cmd).GetFormat() == "json" {
		return err
	}
	// Each issue is escaped on its own line, so a line break in an issue
	// cannot show as another issue
	lines := []string{fmt.Sprintf("failed to %s campaign: %s", action, printer.EscapeControl(notReady.Message))}
	for _, issue := range notReady.Issues {
		lines = append(lines, printer.EscapeControl(issue.Code)+": "+printer.EscapeControl(issue.Message))
	}
	return errors.NewValidationError(strings.Join(lines, "\n"), nil)
}

// showCampaign fetches what fetch returns for the campaign rawID, the
// argument of the campaigns command name, and prints it with print.
func showCampaign[R any](
	cmd *cobra.Command, rawID, name string,
	fetch func(client.AhaSendClient, uuid.UUID) (*R, error),
	print func(printer.ResponseHandler, *R, printer.SingleConfig) error,
	config printer.SingleConfig,
) error {
	campaignID, err := validation.ParseUUIDField("campaign ID", rawID)
	if err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithField("campaign_id", rawID).Debugf("Executing campaigns %s command", name)
	response, err := fetch(apiClient, campaignID)
	if err != nil {
		return err
	}
	return print(printer.GetResponseHandlerFromCommand(cmd), response, config)
}
