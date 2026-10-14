package printer

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// JSON pass-through: the SDK struct must be emitted verbatim, with no CLI-added
// wrapper fields (no "empty"/"success"/"message" envelope).

func TestJSONSubAccountListPassThrough(t *testing.T) {
	var buf bytes.Buffer
	handler := testHandler(t, "json", &buf, os.Stderr)

	subAccount := *mocks.NewSubAccount()
	response := &responses.PaginatedSubAccountsResponse{
		Object: "list",
		Data:   []responses.SubAccount{subAccount},
	}

	require.NoError(t, handler.HandleSubAccountList(response, ListConfig{EmptyMessage: "No sub-accounts"}))

	var decoded responses.PaginatedSubAccountsResponse
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	require.Len(t, decoded.Data, 1)
	assert.Equal(t, subAccount.ID, decoded.Data[0].ID)
	assert.Equal(t, subAccount.ParentAccountID, decoded.Data[0].ParentAccountID)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &raw))
	assert.NotContains(t, raw, "empty")
	assert.NotContains(t, raw, "success")
	assert.NotContains(t, raw, "message")
	assert.Contains(t, raw, "data")
}

func TestJSONSingleSubAccountPassThrough(t *testing.T) {
	var buf bytes.Buffer
	handler := testHandler(t, "json", &buf, os.Stderr)

	subAccount := *mocks.NewSubAccount()
	require.NoError(t, handler.HandleSingleSubAccount(&subAccount, SingleConfig{SuccessMessage: "ok"}))

	var decoded responses.SubAccount
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	assert.Equal(t, subAccount.ID, decoded.ID)
	assert.Equal(t, subAccount.ParentAccountID, decoded.ParentAccountID)
	assert.Equal(t, subAccount.Name, decoded.Name)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &raw))
	assert.NotContains(t, raw, "success")
	assert.NotContains(t, raw, "message")
	assert.Equal(t, "22222222-2222-2222-2222-222222222222", raw["parent_account_id"])
}

func TestJSONSubAccountUsagePassThrough(t *testing.T) {
	var buf bytes.Buffer
	handler := testHandler(t, "json", &buf, os.Stderr)

	usage := mocks.NewSubAccountUsage(0)
	require.NoError(t, handler.HandleSubAccountUsage(usage, SingleConfig{}))

	var decoded responses.SubAccountUsageResponse
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	assert.Equal(t, usage.Total.ReceptionCount, decoded.Total.ReceptionCount)
	assert.Equal(t, usage.Total.AllocatedCost, decoded.Total.AllocatedCost)
	require.Len(t, decoded.SubAccounts, 1)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &raw))
	assert.Contains(t, raw, "billing_period")
	assert.Contains(t, raw, "total")
	assert.NotContains(t, raw, "empty")
	assert.NotContains(t, raw, "message")
}

// Table / plain usage output: headers, parent row, removed-row suppression, total row.

func TestTableSubAccountUsageOrderingAndSuppression(t *testing.T) {
	var buf bytes.Buffer
	handler := testHandler(t, "table", &buf, os.Stderr)

	usage := mocks.NewSubAccountUsage(0) // removed has no reception → suppressed
	require.NoError(t, handler.HandleSubAccountUsage(usage, SingleConfig{}))

	out := buf.String()
	assert.Contains(t, out, "Billing Period")
	assert.Contains(t, out, "proportional")
	assert.Contains(t, out, "Parent")
	assert.Contains(t, out, "Acme Subsidiary")
	assert.Contains(t, out, "Total")
	assert.NotContains(t, out, "Removed Sub-Accounts")

	// Ordering: Parent appears before the sub-account, which appears before Total.
	assert.Less(t, strings.Index(out, "Parent"), strings.Index(out, "Acme Subsidiary"))
	assert.Less(t, strings.Index(out, "Acme Subsidiary"), strings.Index(out, "Total"))
}

func TestTableSubAccountUsageShowsRemovedWhenReceptionPositive(t *testing.T) {
	var buf bytes.Buffer
	handler := testHandler(t, "table", &buf, os.Stderr)

	usage := mocks.NewSubAccountUsage(50) // removed has reception → shown
	require.NoError(t, handler.HandleSubAccountUsage(usage, SingleConfig{}))

	out := buf.String()
	assert.Contains(t, out, "Removed Sub-Accounts")
	// Removed row appears after the sub-account and before the total.
	assert.Less(t, strings.Index(out, "Acme Subsidiary"), strings.Index(out, "Removed Sub-Accounts"))
	assert.Less(t, strings.Index(out, "Removed Sub-Accounts"), strings.Index(out, "Total"))
}

func TestPlainSubAccountUsageRowsAndSuppression(t *testing.T) {
	var buf bytes.Buffer
	handler := testHandler(t, "plain", &buf, os.Stderr)

	usage := mocks.NewSubAccountUsage(0)
	require.NoError(t, handler.HandleSubAccountUsage(usage, SingleConfig{}))

	out := buf.String()
	assert.Contains(t, out, "Allocation Method: proportional")
	assert.Contains(t, out, "Parent:")
	assert.Contains(t, out, "Total:")
	assert.Contains(t, out, "Reception: 4000")
	assert.NotContains(t, out, "Removed Sub-Accounts:")
}

func TestTableAndPlainSingleSubAccountRendersParent(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			var buf bytes.Buffer
			handler := testHandler(t, format, &buf, os.Stderr)

			subAccount := *mocks.NewSubAccount()
			require.NoError(t, handler.HandleSingleSubAccount(&subAccount, SingleConfig{SuccessMessage: "ok"}))

			out := buf.String()
			assert.Contains(t, out, "Acme Subsidiary")
			assert.Contains(t, out, "22222222-2222-2222-2222-222222222222") // parent account ID rendered
		})
	}
}

