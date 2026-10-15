package printer

import (
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
