package printer

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSubAccount() responses.SubAccount {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	parentID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	lastActivity := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	return responses.SubAccount{
		Object:          "sub_account",
		ID:              id,
		ParentAccountID: parentID,
		CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Name:            "Acme Subsidiary",
		Website:         "https://acme.example",
		Status:          "active",
		MonthlyCredit:   500,
		DomainCount:     3,
		MemberCount:     7,
		LastActivityAt:  &lastActivity,
	}
}

func newTestSubAccountUsage(removedReception int64) *responses.SubAccountUsageResponse {
	parentID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	subID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	subName := "Acme Subsidiary"

	return &responses.SubAccountUsageResponse{
		BillingPeriod: responses.SubAccountUsageBillingPeriod{
			Start: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		},
		Currency:         "usd",
		AllocationMethod: "proportional",
		Parent: responses.SubAccountUsageBreakdown{
			AccountID:      &parentID,
			ReceptionCount: 1000,
			AllocatedCost:  10,
		},
		SubAccounts: []responses.SubAccountUsageBreakdown{
			{
				AccountID:      &subID,
				Name:           &subName,
				ReceptionCount: 3000,
				AllocatedCost:  30,
			},
		},
		RemovedSubAccounts: responses.SubAccountUsageBreakdown{
			ReceptionCount: removedReception,
			AllocatedCost:  5,
		},
		Total: responses.SubAccountUsageBreakdown{
			ReceptionCount: 4000,
			AllocatedCost:  40,
		},
	}
}

// JSON pass-through: the SDK struct must be emitted verbatim, with no CLI-added
// wrapper fields (no "empty"/"success"/"message" envelope).

func TestJSONSubAccountListPassThrough(t *testing.T) {
	var buf bytes.Buffer
	handler := GetResponseHandler("json", false, &buf)

	subAccount := newTestSubAccount()
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
	handler := GetResponseHandler("json", false, &buf)

	subAccount := newTestSubAccount()
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

func TestJSONCreateSubAccountPassThrough(t *testing.T) {
	var buf bytes.Buffer
	handler := GetResponseHandler("json", false, &buf)

	subAccount := newTestSubAccount()
	require.NoError(t, handler.HandleCreateSubAccount(&subAccount, CreateConfig{SuccessMessage: "created"}))

	var decoded responses.SubAccount
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	assert.Equal(t, subAccount.ID, decoded.ID)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &raw))
	assert.NotContains(t, raw, "success")
	assert.NotContains(t, raw, "message")
}

func TestJSONSubAccountUsagePassThrough(t *testing.T) {
	var buf bytes.Buffer
	handler := GetResponseHandler("json", false, &buf)

	usage := newTestSubAccountUsage(0)
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
	handler := GetResponseHandler("table", false, &buf)

	usage := newTestSubAccountUsage(0) // removed has no reception → suppressed
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
	handler := GetResponseHandler("table", false, &buf)

	usage := newTestSubAccountUsage(50) // removed has reception → shown
	require.NoError(t, handler.HandleSubAccountUsage(usage, SingleConfig{}))

	out := buf.String()
	assert.Contains(t, out, "Removed Sub-Accounts")
	// Removed row appears after the sub-account and before the total.
	assert.Less(t, strings.Index(out, "Acme Subsidiary"), strings.Index(out, "Removed Sub-Accounts"))
	assert.Less(t, strings.Index(out, "Removed Sub-Accounts"), strings.Index(out, "Total"))
}

func TestPlainSubAccountUsageRowsAndSuppression(t *testing.T) {
	var buf bytes.Buffer
	handler := GetResponseHandler("plain", false, &buf)

	usage := newTestSubAccountUsage(0)
	require.NoError(t, handler.HandleSubAccountUsage(usage, SingleConfig{}))

	out := buf.String()
	assert.Contains(t, out, "Allocation Method: proportional")
	assert.Contains(t, out, "Parent:")
	assert.Contains(t, out, "Total:")
	assert.Contains(t, out, "Reception Count: 4000")
	assert.NotContains(t, out, "Removed Sub-Accounts:")
}

func TestTableAndPlainSingleSubAccountRendersParent(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			var buf bytes.Buffer
			handler := GetResponseHandler(format, false, &buf)

			subAccount := newTestSubAccount()
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
	handler := GetResponseHandler("csv", false, &buf)

	usage := newTestSubAccountUsage(0) // removed suppressed
	require.NoError(t, handler.HandleSubAccountUsage(usage, SingleConfig{}))

	records, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(records), 2)

	expectedHeaders := []string{
		"billing_period", "allocation_method", "currency",
		"account_name", "account_id", "reception_count", "allocated_cost",
	}
	assert.Equal(t, expectedHeaders, records[0])

	// Data rows: parent, sub-account, total (removed suppressed).
	require.Len(t, records, 4)
	assert.Equal(t, "Parent", records[1][3])
	assert.Equal(t, "Acme Subsidiary", records[2][3])

	total := records[3]
	assert.Equal(t, "Total", total[3])
	assert.Equal(t, "4000", total[5])  // reception_count from response.Total
	assert.Equal(t, "40.00", total[6]) // allocated_cost from response.Total

	// Every row repeats billing period, allocation method, and currency.
	for _, row := range records[1:] {
		assert.Equal(t, "proportional", row[1])
		assert.Equal(t, "usd", row[2])
		assert.NotEmpty(t, row[0])
	}
}

func TestCSVSubAccountUsageIncludesRemovedWhenPositive(t *testing.T) {
	var buf bytes.Buffer
	handler := GetResponseHandler("csv", false, &buf)

	usage := newTestSubAccountUsage(50)
	require.NoError(t, handler.HandleSubAccountUsage(usage, SingleConfig{}))

	records, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	require.NoError(t, err)

	// header + parent + sub-account + removed + total
	require.Len(t, records, 5)
	assert.Equal(t, "Removed Sub-Accounts", records[3][3])
	assert.Equal(t, "Total", records[4][3])
}

func TestCSVSubAccountListHeaders(t *testing.T) {
	var buf bytes.Buffer
	handler := GetResponseHandler("csv", false, &buf)

	subAccount := newTestSubAccount()
	response := &responses.PaginatedSubAccountsResponse{Data: []responses.SubAccount{subAccount}}
	require.NoError(t, handler.HandleSubAccountList(response, ListConfig{}))

	records, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Contains(t, records[0], "parent_account_id")
	assert.Contains(t, records[0], "name")
	assert.Equal(t, "Acme Subsidiary", records[1][1])
}

// The shared HandleCreateAPIKey output must remain unchanged and must NOT carry a
// 5-minute replay-window note, even after the sub-account renderers were added.

func TestCreateAPIKeyHasNoReplayWindowNote(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			var buf bytes.Buffer
			handler := GetResponseHandler(format, false, &buf)

			secret := "secret-value"
			key := &responses.APIKey{
				ID:        uuid.MustParse("44444444-4444-4444-4444-444444444444"),
				AccountID: uuid.MustParse("55555555-5555-5555-5555-555555555555"),
				Label:     "ci",
				PublicKey: "pub",
				SecretKey: &secret,
			}
			require.NoError(t, handler.HandleCreateAPIKey(key, CreateConfig{SuccessMessage: "Created"}))

			out := buf.String()
			assert.NotContains(t, out, "5-minute")
			assert.NotContains(t, out, "5 minute")
			assert.NotContains(t, out, "replay")
			assert.Contains(t, out, "secret-value")
		})
	}
}
