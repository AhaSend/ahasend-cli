package stats

import (
	"fmt"
	"strings"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// groupByValues lists the values of --group-by.
var groupByValues = []string{"hour", "day", "week", "month"}

// statsFilters holds the filters every stats command sends. A nil field is
// left out of the request, so the API's default applies.
type statsFilters struct {
	FromTime         *time.Time
	ToTime           *time.Time
	SenderDomain     *string
	RecipientDomains *string
	Tags             *string
	GroupBy          *string
}

// addFilterFlags adds the time range, grouping and filter flags every stats
// command takes.
func addFilterFlags(cmd *cobra.Command) {
	cmd.Flags().String("from-time", "7d", "Start time (RFC3339, or relative like '7d' or '24h' for 7 days or 24 hours ago; empty leaves it to the API)")
	cmd.Flags().String("to-time", "", "End time (RFC3339, or relative like '1d' for 1 day ago; empty leaves it to the API, which uses now)")
	cmdutil.AddOneOfFlag(cmd, "group-by", "day", "Group results by: %s", groupByValues)
	cmdutil.AddRepeatableFlag(cmd, "sender-domain", "Filter by sender domain (can be used multiple times)")
	cmdutil.AddRepeatableFlag(cmd, "recipient-domain", "Filter by recipient domain (can be used multiple times)")
	cmdutil.AddRepeatableFlag(cmd, "tag", "Filter by message tag, none with a comma (can be used multiple times)")
	cmdutil.AddFlagAlias(cmd, "tags", "tag")
}

// addNoEffectFlag adds a bool flag that is accepted for compatibility and
// has no effect.
func addNoEffectFlag(cmd *cobra.Command, name string) {
	cmd.Flags().Bool(name, false, "")
	cmdutil.MarkNoEffect(cmd, name, "")
}

// readFilters returns the filters the flags of cmd set. It runs before
// authentication.
func readFilters(cmd *cobra.Command) (statsFilters, error) {
	var filters statsFilters
	fromTime, toTime, err := cmdutil.ReadPastTimeRange(cmd)
	if err != nil {
		return filters, err
	}
	groupBy, _ := cmd.Flags().GetString("group-by")
	groupBy, err = cmdutil.MatchOneOf(cmd, "group-by", groupBy)
	if err != nil {
		return filters, err
	}
	senderDomains, err := cmdutil.ReadRepeatable(cmd, "sender-domain", "")
	if err != nil {
		return filters, err
	}
	if err := validation.ValidateDomainNames("--sender-domain", senderDomains); err != nil {
		return filters, err
	}
	recipientDomains, err := cmdutil.ReadRepeatable(cmd, "recipient-domain", "")
	if err != nil {
		return filters, err
	}
	if err := validation.ValidateDomainNames("--recipient-domain", recipientDomains); err != nil {
		return filters, err
	}
	tags, err := cmdutil.ReadFilterTags(cmd)
	if err != nil {
		return filters, err
	}

	filters.FromTime = fromTime
	filters.ToTime = toTime
	filters.GroupBy = &groupBy
	filters.SenderDomain = cmdutil.JoinFilter(senderDomains)
	filters.RecipientDomains = cmdutil.JoinFilter(recipientDomains)
	filters.Tags = cmdutil.JoinFilter(tags)
	return filters, nil
}

// runStats reads the filters of cmd, fetches the statistics titled title
// with fetch, and prints them with print.
func runStats[R any](
	cmd *cobra.Command, title string,
	fetch func(client.AhaSendClient, statsFilters) (*R, error),
	print func(printer.ResponseHandler, *R, printer.StatsConfig) error,
) error {
	handler := printer.GetResponseHandlerFromCommand(cmd)

	filters, err := readFilters(cmd)
	if err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithField("filters", fmt.Sprintf("%+v", filters)).Debugf("Fetching %s", strings.ToLower(title))
	response, err := fetch(apiClient, filters)
	if err != nil {
		return err
	}
	return print(handler, response, printer.StatsConfig{Title: title})
}
