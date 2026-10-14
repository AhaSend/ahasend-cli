package printer

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func campaignStatsFixture() *responses.CampaignStats {
	openRate, bounceRate := 42.5, 1.25
	return &responses.CampaignStats{
		Object:       "campaign_stats",
		CampaignID:   uuid.MustParse("44444444-4444-4444-8444-444444444444"),
		Recipients:   100,
		Delivered:    80,
		Opens:        50,
		Clicks:       10,
		Unsubscribes: 2,
		Complaints:   1,
		Report: &responses.CampaignStatsReport{
			ComputedAt:             time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC),
			Recipients:             80,
			Openers:                34,
			Clickers:               8,
			Bounces:                1,
			Deferrals:              0,
			ByStatus:               map[string]int{"Delivered": 79, "Bounced": 1},
			ByBounceClassification: map[string]int{"Unknown": 1},
		},
		Rates: responses.CampaignStatsRates{OpenRate: &openRate, BounceRate: &bounceRate},
	}
}

// campaignStatsWithoutReport returns the stats of a campaign before its first
// recount: no report and no rates.
func campaignStatsWithoutReport() *responses.CampaignStats {
	stats := campaignStatsFixture()
	stats.Report = nil
	stats.Rates = responses.CampaignStatsRates{}
	return stats
}

func TestCampaignStatsJSONDecodesIntoSDKType(t *testing.T) {
	for name, stats := range map[string]*responses.CampaignStats{
		"with report":    campaignStatsFixture(),
		"without report": campaignStatsWithoutReport(),
	} {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleCampaignStats(stats, SingleConfig{}))
			var decoded responses.CampaignStats
			require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
			assert.Equal(t, *stats, decoded)
		})
	}
}

func TestCampaignStatsPlainShowsCountsRatesAndReport(t *testing.T) {
	useLocalTimeZone(t)
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleCampaignStats(campaignStatsFixture(), SingleConfig{SuccessMessage: "Stats"}))
	assert.Equal(t, "Stats\n"+
		"Campaign ID: 44444444-4444-4444-8444-444444444444\n"+
		"Recipients: 100\nDelivered: 80\nOpens: 50\nClicks: 10\nUnsubscribes: 2\nComplaints: 1\n"+
		"Open Rate: 42.50%\nBounce Rate: 1.25%\nRecount Failed: No\n"+
		"Report Computed At: 2026-10-02 12:00:00 UTC+2\nReport Recipients: 80\nReport Openers: 34\n"+
		"Report Clickers: 8\nReport Bounces: 1\nReport Deferrals: 0\n"+
		`Report By Status: {"Bounced":1,"Delivered":79}`+"\n"+
		`Report By Bounce Classification: {"Unknown":1}`+"\n", output.String())
}

func TestCampaignStatsWithNullRatesAndNoReport(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleCampaignStats(campaignStatsWithoutReport(), SingleConfig{SuccessMessage: "Stats"}))
		assert.Equal(t, "Stats\n"+
			"Campaign ID: 44444444-4444-4444-8444-444444444444\n"+
			"Recipients: 100\nDelivered: 80\nOpens: 50\nClicks: 10\nUnsubscribes: 2\nComplaints: 1\n"+
			"Recount Failed: No\n", output.String())
	})
	t.Run("table", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleCampaignStats(campaignStatsWithoutReport(), SingleConfig{SuccessMessage: "Stats"}))
		text := output.String()
		assert.Contains(t, text, "Open Rate")
		assert.NotContains(t, text, "%")
		assert.NotContains(t, text, "Report")
	})
	t.Run("csv", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleCampaignStats(campaignStatsWithoutReport(), SingleConfig{}))
		_, rows := readCSV(t, output.String())
		require.Len(t, rows, 1)
		row := rows[0]
		assert.Equal(t, "", row["open_rate"])
		assert.Equal(t, "", row["report_computed_at"])
		assert.Equal(t, "", row["report_by_status"])
	})
}

func TestCampaignStatsTableShowsReport(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleCampaignStats(campaignStatsFixture(), SingleConfig{SuccessMessage: "Stats"}))
	text := output.String()
	for _, expected := range []string{"Open Rate", "42.50%", "Report By Status", `{"Bounced":1,"Delivered":79}`} {
		assert.Contains(t, text, expected)
	}
}

func TestCampaignStatsCSVWritesOneRow(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleCampaignStats(campaignStatsFixture(), SingleConfig{}))
	headers, rows := readCSV(t, output.String())
	require.Len(t, rows, 1)
	assert.Equal(t, campaignStatsCSVHeaders, headers)
	row := rows[0]
	assert.Equal(t, "100", row["recipients"])
	assert.Equal(t, "42.5", row["open_rate"]) // as the API sent it
	assert.Equal(t, "false", row["recount_failed"])
	assert.Equal(t, "34", row["report_openers"])
	assert.Equal(t, `{"Bounced":1,"Delivered":79}`, row["report_by_status"])
	assert.Equal(t, `{"Unknown":1}`, row["report_by_bounce_classification"])
}

func TestCampaignStatsRendersInEveryFormat(t *testing.T) {
	for _, format := range GetSupportedFormats() {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleCampaignStats(campaignStatsFixture(), SingleConfig{SuccessMessage: "Stats"}))
			assert.Contains(t, output.String(), "44444444-4444-4444-8444-444444444444")
		})
	}
}
