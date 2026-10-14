package printer

import (
	"fmt"

	"github.com/AhaSend/ahasend-go/models/responses"
)

// The fields of the views of a sub-account, which the table, plain and CSV
// output of each view show in this order, except a table of sub-accounts,
// which shows subAccountTableColumns.
var (
	// subAccountListFields are the fields of each sub-account of a list.
	subAccountListFields = []string{
		"name", "id", "status", "website", "monthly_credit", "domain_count", "member_count", "created_at",
	}
	// subAccountTableColumns are the fields a table of sub-accounts shows.
	subAccountTableColumns = []string{"name", "id", "status", "website", "monthly_credit", "created_at"}
	// subAccountDetailFields are the fields of a sub-account's detail view.
	subAccountDetailFields = []string{
		"name", "id", "parent_account_id", "status", "website", "monthly_credit",
		"domain_count", "member_count", "created_at", "last_activity_at",
	}
)

// subAccountHeaders names the table column or row of each sub-account field.
var subAccountHeaders = map[string]string{
	"name":              "Name",
	"id":                "ID",
	"parent_account_id": "Parent Account ID",
	"status":            "Status",
	"website":           "Website",
	"monthly_credit":    "Monthly Credit",
	"domain_count":      "Domains",
	"member_count":      "Members",
	"created_at":        "Created",
	"last_activity_at":  "Last Activity",
}

func (h *jsonHandler) HandleSubAccountList(response *responses.PaginatedSubAccountsResponse, config ListConfig) error {
	return h.printAPIResponse(response)
}

func (h *jsonHandler) HandleSingleSubAccount(subAccount *responses.SubAccount, config SingleConfig) error {
	return h.printAPIResponse(subAccount)
}

func (h *jsonHandler) HandleSubAccountUsage(response *responses.SubAccountUsageResponse, config SingleConfig) error {
	return h.printAPIResponse(response)
}

func (h *tableHandler) HandleSubAccountList(response *responses.PaginatedSubAccountsResponse, config ListConfig) error {
	return writeTableList(h, config, pageOf(response), "sub-accounts", subAccountTableColumns, subAccountHeaders, subAccountDisplayFields)
}

func (h *tableHandler) HandleSingleSubAccount(subAccount *responses.SubAccount, config SingleConfig) error {
	if subAccount == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderFieldTable(subAccountDetailRows(subAccount))
	return nil
}

func (h *tableHandler) HandleSubAccountUsage(response *responses.SubAccountUsageResponse, config SingleConfig) error {
	if response == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if config.SuccessMessage != "" {
		if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
			return err
		}
	}
	if err := writeLabelLines(h.writer, "", subAccountUsageSummaryRows(response)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(h.writer); err != nil {
		return err
	}

	var rows [][]string
	for _, row := range subAccountUsageRows(response) {
		rows = append(rows, []string{
			row.name,
			row.accountID,
			formatInt(int(row.breakdown.ReceptionCount)),
			formatFloat64(row.breakdown.AllocatedCost),
		})
	}
	h.renderTable([]any{"Account", "Account ID", "Reception", "Allocated Cost"}, rows...)
	return nil
}

func (h *plainHandler) HandleSubAccountList(response *responses.PaginatedSubAccountsResponse, config ListConfig) error {
	return writePlainList(h, config, pageOf(response), "sub-accounts", func(subAccount *responses.SubAccount) error {
		return h.writeFieldLines(subAccountListFields, subAccountHeaders, subAccountDisplayFields(subAccount))
	})
}

func (h *plainHandler) HandleSingleSubAccount(subAccount *responses.SubAccount, config SingleConfig) error {
	if subAccount == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	return writeLabelLines(h.writer, "", subAccountDetailRows(subAccount))
}

