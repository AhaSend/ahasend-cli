package stats

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/apiresponse"
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
)

// breakdownValues lists the values of --breakdown.
var breakdownValues = []string{"domain"}

// emailStatsPageLimit is the number of domains the command asks for on each
// page of a breakdown: the API's maximum, so a breakdown takes as few
// requests as it can.
const emailStatsPageLimit int32 = 50

// NewEmailsCommand creates the stats emails command
func NewEmailsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "emails",
		Short: "View email counts of any period, overall or by domain",
		Long: `View email counts of any period in the account's history, for the whole
account or for each of its domains. Unlike the other stats commands, the
counts are not limited to the account's message metadata retention period.

Each period shows how many messages were received (accepted by AhaSend,
inbound mail included), delivered, bounced and suppressed, how many spam
complaints came in, and how many temporary delivery failures (deferred),
opens and clicks there were. Deferred, opened and clicked count events, so
one message can add more than one; stats deliverability counts messages, so
its numbers differ. Opens and clicks are not fully accurate: AhaSend makes a
reasonable effort to filter out bots and scanners.

The counts cover transactional, campaign and inbound mail; inbound mail
counts under the domain that received it. Sandbox mail is not counted.
Deleted domains are included. Without --sender-domain the counts cover every
domain the API key can read.

Periods are in UTC, and weeks start on Monday. The range includes --from-time
and excludes --to-time, and the API rounds it out to whole periods. Without
--to-time the range ends now; without --from-time it starts 30 days before
its end. A range can be at most 31 days with --group-by hour, 366 days with
day (the default) and week, and 5 years with month. The table and plain
outputs show the start and end of each period in the local time zone.

--breakdown domain shows the counts of each domain apart, and the command
fetches every page of domains. The totals cover the whole request; the rows
of the domains can add up to a few events more or less than the totals.

Table and plain output show the totals of the whole range, then each row.
CSV output has one line per row, without the totals. JSON output is the
API's response; with a breakdown, one response that holds the rows of every
page.

Time ranges can be specified using RFC3339 format or relative formats:
- RFC3339: "2024-01-15T00:00:00Z"
- Relative: "1h", "24h", "7d", "30d" (ago, such as 7d for 7 days ago)`,
		Example: `  # Daily counts of today and the 30 days before it
  ahasend stats emails

  # Monthly counts of 2025 for each domain
  ahasend stats emails \
    --from-time 2025-01-01T00:00:00Z \
    --to-time 2026-01-01T00:00:00Z \
    --group-by month \
    --breakdown domain

  # Weekly counts of one domain over the last 90 days
  ahasend stats emails --from-time 90d --group-by week --sender-domain example.com

  # Export the monthly counts of the last year to CSV
  ahasend stats emails --from-time 1y --group-by month --output csv`,
		Args:         cmdutil.NoArgs,
		RunE:         runEmailStats,
		SilenceUsage: true,
	}

	cmd.Flags().String("from-time", "", "Start time, included (RFC3339, or relative like '90d' for 90 days ago; without it, the API starts 30 days before the end)")
	cmd.Flags().String("to-time", "", "End time, excluded (RFC3339, or relative like '1d' for 1 day ago; without it, the API ends the range now)")
	cmdutil.AddOneOfFlag(cmd, "group-by", "", "Group results by: %s (without it, the API groups by day)", groupByValues)
	cmdutil.AddRepeatableFlag(cmd, "sender-domain", "Count only this domain of the account (can be used multiple times)")
	cmdutil.AddOneOfFlag(cmd, "breakdown", "", "Break the counts down by: %s", breakdownValues)

	return cmd
}

func runEmailStats(cmd *cobra.Command, args []string) error {
	handler := printer.GetResponseHandlerFromCommand(cmd)

	params, err := readEmailStatsParams(cmd)
	if err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	response, err := fetchEmailStats(apiClient, params)
	if err != nil {
		return err
	}
	return handler.HandleEmailStats(response, printer.StatsConfig{Title: "Email Statistics"})
}

// readEmailStatsParams returns the request params the flags of cmd set. A
// flag that is not set leaves its param nil, so the API's default applies.
// It runs before authentication.
func readEmailStatsParams(cmd *cobra.Command) (requests.GetEmailStatisticsParams, error) {
	fromTime, toTime, err := cmdutil.ReadPastTimeRange(cmd)
	if err != nil {
		return requests.GetEmailStatisticsParams{}, err
	}
	groupBy, err := cmdutil.ReadOneOf(cmd, "group-by")
	if err != nil {
		return requests.GetEmailStatisticsParams{}, err
	}
	senderDomains, err := cmdutil.ReadRepeatable(cmd, "sender-domain", "")
	if err != nil {
		return requests.GetEmailStatisticsParams{}, err
	}
	if err := validation.ValidateDomainNames("--sender-domain", senderDomains); err != nil {
		return requests.GetEmailStatisticsParams{}, err
	}
	breakdown, err := cmdutil.ReadOneOf(cmd, "breakdown")
	if err != nil {
		return requests.GetEmailStatisticsParams{}, err
	}

	return requests.GetEmailStatisticsParams{
		FromTime:     fromTime,
		ToTime:       toTime,
		GroupBy:      groupBy,
		SenderDomain: cmdutil.JoinFilter(senderDomains),
		Breakdown:    breakdown,
	}, nil
}

