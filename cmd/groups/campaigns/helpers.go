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
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// The most lists and segments a campaign uses, included and excluded
// together, counting an ID given twice once, as the API does.
const (
	maxAudienceLists    = 50
	maxAudienceSegments = 10
)

// audienceSet is one of the four sets of a campaign's audience: the flag
// that gives its IDs, the flag of update that empties it, and what its IDs
// name.
type audienceSet struct {
	flag, clearFlag, what string
}

var (
	listsSet           = audienceSet{"list-id", "clear-lists", "list ID"}
	segmentsSet        = audienceSet{"segment-id", "clear-segments", "segment ID"}
	excludeListsSet    = audienceSet{"exclude-list-id", "clear-exclude-lists", "list ID"}
	excludeSegmentsSet = audienceSet{"exclude-segment-id", "clear-exclude-segments", "segment ID"}
)

// audience holds the four sets of a campaign's audience as a create or an
// update sends them: nil for a set no flag names, which an update leaves as
// it is, and an empty slice for a set a clear flag empties.
type audience struct {
	listIDs, segmentIDs, excludeListIDs, excludeSegmentIDs []uuid.UUID
}

// readAudience parses the audience flags of a create or an update. On
// update, each clear flag empties its set and cannot be used with the set's
// ID flag. More than maxAudienceLists distinct lists, or maxAudienceSegments
// distinct segments, included and excluded together, is a validation error;
// the API checks the sets it stores.
func readAudience(cmd *cobra.Command) (audience, error) {
	var sets audience
	for _, read := range []struct {
		set audienceSet
		ids *[]uuid.UUID
	}{
		{listsSet, &sets.listIDs},
		{segmentsSet, &sets.segmentIDs},
		{excludeListsSet, &sets.excludeListIDs},
		{excludeSegmentsSet, &sets.excludeSegmentIDs},
	} {
		ids, err := readAudienceSet(cmd, read.set)
		if err != nil {
			return audience{}, err
		}
		*read.ids = ids
	}

	if count := countDistinct(sets.listIDs, sets.excludeListIDs); count > maxAudienceLists {
		return audience{}, errors.NewValidationError(fmt.Sprintf(
			"--%s and --%s take at most %d lists in all, got %d", listsSet.flag, excludeListsSet.flag, maxAudienceLists, count), nil)
	}
	if count := countDistinct(sets.segmentIDs, sets.excludeSegmentIDs); count > maxAudienceSegments {
		return audience{}, errors.NewValidationError(fmt.Sprintf(
			"--%s and --%s take at most %d segments in all, got %d", segmentsSet.flag, excludeSegmentsSet.flag, maxAudienceSegments, count), nil)
	}
	return sets, nil
}

// readAudienceSet returns the IDs of set, an empty slice when its clear flag
// is set, or nil when neither flag is.
func readAudienceSet(cmd *cobra.Command, set audienceSet) ([]uuid.UUID, error) {
	clearFlag := ""
	if cmd.Flags().Lookup(set.clearFlag) != nil {
		clearFlag = set.clearFlag
		if err := cmdutil.RejectTogether(cmd, set.flag, clearFlag); err != nil {
			return nil, err
		}
	}
	values, err := cmdutil.ReadRepeatable(cmd, set.flag, clearFlag)
	if err != nil {
		return nil, err
	}
	if clearFlag != "" {
		if cleared, _ := cmd.Flags().GetBool(clearFlag); cleared {
			return []uuid.UUID{}, nil
		}
	}
	return validation.ParseUUIDs(set.what, values)
}

// countDistinct counts the distinct IDs of sets.
func countDistinct(sets ...[]uuid.UUID) int {
	distinct := map[uuid.UUID]bool{}
	for _, ids := range sets {
		for _, id := range ids {
			distinct[id] = true
		}
	}
	return len(distinct)
}

// textUncheckedRetry is the line a refused send or test adds when one of its
// issues is text_unchecked: the API could not check the text right now, and
// a later run can pass.
const textUncheckedRetry = "text_unchecked clears on its own: run the command again"

// campaignNotReadyError replaces the error of a send or test that the API
// refused because the campaign has issues. Outside JSON mode it returns an
// error that lists each issue as "code: message" on its own line, after a line
// that starts with "failed to <action> campaign", and exits 4 like the API's
// 400 it replaces. When an issue is text_unchecked, textUncheckedRetry
// follows the issues, asking for a new idempotency key when keyStored: the
// API stores its refusal of the request's key, as it does for a send with
// --idempotency-key. JSON mode and every other error keep the SDK error, so
// the raw API body prints.
func campaignNotReadyError(cmd *cobra.Command, err error, action string, keyStored bool) error {
	notReady, ok := api.AsCampaignNotReady(err)
	if !ok || printer.GetResponseHandlerFromCommand(cmd).GetFormat() == "json" {
		return err
	}
	// Each issue is escaped on its own line, so a line break in an issue
	// cannot show as another issue
	lines := []string{fmt.Sprintf("failed to %s campaign: %s", action, printer.EscapeControl(notReady.Message))}
	unchecked := false
	for _, issue := range notReady.Issues {
		lines = append(lines, printer.EscapeControl(issue.Code)+": "+printer.EscapeControl(issue.Message))
		unchecked = unchecked || issue.Code == responses.CampaignIssueTextUnchecked
	}
	if unchecked {
		retry := textUncheckedRetry
		if keyStored {
			retry += " with a new --idempotency-key"
		}
		lines = append(lines, retry)
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
