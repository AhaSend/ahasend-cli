package printer

import (
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
)

// TestBouncePercentagesHaveTwoDecimals checks that bounce stats give each
// classification's share with two decimals, like the other rates.
func TestBouncePercentagesHaveTwoDecimals(t *testing.T) {
	stats := &responses.BounceStatisticsResponse{Data: []responses.BounceStatistics{{
		FromTimestamp: styleTime, ToTimestamp: styleTime,
		Bounces: []responses.Bounce{{Classification: "BadMailbox", Count: 1}, {Classification: "SpamBlock", Count: 2}},
	}}}
	render := func(h ResponseHandler) error { return h.HandleBounceStats(stats, StatsConfig{}) }

	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			output := renderWith(t, format, render)
			assert.Contains(t, output, "33.33%")
			assert.Contains(t, output, "66.67%")
		})
	}
}

// TestDeliveryTimesHaveOneFormInEveryFormat checks that table and plain
// output give delivery times the same way, such as 1.50s or 2.50m.
func TestDeliveryTimesHaveOneFormInEveryFormat(t *testing.T) {
	stats := styleDeliveryTimes()
	stats.Data[0].AvgDeliveryTime = 150
	render := func(h ResponseHandler) error { return h.HandleDeliveryTimeStats(stats, StatsConfig{}) }

	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			output := renderWith(t, format, render)
			assert.Contains(t, output, "2.50m")
			assert.Contains(t, output, "1.50s")
			assert.NotContains(t, output, "seconds")
		})
	}
}

// TestDeliveryTimeEntriesShowInEveryFormat checks one rule for a recipient
// domain entry without a domain or a time: every format shows it, table and
// plain output with "-" for what is missing, and each with its count.
func TestDeliveryTimeEntriesShowInEveryFormat(t *testing.T) {
	seconds, domain := 3.0, "example.org"
	stats := styleDeliveryTimes()
	stats.Data[0].DeliveryTimes = append(stats.Data[0].DeliveryTimes,
		responses.DeliveryTime{DeliveryTime: &seconds, Count: 4},
		responses.DeliveryTime{RecipientDomain: &domain, Count: 5})
	stats.Data[0].DeliveryTimes[0].Count = 6
	render := func(h ResponseHandler) error { return h.HandleDeliveryTimeStats(stats, StatsConfig{}) }

	assert.Contains(t, renderWith(t, "plain", render), "  Per-Domain Delivery Times:\n"+
		"    example.com: 1.50s (6 delivered)\n    -: 3.00s (4 delivered)\n    example.org: - (5 delivered)\n")
	table := renderWith(t, "table", render)
	assert.Regexp(t, `│ example.com +│ 6 +│ 1.50s +│`, table)
	assert.Regexp(t, `│ - +│ 4 +│ 3.00s +│`, table)
	assert.Regexp(t, `│ example.org +│ 5 +│ - +│`, table)
	csv := renderWith(t, "csv", render)
	assert.Contains(t, csv, ",example.com,6,1.5\n")
	assert.Contains(t, csv, ",,4,3\n")
	assert.Contains(t, csv, ",example.org,5,\n")
}

// TestBounceStatsShowAPeriodWithoutBouncesInEveryFormat checks that CSV
// output, like table and plain output, has a row for a period without
// bounces.
func TestBounceStatsShowAPeriodWithoutBouncesInEveryFormat(t *testing.T) {
	stats := &responses.BounceStatisticsResponse{Data: []responses.BounceStatistics{
		{FromTimestamp: styleTime, ToTimestamp: styleTime.Add(time.Hour)},
		{FromTimestamp: styleTime.Add(time.Hour), ToTimestamp: styleTime.Add(2 * time.Hour),
			Bounces: []responses.Bounce{{Classification: "BadMailbox", Count: 2}}},
	}}
	render := func(h ResponseHandler) error { return h.HandleBounceStats(stats, StatsConfig{}) }

	assert.Equal(t, "from_timestamp,to_timestamp,classification,count,percentage\n"+
		"2026-09-10T08:30:00Z,2026-09-10T09:30:00Z,,0,\n"+
		"2026-09-10T09:30:00Z,2026-09-10T10:30:00Z,BadMailbox,2,100.00\n", renderWith(t, "csv", render))
	assert.Contains(t, renderWith(t, "plain", render), "  No bounces recorded\n")
}

// TestEmailStatsShowTheSenderDomainOnlyInABreakdown checks that table and
// plain output show a sender domain only for a breakdown by domain, and that
// CSV output always has the column, empty without a breakdown.
func TestEmailStatsShowTheSenderDomainOnlyInABreakdown(t *testing.T) {
	render := func(h ResponseHandler) error { return h.HandleEmailStats(styleEmailStats(false), StatsConfig{}) }

	assert.NotContains(t, renderWith(t, "table", render), "SENDER DOMAIN")
	assert.NotContains(t, renderWith(t, "plain", render), "Sender Domain")
	headers, rows := readCSV(t, renderWith(t, "csv", render))
	assert.Equal(t, "sender_domain", headers[2])
	assert.Equal(t, "", rows[0]["sender_domain"])
}

// TestEmailStatsCSVHasOneLinePerRowWithoutTheTotals checks that CSV output
// writes the counters of each row with the API's field names, and no line
// for the totals.
func TestEmailStatsCSVHasOneLinePerRowWithoutTheTotals(t *testing.T) {
	render := func(h ResponseHandler) error { return h.HandleEmailStats(styleEmailStats(true), StatsConfig{}) }

	assert.Equal(t, "from_timestamp,to_timestamp,sender_domain,reception_count,delivered_count,bounced_count,"+
		"suppressed_count,complained_count,deferred_count,opened_count,clicked_count\n"+
		"2026-09-10T08:30:00Z,2026-09-10T09:30:00Z,example.com,9,8,1,2,1,3,5,4\n"+
		"2026-09-10T08:30:00Z,2026-09-10T09:30:00Z,example.org,9,8,1,2,1,3,5,4\n", renderWith(t, "csv", render))
}

// TestEmailStatsWithoutRowsShowTheTotals checks that a breakdown in which no
// domain has counts shows its totals and says so in table and plain output,
// and writes only the header in CSV output.
func TestEmailStatsWithoutRowsShowTheTotals(t *testing.T) {
	stats := styleEmailStats(true)
	stats.Data = nil
	render := func(h ResponseHandler) error { return h.HandleEmailStats(stats, StatsConfig{}) }

	table := renderWith(t, "table", render)
	assert.Equal(t, []string{"18", "16", "2", "4", "2", "6", "10", "8"}, tableCells(table)[1][1:])
	assert.True(t, strings.HasSuffix(table, "\nNo domain has email statistics in this time range\n"), table)
	plain := renderWith(t, "plain", render)
	assert.Contains(t, plain, "Totals:\n  Time Period: ")
	assert.Contains(t, plain, "  Reception: 18\n")
	assert.True(t, strings.HasSuffix(plain, "\n\nNo domain has email statistics in this time range\n"), plain)
	headers, rows := readCSV(t, renderWith(t, "csv", render))
	assert.Equal(t, "from_timestamp", headers[0])
	assert.Empty(t, rows)
}
