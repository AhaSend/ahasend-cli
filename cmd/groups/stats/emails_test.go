package stats

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// emailsToTime is the end of the range of the email statistics bodies.
const emailsToTime = "2026-10-01T00:00:00Z"

// emailsBody returns the body of a monthly breakdown by domain from
// 2026-08-01 to toTime, with rows and pagination.
func emailsBody(toTime, rows, pagination string) string {
	return `{"object":"email_statistics","from_time":"2026-08-01T00:00:00Z","to_time":"` + toTime + `",` +
		`"group_by":"month","breakdown":"domain","totals":{"reception_count":30,"delivered_count":27,"bounced_count":2,` +
		`"suppressed_count":1,"complained_count":1,"deferred_count":4,"opened_count":9,"clicked_count":3},` +
		`"data":[` + rows + `],"pagination":` + pagination + `}`
}

// emailsRow returns the row of domain for August 2026 with reception
// messages received.
func emailsRow(domain string, reception int) string {
	return fmt.Sprintf(`{"from_timestamp":"2026-08-01T00:00:00Z","to_timestamp":"2026-09-01T00:00:00Z",`+
		`"sender_domain":%q,"reception_count":%d,"delivered_count":5,"bounced_count":1,"suppressed_count":0,`+
		`"complained_count":0,"deferred_count":2,"opened_count":4,"clicked_count":1}`, domain, reception)
}

// emailsPages makes the mock answer the first page of a breakdown with
// first, and the page after the cursor "cursor-1" with second, and returns
// the params of every request.
func emailsPages(t *testing.T, mockClient *mocks.MockClient, first, second string) *[]requests.GetEmailStatisticsParams {
	t.Helper()
	var sent []requests.GetEmailStatisticsParams
	record := func(arguments mock.Arguments) {
		sent = append(sent, arguments.Get(0).(requests.GetEmailStatisticsParams))
	}
	mockClient.On("GetEmailStatistics", mock.MatchedBy(func(params requests.GetEmailStatisticsParams) bool {
		return params.After == nil
	})).Run(record).Return(mocks.APIResponse[responses.EmailStatisticsResponse](t, first), nil).Once()
	mockClient.On("GetEmailStatistics", mock.MatchedBy(func(params requests.GetEmailStatisticsParams) bool {
		return params.After != nil && *params.After == "cursor-1"
	})).Run(record).Return(mocks.APIResponse[responses.EmailStatisticsResponse](t, second), nil).Once()
	return &sent
}

// twoPages are the bodies of a breakdown of two pages: a.example.com on the
// first and b.example.com on the second.
var twoPages = []string{
	emailsBody(emailsToTime, emailsRow("a.example.com", 10), `{"has_more":true,"next_cursor":"cursor-1"}`),
	emailsBody(emailsToTime, emailsRow("b.example.com", 20), `{"has_more":false}`),
}

// emailsParams runs stats emails with args against a single response and
// returns the params of the one request it sent.
func emailsParams(t *testing.T, args ...string) requests.GetEmailStatisticsParams {
	t.Helper()
	var sent []requests.GetEmailStatisticsParams
	mockClient := &mocks.MockClient{}
	mockClient.On("GetEmailStatistics", mock.Anything).Run(func(arguments mock.Arguments) {
		sent = append(sent, arguments.Get(0).(requests.GetEmailStatisticsParams))
	}).Return(mocks.APIResponse[responses.EmailStatisticsResponse](t, emailsBody(emailsToTime, "", `{"has_more":false}`)), nil)
	testutil.InjectClient(t, mockClient)

	_, _, err := testutil.RunCommand(t, NewEmailsCommand(), "plain", "", args...)
	require.NoError(t, err)
	require.Len(t, sent, 1)
	return sent[0]
}

func TestEmailsSendsNoParamWithoutFlags(t *testing.T) {
	assert.Equal(t, requests.GetEmailStatisticsParams{}, emailsParams(t))
}