// CSV usage flattening: header order and per-row content from response.Total.

func TestCSVSubAccountUsageFlattening(t *testing.T) {
	var buf bytes.Buffer
	handler := testHandler(t, "csv", &buf, os.Stderr)

	usage := mocks.NewSubAccountUsage(0) // removed suppressed
	usage.AllocationNote = "Split by reception"
	require.NoError(t, handler.HandleSubAccountUsage(usage, SingleConfig{}))

	records, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(records), 2)

	expectedHeaders := []string{
		"billing_period_start", "billing_period_end", "currency", "allocation_method", "allocation_note",
		"account_name", "account_id", "reception_count", "allocated_cost",
	}
	assert.Equal(t, expectedHeaders, records[0])

	// Data rows: parent, sub-account, total (removed suppressed).
	require.Len(t, records, 4)
	assert.Equal(t, "Parent", records[1][5])
	assert.Equal(t, "Acme Subsidiary", records[2][5])

	total := records[3]
	assert.Equal(t, "Total", total[5])
	assert.Equal(t, "4000", total[7]) // reception_count from response.Total
	assert.Equal(t, "40", total[8])   // allocated_cost from response.Total, as the API sent it

	// Every row repeats the billing period as RFC 3339 times, the currency,
	// and the allocation method and note.
	for _, row := range records[1:] {
		assert.Equal(t, usage.BillingPeriod.Start.Format(time.RFC3339), row[0])
		assert.Equal(t, usage.BillingPeriod.End.Format(time.RFC3339), row[1])
		assert.Equal(t, "usd", row[2])
		assert.Equal(t, "proportional", row[3])
		assert.Equal(t, "Split by reception", row[4])
	}
}

// TestCSVSubAccountUsageKeepsEveryDigitOfTheCost checks that CSV output
// prints the allocated cost as the API sent it, where table and plain output
// round it for display.
func TestCSVSubAccountUsageKeepsEveryDigitOfTheCost(t *testing.T) {
	usage := mocks.NewSubAccountUsage(0)
	usage.Total.AllocatedCost = 0.123456

	assert.Contains(t, renderWith(t, "csv", func(h ResponseHandler) error { return h.HandleSubAccountUsage(usage, SingleConfig{}) }),
		",4000,0.123456\n")
	assert.Contains(t, renderWith(t, "plain", func(h ResponseHandler) error { return h.HandleSubAccountUsage(usage, SingleConfig{}) }),
		"Allocated Cost: 0.12\n")
}

func TestCSVSubAccountUsageIncludesRemovedWhenPositive(t *testing.T) {
	var buf bytes.Buffer
	handler := testHandler(t, "csv", &buf, os.Stderr)

	usage := mocks.NewSubAccountUsage(50)
	require.NoError(t, handler.HandleSubAccountUsage(usage, SingleConfig{}))

	records, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	require.NoError(t, err)

	// header + parent + sub-account + removed + total
	require.Len(t, records, 5)
	assert.Equal(t, "Removed Sub-Accounts", records[3][5])
	assert.Equal(t, "Total", records[4][5])
}

func TestCSVSubAccountListHeaders(t *testing.T) {
	var buf bytes.Buffer
	handler := testHandler(t, "csv", &buf, os.Stderr)

	subAccount := *mocks.NewSubAccount()
	response := &responses.PaginatedSubAccountsResponse{Data: []responses.SubAccount{subAccount}}
	require.NoError(t, handler.HandleSubAccountList(response, ListConfig{}))

	records, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, []string{
		"name", "id", "status", "website", "monthly_credit", "domain_count", "member_count", "created_at",
	}, records[0])
	assert.Equal(t, "Acme Subsidiary", records[1][0])
}

// TestCreateAPIKeyNoticeNamesTheReplayWindow checks that the one notice after
// a created key's secret says the secret is shown again only to a retry within
// the 5-minute replay window.
func TestCreateAPIKeyNoticeNamesTheReplayWindow(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			var buf bytes.Buffer
			handler := testHandler(t, format, &buf, os.Stderr)

			secret := "secret-value"
			key := &responses.APIKey{
				ID:        uuid.MustParse("44444444-4444-4444-4444-444444444444"),
				AccountID: uuid.MustParse("55555555-5555-5555-5555-555555555555"),
				Label:     "ci",
				PublicKey: "pub",
				SecretKey: &secret,
			}
			require.NoError(t, handler.HandleCreateAPIKey(key, ResultConfig{SuccessMessage: "Created"}))

			out := buf.String()
			assert.Equal(t, 1, strings.Count(out, apiKeySecretNotice))
			assert.Contains(t, apiKeySecretNotice, "5-minute replay window")
			assert.Contains(t, out, "secret-value")
		})
	}
}

func TestPlainSubAccountUsageLeavesOutAMissingBillingPeriod(t *testing.T) {
	var buf bytes.Buffer
	handler := testHandler(t, "plain", &buf, os.Stderr)

	usage := mocks.NewSubAccountUsage(0)
	usage.BillingPeriod = responses.SubAccountUsageBillingPeriod{}
	require.NoError(t, handler.HandleSubAccountUsage(usage, SingleConfig{}))

	assert.NotContains(t, buf.String(), "Billing Period")
}

func TestSubAccountBillingPeriodShowsAMissingEndAsADash(t *testing.T) {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	got := formatSubAccountBillingPeriod(responses.SubAccountUsageBillingPeriod{Start: start})
	assert.Equal(t, DisplayTime(start)+" to -", got)
}
