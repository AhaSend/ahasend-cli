package printer

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPausedDomain() *responses.Domain {
	pausedAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	reason := responses.DomainPauseReasonBounceRate
	return &responses.Domain{
		Object:      "domain",
		ID:          uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		Domain:      "news.example.com",
		DNSValid:    true,
		SendingType: common.DomainSendingTypeMarketing,
		Paused:      true,
		PausedAt:    &pausedAt,
		PauseReason: &reason,
	}
}

// readCSV parses CSV output into its header row and one map per data row,
// keyed by header.
func readCSV(t *testing.T, out string) ([]string, []map[string]string) {
	t.Helper()
	records, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	require.NoError(t, err)
	require.NotEmpty(t, records)

	rows := make([]map[string]string, 0, len(records)-1)
	for _, record := range records[1:] {
		row := make(map[string]string, len(record))
		for i, header := range records[0] {
			row[header] = record[i]
		}
		rows = append(rows, row)
	}
	return records[0], rows
}

func TestDomainCSV_SingleFillsNewColumns(t *testing.T) {
	var buf bytes.Buffer
	domain := newTestPausedDomain()
	require.NoError(t, testHandler(t, "csv", &buf, os.Stderr).HandleSingleDomain(domain, SingleConfig{}))

	_, rows := readCSV(t, buf.String())
	require.Len(t, rows, 1)
	assert.Equal(t, "marketing", rows[0]["sending_type"])
	assert.Equal(t, "2026-03-01T12:00:00Z", rows[0]["paused_at"])
	assert.Equal(t, "bounce_rate", rows[0]["pause_reason"])
}

func TestDomainCSV_ListFillsNewColumns(t *testing.T) {
	active := newTestPausedDomain()
	active.Domain = "example.com"
	active.SendingType = common.DomainSendingTypeTransactional
	active.Paused, active.PausedAt, active.PauseReason = false, nil, nil

	var buf bytes.Buffer
	response := &responses.PaginatedDomainsResponse{Data: []responses.Domain{*newTestPausedDomain(), *active}}
	require.NoError(t, testHandler(t, "csv", &buf, os.Stderr).HandleDomainList(response, ListConfig{}))

	_, rows := readCSV(t, buf.String())
	require.Len(t, rows, 2)
	assert.Equal(t, "marketing", rows[0]["sending_type"])
	assert.Equal(t, "bounce_rate", rows[0]["pause_reason"])
	assert.NotEmpty(t, rows[0]["paused_at"])
	assert.Equal(t, "transactional", rows[1]["sending_type"])
	assert.Empty(t, rows[1]["paused_at"])
	assert.Empty(t, rows[1]["pause_reason"])
}

func TestDomainPlain_ShowsSendingTypeAndPause(t *testing.T) {
	domain := newTestPausedDomain()

	var single, list bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &single, os.Stderr).HandleSingleDomain(domain, SingleConfig{}))
	require.NoError(t, testHandler(t, "plain", &list, os.Stderr).HandleDomainList(
		&responses.PaginatedDomainsResponse{Data: []responses.Domain{*domain}}, ListConfig{}))

	for name, out := range map[string]string{"single": single.String(), "list": list.String()} {
		t.Run(name, func(t *testing.T) {
			assert.Contains(t, out, "Sending Type: marketing")
			assert.Contains(t, out, "Paused At: "+formatOptionalTime(domain.PausedAt))
			assert.Contains(t, out, "Pause Reason: bounce_rate")
		})
	}
}

func TestDomainPlain_ActiveDomainHasNoPauseLines(t *testing.T) {
	domain := newTestPausedDomain()
	domain.Paused, domain.PausedAt, domain.PauseReason = false, nil, nil

	var buf bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &buf, os.Stderr).HandleSingleDomain(domain, SingleConfig{}))
	assert.NotContains(t, buf.String(), "Paused At")
	assert.NotContains(t, buf.String(), "Pause Reason")
}

func TestDomainJSON_IncludesNewFields(t *testing.T) {
	var buf bytes.Buffer
	domain := newTestPausedDomain()
	selector := "news"
	domain.DKIMSelector = &selector
	require.NoError(t, testHandler(t, "json", &buf, os.Stderr).HandleSingleDomain(domain, SingleConfig{}))

	var got responses.Domain
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, "marketing", got.SendingType)
	assert.True(t, got.Paused)
	require.NotNil(t, got.PausedAt)
	assert.True(t, domain.PausedAt.Equal(*got.PausedAt))
	assert.Equal(t, domain.PauseReason, got.PauseReason)
	assert.Equal(t, domain.DKIMSelector, got.DKIMSelector)
}

func TestSingleDomainShowsItsSettingsInEveryFormat(t *testing.T) {
	tracking, returnPath, subscription, media, selector := "track", "bounce", "prefs", "media", "s2026"
	days := 90
	domain := responses.Domain{
		Domain: "example.com", TrackingSubdomain: &tracking, ReturnPathSubdomain: &returnPath,
		SubscriptionSubdomain: &subscription, MediaSubdomain: &media, DKIMSelector: &selector,
		DKIMRotationIntervalDays: &days,
	}
	render := func(h ResponseHandler) error { return h.HandleSingleDomain(&domain, SingleConfig{}) }

	settings := [][]string{
		{"Tracking Subdomain", "track"}, {"Return Path Subdomain", "bounce"},
		{"Subscription Subdomain", "prefs"}, {"Media Subdomain", "media"},
		{"DKIM Selector", "s2026"}, {"DKIM Rotation Interval", "90 days"},
	}
	cells := tableCells(renderWith(t, "table", render))
	plain := renderWith(t, "plain", render)
	for _, setting := range settings {
		assert.Contains(t, cells, setting)
		assert.Contains(t, plain, "\n"+setting[0]+": "+setting[1]+"\n")
	}
	_, rows := readCSV(t, renderWith(t, "csv", render))
	assert.Equal(t, map[string]string{
		"tracking_subdomain": "track", "return_path_subdomain": "bounce", "subscription_subdomain": "prefs",
		"media_subdomain": "media", "dkim_selector": "s2026", "dkim_rotation_interval_days": "90",
	}, map[string]string{
		"tracking_subdomain": rows[0]["tracking_subdomain"], "return_path_subdomain": rows[0]["return_path_subdomain"],
		"subscription_subdomain": rows[0]["subscription_subdomain"], "media_subdomain": rows[0]["media_subdomain"],
		"dkim_selector": rows[0]["dkim_selector"], "dkim_rotation_interval_days": rows[0]["dkim_rotation_interval_days"],
	})
}

// TestSingleDomainCSVHasTheDNSRecords checks that CSV output of a domain
// carries its DNS records, as table and plain output do, as a JSON array.
func TestSingleDomainCSVHasTheDNSRecords(t *testing.T) {
	domain := styleDomain()
	domain.DNSRecords = []responses.DNSRecord{{Type: "TXT", Host: "_dmarc.example.com", Content: "v=DMARC1; p=none;", Required: true}}

	_, rows := readCSV(t, renderWith(t, "csv", func(h ResponseHandler) error { return h.HandleSingleDomain(&domain, SingleConfig{}) }))

	require.Len(t, rows, 1)
	assert.JSONEq(t, `[{"type":"TXT","host":"_dmarc.example.com","content":"v=DMARC1; p=none;","required":true,"propagated":false}]`,
		rows[0]["dns_records"])
}