func TestEmailsSendsTheFlagsAsParams(t *testing.T) {
	sent := emailsParams(t, "--from-time", "2025-01-01T00:00:00Z", "--to-time", "2026-01-01T00:00:00Z",
		"--group-by", "MONTH", "--sender-domain", "example.com", "--sender-domain", "example.org", "--breakdown", "Domain")

	require.NotNil(t, sent.FromTime)
	assert.True(t, sent.FromTime.Equal(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)), sent.FromTime)
	require.NotNil(t, sent.ToTime)
	assert.True(t, sent.ToTime.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), sent.ToTime)
	assert.Equal(t, "month", *sent.GroupBy)
	assert.Equal(t, "example.com,example.org", *sent.SenderDomain)
	assert.Equal(t, "domain", *sent.Breakdown)
	require.NotNil(t, sent.Limit, "a breakdown asks for the most domains a page holds")
	assert.Equal(t, int32(50), *sent.Limit)
	assert.Nil(t, sent.After)
}

func TestEmailsTakesRelativeTimes(t *testing.T) {
	sent := emailsParams(t, "--from-time", "90d", "--to-time", "1d")

	require.NotNil(t, sent.FromTime)
	assert.WithinDuration(t, time.Now().AddDate(0, 0, -90), *sent.FromTime, time.Minute)
	require.NotNil(t, sent.ToTime)
	assert.WithinDuration(t, time.Now().AddDate(0, 0, -1), *sent.ToTime, time.Minute)
}

func TestEmailsSendsNoPageParamsWithoutABreakdown(t *testing.T) {
	sent := emailsParams(t, "--group-by", "week")

	assert.Nil(t, sent.Limit)
	assert.Nil(t, sent.After)
}

func TestEmailsHasNoPageFlags(t *testing.T) {
	cmd := NewEmailsCommand()

	assert.Nil(t, cmd.Flags().Lookup("limit"))
	assert.Nil(t, cmd.Flags().Lookup("after"))
}

// TestEmailsBreakdownOfTwoPagesShowsTheRowsOfBoth fetches a breakdown of two
// pages and checks that each format shows the rows of both pages.
func TestEmailsBreakdownOfTwoPagesShowsTheRowsOfBoth(t *testing.T) {
	want := map[string][]string{
		"table": {"SENDER DOMAIN", "│ a.example.com │ 10 ", "│ b.example.com │ 20 "},
		"plain": {"  Sender Domain: a.example.com\n  Reception: 10\n", "  Sender Domain: b.example.com\n  Reception: 20\n"},
		"csv": {
			"2026-08-01T00:00:00Z,2026-09-01T00:00:00Z,a.example.com,10,5,1,0,0,2,4,1\n",
			"2026-08-01T00:00:00Z,2026-09-01T00:00:00Z,b.example.com,20,5,1,0,0,2,4,1\n",
		},
		"json": {`"sender_domain": "a.example.com"`, `"sender_domain": "b.example.com"`},
	}
	for format, texts := range want {
		t.Run(format, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			sent := emailsPages(t, mockClient, twoPages[0], twoPages[1])

			stdout, err := testutil.RunWithClient(t, mockClient, NewEmailsCommand(), format, "--breakdown", "domain")

			require.NoError(t, err)
			mockClient.AssertExpectations(t)
			for _, text := range texts {
				assert.Contains(t, stdout, text)
			}
			require.Len(t, *sent, 2)
			assert.Equal(t, "cursor-1", *(*sent)[1].After)
			assert.Equal(t, int32(50), *(*sent)[1].Limit)
		})
	}
}

func TestEmailsCSVHasNoTotals(t *testing.T) {
	mockClient := &mocks.MockClient{}
	emailsPages(t, mockClient, twoPages[0], twoPages[1])

	stdout, err := testutil.RunWithClient(t, mockClient, NewEmailsCommand(), "csv", "--breakdown", "domain")

	require.NoError(t, err)
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	assert.Equal(t, "from_timestamp,to_timestamp,sender_domain,reception_count,delivered_count,bounced_count,"+
		"suppressed_count,complained_count,deferred_count,opened_count,clicked_count", lines[0])
	assert.Len(t, lines, 3, "a header and one line per row")
}

