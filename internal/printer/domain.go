package printer

import (
	"fmt"
	"slices"

	"github.com/AhaSend/ahasend-go/models/responses"
)

// The fields of the views of a domain, which the table, plain and CSV output
// of each view show in this order, except a table of domains, which shows
// domainTableColumns.
var (
	// domainListFields are the fields of each domain of a list.
	domainListFields = []string{
		"domain", "id", "dns_valid", "sending_type", "paused_at", "pause_reason",
		"created_at", "updated_at", "last_dns_check_at",
	}
	// domainTableColumns are the fields a table of domains shows.
	domainTableColumns = []string{"domain", "id", "dns_valid", "sending_type", "paused_at", "pause_reason"}
	// domainDetailFields are the fields of a domain's detail view. Table and
	// plain output show the pause fields only for a paused domain.
	domainDetailFields = []string{
		"domain", "id", "account_id", "dns_valid", "sending_type", "paused_at",
		"pause_reason", "tracking_subdomain", "return_path_subdomain",
		"subscription_subdomain", "media_subdomain", "dkim_selector",
		"dkim_rotation_interval_days", "created_at", "updated_at", "last_dns_check_at",
	}
)

// domainHeaders names the table column or row of each domain field.
var domainHeaders = map[string]string{
	"domain":                      "Domain",
	"id":                          "ID",
	"account_id":                  "Account ID",
	"dns_valid":                   "DNS Status",
	"sending_type":                "Sending Type",
	"paused_at":                   "Paused At",
	"pause_reason":                "Pause Reason",
	"tracking_subdomain":          "Tracking Subdomain",
	"return_path_subdomain":       "Return Path Subdomain",
	"subscription_subdomain":      "Subscription Subdomain",
	"media_subdomain":             "Media Subdomain",
	"dkim_selector":               "DKIM Selector",
	"dkim_rotation_interval_days": "DKIM Rotation Interval",
	"created_at":                  "Created",
	"updated_at":                  "Updated",
	"last_dns_check_at":           "Last DNS Check",
}

func (h *jsonHandler) HandleDomainList(response *responses.PaginatedDomainsResponse, config ListConfig) error {
	return h.printAPIResponse(response)
}

func (h *jsonHandler) HandleSingleDomain(domain *responses.Domain, config SingleConfig) error {
	return h.printAPIResponse(domain)
}

func (h *tableHandler) HandleDomainList(response *responses.PaginatedDomainsResponse, config ListConfig) error {
	return writeTableList(h, config, pageOf(response), "domains", domainTableColumns, domainHeaders, domainDisplayFields)
}

func (h *tableHandler) HandleSingleDomain(domain *responses.Domain, config SingleConfig) error {
	if domain == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}

	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderFieldTable(domainDetailRows(domain))

	if len(domain.DNSRecords) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(h.writer, "\nDNS Records:\n"); err != nil {
		return err
	}
	rows := make([][]string, len(domain.DNSRecords))
	for i, record := range domain.DNSRecords {
		rows[i] = []string{
			record.Host,
			fmt.Sprintf("Type: %s\nContent: %s\nRequired: %s\nPropagated: %s\n",
				record.Type, record.Content,
				formatBooleanStatus(record.Required), formatBooleanStatus(record.Propagated)),
		}
	}
	h.renderTable([]any{"Host", "Record"}, rows...)
	return nil
}

func (h *plainHandler) HandleDomainList(response *responses.PaginatedDomainsResponse, config ListConfig) error {
	return writePlainList(h, config, pageOf(response), "domains", func(domain *responses.Domain) error {
		return h.writeFieldLines(domainListFields, domainHeaders, domainDisplayFields(domain))
	})
}

func (h *plainHandler) HandleSingleDomain(domain *responses.Domain, config SingleConfig) error {
	if domain == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}

	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	if err := writeLabelLines(h.writer, "", domainDetailRows(domain)); err != nil {
		return err
	}

	if len(domain.DNSRecords) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(h.writer, "\nDNS Records:\n"); err != nil {
		return err
	}
	for i, record := range domain.DNSRecords {
		if _, err := fmt.Fprintf(h.writer, "  %d. Type: %s, Host: %s, Content: %s, Required: %s, Propagated: %s\n",
			i+1, record.Type, record.Host, record.Content,
			formatBooleanStatus(record.Required), formatBooleanStatus(record.Propagated)); err != nil {
			return err
		}
	}
	return nil
}