func (h *plainHandler) HandleSubAccountUsage(response *responses.SubAccountUsageResponse, config SingleConfig) error {
	if response == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if config.SuccessMessage != "" {
		if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
			return err
		}
	}
	if err := writeLabelLines(h.writer, "", subAccountUsageSummaryRows(response)); err != nil {
		return err
	}

	for _, row := range subAccountUsageRows(response) {
		if _, err := fmt.Fprintf(h.writer, "\n%s:\n", row.name); err != nil {
			return err
		}
		if err := writeLabelLines(h.writer, "  ", [][]string{
			{"Account ID", row.accountID},
			{"Reception", formatInt(int(row.breakdown.ReceptionCount))},
			{"Allocated Cost", formatFloat64(row.breakdown.AllocatedCost)},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (h *csvHandler) HandleSubAccountList(response *responses.PaginatedSubAccountsResponse, config ListConfig) error {
	return writeCSVList(h, pageOf(response), subAccountListFields, subAccountCSVFields)
}

func (h *csvHandler) HandleSingleSubAccount(subAccount *responses.SubAccount, config SingleConfig) error {
	if subAccount == nil {
		return nil // No CSV output for empty data
	}
	return h.writeFieldMaps(subAccountDetailFields, subAccountCSVFields(subAccount))
}

// subAccountCSVFields maps a sub-account to its CSV field values.
func subAccountCSVFields(subAccount *responses.SubAccount) map[string]string {
	return map[string]string{
		"id":                formatUUID(subAccount.ID),
		"name":              subAccount.Name,
		"status":            subAccount.Status,
		"website":           subAccount.Website,
		"monthly_credit":    formatInt(int(subAccount.MonthlyCredit)),
		"domain_count":      formatInt(int(subAccount.DomainCount)),
		"member_count":      formatInt(int(subAccount.MemberCount)),
		"parent_account_id": formatUUID(subAccount.ParentAccountID),
		"created_at":        formatCSVTime(subAccount.CreatedAt),
		"last_activity_at":  formatOptionalCSVTime(subAccount.LastActivityAt),
	}
}

func (h *csvHandler) HandleSubAccountUsage(response *responses.SubAccountUsageResponse, config SingleConfig) error {
	if response == nil {
		return nil // No CSV output for empty data
	}

	var rows [][]string
	for _, row := range subAccountUsageRows(response) {
		rows = append(rows, []string{
			formatCSVTime(response.BillingPeriod.Start),
			formatCSVTime(response.BillingPeriod.End),
			response.Currency,
			response.AllocationMethod,
			response.AllocationNote,
			row.name,
			row.accountID,
			formatInt(int(row.breakdown.ReceptionCount)),
			formatCSVNumber(row.breakdown.AllocatedCost),
		})
	}
	return h.writeRows([]string{
		"billing_period_start", "billing_period_end", "currency", "allocation_method", "allocation_note",
		"account_name", "account_id", "reception_count", "allocated_cost",
	}, rows...)
}

// subAccountDetailRows returns the rows of a sub-account's detail view.
func subAccountDetailRows(subAccount *responses.SubAccount) [][]string {
	return fieldRows(subAccountDetailFields, subAccountHeaders, subAccountDisplayFields(subAccount))
}

// subAccountDisplayFields maps each sub-account field to its table and plain
// value.
func subAccountDisplayFields(subAccount *responses.SubAccount) map[string]string {
	return map[string]string{
		"name":              subAccount.Name,
		"id":                formatUUID(subAccount.ID),
		"parent_account_id": formatUUID(subAccount.ParentAccountID),
		"status":            subAccount.Status,
		"website":           subAccount.Website,
		"monthly_credit":    formatInt(int(subAccount.MonthlyCredit)),
		"domain_count":      formatInt(int(subAccount.DomainCount)),
		"member_count":      formatInt(int(subAccount.MemberCount)),
		"created_at":        formatTime(subAccount.CreatedAt),
		"last_activity_at":  formatOptionalTime(subAccount.LastActivityAt),
	}
}

// subAccountUsageSummaryRows returns the billing period, currency and
// allocation of a usage report.
func subAccountUsageSummaryRows(response *responses.SubAccountUsageResponse) [][]string {
	return [][]string{
		{"Billing Period", formatSubAccountBillingPeriod(response.BillingPeriod)},
		{"Currency", response.Currency},
		{"Allocation Method", response.AllocationMethod},
		{"Allocation Note", response.AllocationNote},
	}
}

// subAccountUsageRow pairs a usage breakdown with its display label and account ID.
type subAccountUsageRow struct {
	name      string
	accountID string
	breakdown responses.SubAccountUsageBreakdown
}

// subAccountUsageRows returns the usage breakdown rows in their stable display order:
// parent, then the API-ordered sub-accounts, then removed sub-accounts only when they
// have a positive reception count, then the API-provided total. The total is taken
// directly from response.Total and is never recomputed client-side.
func subAccountUsageRows(response *responses.SubAccountUsageResponse) []subAccountUsageRow {
	rows := []subAccountUsageRow{
		{
			name:      subAccountUsageName(response.Parent, "Parent"),
			accountID: subAccountUsageAccountID(response.Parent),
			breakdown: response.Parent,
		},
	}

	for _, sub := range response.SubAccounts {
		rows = append(rows, subAccountUsageRow{
			name:      subAccountUsageName(sub, "Sub-Account"),
			accountID: subAccountUsageAccountID(sub),
			breakdown: sub,
		})
	}

	// Removed sub-accounts are only shown when they carry usage.
	if response.RemovedSubAccounts.ReceptionCount > 0 {
		rows = append(rows, subAccountUsageRow{
			name:      subAccountUsageName(response.RemovedSubAccounts, "Removed Sub-Accounts"),
			accountID: subAccountUsageAccountID(response.RemovedSubAccounts),
			breakdown: response.RemovedSubAccounts,
		})
	}

	rows = append(rows, subAccountUsageRow{
		name:      subAccountUsageName(response.Total, "Total"),
		accountID: subAccountUsageAccountID(response.Total),
		breakdown: response.Total,
	})

	return rows
}

// subAccountUsageName returns the breakdown's name, falling back to the provided
// label when the API omits a name (e.g. for parent/removed/total buckets).
func subAccountUsageName(b responses.SubAccountUsageBreakdown, fallback string) string {
	if b.Name != nil && *b.Name != "" {
		return *b.Name
	}
	return fallback
}

// subAccountUsageAccountID renders the optional account ID, returning an empty
// string when it is absent.
func subAccountUsageAccountID(b responses.SubAccountUsageBreakdown) string {
	if b.AccountID != nil {
		return formatUUID(*b.AccountID)
	}
	return ""
}

// formatSubAccountBillingPeriod renders a usage billing period as a "start to
// end" range, with "-" for a missing end, or "" when both ends are missing.
func formatSubAccountBillingPeriod(period responses.SubAccountUsageBillingPeriod) string {
	if period.Start.IsZero() && period.End.IsZero() {
		return ""
	}
	return fmt.Sprintf("%s to %s", orDash(formatTime(period.Start)), orDash(formatTime(period.End)))
}

func orDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