// TestEmailsBreakdownJSONIsOneResponseWithEveryRow checks that the JSON of a
// breakdown of two pages is the last page's body with the rows of both, so
// it has the totals and pagination.has_more false.
func TestEmailsBreakdownJSONIsOneResponseWithEveryRow(t *testing.T) {
	mockClient := &mocks.MockClient{}
	emailsPages(t, mockClient, twoPages[0], twoPages[1])

	stdout, err := testutil.RunWithClient(t, mockClient, NewEmailsCommand(), "json", "--breakdown", "domain")

	require.NoError(t, err)
	assert.Equal(t, testutil.IndentedJSON(t, emailsBody(emailsToTime,
		emailsRow("a.example.com", 10)+","+emailsRow("b.example.com", 20), `{"has_more":false}`)), stdout)
	var merged map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &merged))
	assert.Equal(t, float64(30), merged["totals"].(map[string]any)["reception_count"])
	assert.Equal(t, map[string]any{"has_more": false}, merged["pagination"])
}

func TestEmailsBreakdownOfOnePagePrintsItsBodyUnchanged(t *testing.T) {
	body := emailsBody(emailsToTime, emailsRow("a.example.com", 10), `{"has_more":false}`)
	mockClient := &mocks.MockClient{}
	mockClient.On("GetEmailStatistics", mock.Anything).
		Return(mocks.APIResponse[responses.EmailStatisticsResponse](t, body), nil).Once()

	stdout, err := testutil.RunWithClient(t, mockClient, NewEmailsCommand(), "json", "--breakdown", "domain")

	require.NoError(t, err)
	assert.Equal(t, testutil.IndentedJSON(t, body), stdout)
	mockClient.AssertExpectations(t)
}

func TestEmailsFailsWhenAPageHasAnotherRange(t *testing.T) {
	mockClient := &mocks.MockClient{}
	emailsPages(t, mockClient, twoPages[0],
		emailsBody("2026-11-01T00:00:00Z", emailsRow("b.example.com", 20), `{"has_more":false}`))
	testutil.InjectClient(t, mockClient)

	stdout, _, err := testutil.RunCommand(t, NewEmailsCommand(), "plain", "", "--breakdown", "domain")

	require.Error(t, err)
	assert.Equal(t, "the time range moved while the pages of the breakdown were fetched: "+
		"page 1 covers 2026-08-01T00:00:00Z to 2026-10-01T00:00:00Z, page 2 covers 2026-08-01T00:00:00Z to 2026-11-01T00:00:00Z; "+
		"run the command again, or give a --to-time in the past", err.Error())
	assert.Equal(t, 1, clierrors.GetExitCode(err))
	assert.Empty(t, stdout, "no page is printed")
}

func TestEmailsFailsWhenAPageHasMoreWithoutACursor(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("GetEmailStatistics", mock.Anything).Return(mocks.APIResponse[responses.EmailStatisticsResponse](t,
		emailsBody(emailsToTime, emailsRow("a.example.com", 10), `{"has_more":true}`)), nil).Once()
	testutil.InjectClient(t, mockClient)

	stdout, _, err := testutil.RunCommand(t, NewEmailsCommand(), "plain", "", "--breakdown", "domain")

	require.EqualError(t, err, "page 1 of the breakdown says that more pages follow, but has no new next_cursor")
	assert.Empty(t, stdout)
	mockClient.AssertExpectations(t)
}

func TestEmailsRejectsBadFlagsBeforeAuthenticating(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"--from-time", "soon"}, `invalid --from-time value "soon": use RFC 3339 or a relative time such as 24h or 7d`},
		{[]string{"--to-time", "soon"}, `invalid --to-time value "soon": use RFC 3339 or a relative time such as 24h or 7d`},
		{[]string{"--from-time", "1d", "--to-time", "7d"}, "--from-time must be before --to-time"},
		{[]string{"--group-by", "year"}, "--group-by must be one of: hour, day, week, month"},
		{[]string{"--group-by", ""}, "--group-by cannot be empty"},
		{[]string{"--sender-domain", ""}, "--sender-domain cannot be empty"},
		{[]string{"--sender-domain", "not a domain"}, `invalid --sender-domain value "not a domain": not a domain name`},
		{[]string{"--breakdown", "sender"}, "--breakdown must be one of: domain"},
	}
	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			_, _, err := testutil.RunCommand(t, NewEmailsCommand(), "plain", "", tt.args...)

			require.Error(t, err)
			assert.Equal(t, tt.message, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.False(t, *called)
		})
	}
}