func (h *csvHandler) HandleDomainList(response *responses.PaginatedDomainsResponse, config ListConfig) error {
	return writeCSVList(h, pageOf(response), domainListFields, domainCSVFields)
}

// HandleSingleDomain writes the domain's detail fields, then its DNS records
// as a JSON array in the dns_records column, as table and plain output show
// them after the details.
func (h *csvHandler) HandleSingleDomain(domain *responses.Domain, config SingleConfig) error {
	if domain == nil {
		return nil // No CSV output for empty data
	}
	fields := domainCSVFields(domain)
	records, err := stableJSON(domain.DNSRecords)
	if err != nil {
		return err
	}
	fields["dns_records"] = records
	return h.writeFieldMaps(append(slices.Clone(domainDetailFields), "dns_records"), fields)
}

func domainCSVFields(domain *responses.Domain) map[string]string {
	return map[string]string{
		"domain":            domain.Domain,
		"id":                formatUUID(domain.ID),
		"account_id":        formatUUID(domain.AccountID),
		"dns_valid":         formatBool(domain.DNSValid),
		"created_at":        formatCSVTime(domain.CreatedAt),
		"updated_at":        formatCSVTime(domain.UpdatedAt),
		"last_dns_check_at": formatOptionalCSVTime(domain.LastDNSCheckAt),
		"sending_type":      domain.SendingType,
		"paused_at":         formatOptionalCSVTime(domainPausedAt(domain)),
		"pause_reason":      formatDomainPauseReason(domain),

		"tracking_subdomain":          formatOptionalString(domain.TrackingSubdomain),
		"return_path_subdomain":       formatOptionalString(domain.ReturnPathSubdomain),
		"subscription_subdomain":      formatOptionalString(domain.SubscriptionSubdomain),
		"media_subdomain":             formatOptionalString(domain.MediaSubdomain),
		"dkim_selector":               formatOptionalString(domain.DKIMSelector),
		"dkim_rotation_interval_days": formatOptionalInt(domain.DKIMRotationIntervalDays),
	}
}

// domainDisplayFields maps each domain field to its table and plain value.
func domainDisplayFields(domain *responses.Domain) map[string]string {
	return map[string]string{
		"domain":                      domain.Domain,
		"id":                          formatUUID(domain.ID),
		"account_id":                  formatUUID(domain.AccountID),
		"dns_valid":                   formatDNSStatus(domain.DNSValid),
		"sending_type":                domain.SendingType,
		"paused_at":                   formatOptionalTime(domainPausedAt(domain)),
		"pause_reason":                formatDomainPauseReason(domain),
		"tracking_subdomain":          formatOptionalString(domain.TrackingSubdomain),
		"return_path_subdomain":       formatOptionalString(domain.ReturnPathSubdomain),
		"subscription_subdomain":      formatOptionalString(domain.SubscriptionSubdomain),
		"media_subdomain":             formatOptionalString(domain.MediaSubdomain),
		"dkim_selector":               formatOptionalString(domain.DKIMSelector),
		"dkim_rotation_interval_days": formatOptionalDays(domain.DKIMRotationIntervalDays),
		"created_at":                  formatTime(domain.CreatedAt),
		"updated_at":                  formatTime(domain.UpdatedAt),
		"last_dns_check_at":           formatOptionalTime(domain.LastDNSCheckAt),
	}
}

// domainDetailRows returns the rows of a domain's detail view. The pause rows
// appear only for a paused domain.
func domainDetailRows(domain *responses.Domain) [][]string {
	values := domainDisplayFields(domain)
	var rows [][]string
	for _, field := range domainDetailFields {
		if !domain.Paused && (field == "paused_at" || field == "pause_reason") {
			continue
		}
		rows = append(rows, []string{domainHeaders[field], values[field]})
	}
	return rows
}

// formatOptionalDays formats a number of days, or "" when it is nil.
func formatOptionalDays(days *int) string {
	if days == nil {
		return ""
	}
	return fmt.Sprintf("%d days", *days)
}