// fetchEmailStats returns the email statistics params ask for: the one
// response without a breakdown, and with one every page, fetched in order
// and merged into one response.
func fetchEmailStats(c client.AhaSendClient, params requests.GetEmailStatisticsParams) (*responses.EmailStatisticsResponse, error) {
	if params.Breakdown == nil {
		logger.Get().Debug("Fetching email statistics")
		return c.GetEmailStatistics(params)
	}

	limit := emailStatsPageLimit
	params.Limit = &limit
	var pages []*responses.EmailStatisticsResponse
	for {
		number := len(pages) + 1
		logger.Get().WithField("page", number).Debug("Fetching email statistics")
		page, err := c.GetEmailStatistics(params)
		if err != nil {
			return nil, err
		}
		if number > 1 {
			if err := checkPageRange(pages[0], page, number); err != nil {
				return nil, err
			}
		}
		pages = append(pages, page)
		if !page.Pagination.HasMore {
			return mergeEmailStatsPages(pages)
		}

		next := page.Pagination.NextCursor
		if next == nil || *next == "" || (params.After != nil && *next == *params.After) {
			return nil, fmt.Errorf("page %d of the breakdown says that more pages follow, but has no new next_cursor", number)
		}
		params.After = next
	}
}

// checkPageRange returns an error when page, page number of a breakdown,
// covers another time range than first, the first page. The API takes a
// missing --to-time, or one after now, as the time of each request, so the
// range moves when a period ends while the pages are fetched, and pages of
// different ranges cannot be merged.
func checkPageRange(first, page *responses.EmailStatisticsResponse, number int) error {
	if page.FromTime.Equal(first.FromTime) && page.ToTime.Equal(first.ToTime) {
		return nil
	}
	return fmt.Errorf("the time range moved while the pages of the breakdown were fetched: "+
		"page 1 covers %s to %s, page %d covers %s to %s; run the command again, or give a --to-time in the past",
		first.FromTime.Format(time.RFC3339), first.ToTime.Format(time.RFC3339),
		number, page.FromTime.Format(time.RFC3339), page.ToTime.Format(time.RFC3339))
}

// mergeEmailStatsPages returns the pages of a breakdown as one response: the
// last page, whose pagination has no more pages, with the rows of every page
// in order. Every page has the same range and totals. When the API body of
// every page was recorded, the merged response records the last page's body
// with the rows of every page's body, which JSON output prints; otherwise
// JSON output encodes the merged response.
func mergeEmailStatsPages(pages []*responses.EmailStatisticsResponse) (*responses.EmailStatisticsResponse, error) {
	last := pages[len(pages)-1]
	if len(pages) == 1 {
		return last, nil
	}

	merged := *last
	merged.Data = []responses.EmailStatistics{}
	for _, page := range pages {
		merged.Data = append(merged.Data, page.Data...)
	}
	body, err := mergeEmailStatsBodies(pages)
	if err != nil {
		return nil, err
	}
	if body != nil {
		apiresponse.Remember(&merged, body)
	}
	return &merged, nil
}

// mergeEmailStatsBodies returns the API body of the last page with the "data"
// rows of every page's body, or nil when a page has no recorded body.
func mergeEmailStatsBodies(pages []*responses.EmailStatisticsResponse) ([]byte, error) {
	var rows [][]byte
	for _, page := range pages {
		body, ok := apiresponse.Body(page)
		if !ok {
			return nil, nil
		}
		var fields struct {
			Data []json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(body, &fields); err != nil {
			return nil, fmt.Errorf("could not merge the pages of the breakdown: %w", err)
		}
		for _, row := range fields.Data {
			rows = append(rows, row)
		}
	}
	data := append(append([]byte("["), bytes.Join(rows, []byte(","))...), ']')

	last, _ := apiresponse.Body(pages[len(pages)-1])
	return replaceJSONField(last, "data", data)
}

// replaceJSONField returns the JSON object body with the value of its
// top-level field name replaced by value. The other fields keep their order
// and their values as body has them.
func replaceJSONField(body []byte, name string, value []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("could not merge the pages of the breakdown: the API response is not a JSON object")
	}

	var merged bytes.Buffer
	merged.WriteByte('{')
	replaced := false
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("could not merge the pages of the breakdown: %w", err)
		}
		key, _ := token.(string)
		var field json.RawMessage
		if err := decoder.Decode(&field); err != nil {
			return nil, fmt.Errorf("could not merge the pages of the breakdown: %w", err)
		}
		if key == name {
			field, replaced = value, true
		}
		encodedKey, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		if merged.Len() > 1 {
			merged.WriteByte(',')
		}
		merged.Write(encodedKey)
		merged.WriteByte(':')
		merged.Write(field)
	}
	if !replaced {
		return nil, fmt.Errorf("could not merge the pages of the breakdown: the API response has no %s field", name)
	}
	merged.WriteByte('}')
	return merged.Bytes(), nil
}
