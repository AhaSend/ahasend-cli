package printer

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/AhaSend/ahasend-cli/internal/webhooks"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
)

// Common formatting utilities for all output formats

// Notices that follow a created resource in table and plain output.
const (
	webhookSecretNotice = "Save the secret now. It will not be shown again. Use it to verify webhook signatures."
	smtpPasswordNotice  = "Save the password now. It will not be shown again."
	apiKeySecretNotice  = "Save the secret key now. It will not be shown again, except to a retry with the same --idempotency-key within the 5-minute replay window."
	routeSecretNotice   = "Save the secret now. It will not be shown again. Use it to verify route signatures."
	routeDisabledNotice = "The route is disabled. Enable it to start processing emails."
)

// hiddenSecret stands in table and plain output for a secret or password,
// which the API returns only on creation.
const hiddenSecret = "[HIDDEN]"

// displayTimeLayout is the layout of times in table and plain output: local
// time followed by the zone abbreviation, so a reader can tell the zone.
const displayTimeLayout = "2006-01-02 15:04:05 MST"

// DisplayTime formats t as table and plain output show times, in the local
// zone, for the commands that show a time outside a printer, such as in a
// prompt.
func DisplayTime(t time.Time) string {
	return t.Local().Format(displayTimeLayout)
}

// formatTime formats a time for table and plain output in the local zone, or
// "" when it is zero, which those formats treat as missing.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return DisplayTime(t)
}

// formatOptionalTime formats an optional time like formatTime, or "" when it
// is nil
func formatOptionalTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}

// formatCSVTime formats a time for csv output as RFC 3339 with the offset the
// API sent, or "" when it is zero.
func formatCSVTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// formatOptionalCSVTime formats an optional time like formatCSVTime, or ""
// when it is nil
func formatOptionalCSVTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatCSVTime(*t)
}

// formatBooleanStatus formats boolean values as Yes/No for human readability
func formatBooleanStatus(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

// formatDNSStatus formats DNS validation status
func formatDNSStatus(valid bool) string {
	if valid {
		return "Valid"
	}
	return "Invalid"
}

// domainPausedAt returns when a paused domain was paused, or nil when the
// domain is not paused.
func domainPausedAt(domain *responses.Domain) *time.Time {
	if !domain.Paused {
		return nil
	}
	return domain.PausedAt
}

// formatDomainPauseReason returns why a paused domain was paused, or "" when
// the domain is not paused.
func formatDomainPauseReason(domain *responses.Domain) string {
	if !domain.Paused {
		return ""
	}
	return formatOptionalString(domain.PauseReason)
}

// formatOptionalString handles nil string pointers safely
func formatOptionalString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// formatStringSlice joins the values of a field with ", ", the separator of
// every output format
func formatStringSlice(slice []string) string {
	if len(slice) == 0 {
		return ""
	}
	return strings.Join(slice, ", ")
}

// formatTags joins tags with ", " like formatStringSlice. A tag that holds a
// comma or a double quote is written in double quotes, with each double quote
// in it doubled, as CSV quotes a field, so the list reads back as the tags it
// holds: news, "a, b".
func formatTags(tags []string) string {
	quoted := make([]string, len(tags))
	for i, tag := range tags {
		if strings.ContainsAny(tag, `,"`) {
			tag = `"` + strings.ReplaceAll(tag, `"`, `""`) + `"`
		}
		quoted[i] = tag
	}
	return formatStringSlice(quoted)
}

// formatOptionalUUID formats an optional UUID, or "" when it is nil
func formatOptionalUUID(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return formatUUID(*id)
}

// formatUUID formats a UUID, or "" for the zero UUID, which the API sends for
// an ID that is not set
func formatUUID(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}

// formatInt formats an integer.
func formatInt(i int) string {
	return strconv.Itoa(i)
}

// formatOptionalInt formats an integer, or "" when it is nil.
func formatOptionalInt(i *int) string {
	if i == nil {
		return ""
	}
	return formatInt(*i)
}

// formatUint64 formats an unsigned integer.
func formatUint64(i uint64) string {
	return strconv.FormatUint(i, 10)
}

// formatBool formats a bool as true or false, the CSV value of a flag.
func formatBool(b bool) string {
	return strconv.FormatBool(b)
}

// formatOptionalBool formats a bool like formatBool, or "" when it is nil.
func formatOptionalBool(value *bool) string {
	if value == nil {
		return ""
	}
	return formatBool(*value)
}

// formatInt64 formats a 64-bit integer.
func formatInt64(i int64) string {
	return strconv.FormatInt(i, 10)
}

// formatFloat64 formats a number with two decimals: a number table and plain
// output show, or one the CLI computes, such as a percentage of a total.
func formatFloat64(f float64) string {
	return fmt.Sprintf("%.2f", f)
}

// formatCSVNumber formats a number the API sent for CSV output, in the
// shortest form that reads back as the same number, so CSV output keeps
// every digit the API sent.
func formatCSVNumber(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// formatOptionalCSVNumber formats a number like formatCSVNumber, or "" when
// it is nil.
func formatOptionalCSVNumber(f *float64) string {
	if f == nil {
		return ""
	}
	return formatCSVNumber(*f)
}

// formatPercent formats a percentage for table and plain output.
func formatPercent(percentage float64) string {
	return formatFloat64(percentage) + "%"
}

// formatRate formats a rate as a percentage for table and plain output, or
// missing when there is no rate.
func formatRate(rate *float64, missing string) string {
	if rate == nil {
		return missing
	}
	return formatPercent(*rate)
}

// formatOptionalCSVRate formats a rate for csv, a bare number without the
// percent sign, or "" when there is no rate.
func formatOptionalCSVRate(rate *float64) string {
	if rate == nil {
		return ""
	}
	return formatFloat64(*rate)
}

// Webhook-specific utility functions

// formatWebhookEvents formats webhook event subscriptions as a comma-separated
// list of event names, in the order of webhooks.Events
func formatWebhookEvents(webhook *responses.Webhook) string {
	subscribed := map[string]bool{
		"reception":           webhook.OnReception,
		"delivered":           webhook.OnDelivered,
		"transient_error":     webhook.OnTransientError,
		"failed":              webhook.OnFailed,
		"bounced":             webhook.OnBounced,
		"suppressed":          webhook.OnSuppressed,
		"opened":              webhook.OnOpened,
		"clicked":             webhook.OnClicked,
		"suppression_created": webhook.OnSuppressionCreated,
		"dns_error":           webhook.OnDNSError,
	}
	var events []string
	for _, event := range webhooks.Events {
		if subscribed[event.Name] {
			events = append(events, event.Name)
		}
	}

	if len(events) == 0 {
		return "none"
	}
	return strings.Join(events, ", ")
}

// formatDomains returns the domains a webhook or an SMTP credential is
// limited to, for table and plain output, or "all" when it has none and works
// for every domain. CSV output leaves the column empty instead, as it does
// every missing value.
func formatDomains(domains []string) string {
	if len(domains) == 0 {
		return "all"
	}
	return strings.Join(domains, ", ")
}

// batchResultHeaders name the columns of a batch row in table output.
var batchResultHeaders = []any{"Position", "Email", "Contact ID", "Outcome", "Reason"}

// batchResultCSVHeaders name the columns of a batch row in csv output.
var batchResultCSVHeaders = []string{"position", "email", "contact_id", "outcome", "reason"}

// writeBatchResult writes the result number, counted from 1 like every
// "Result" of plain output, of a batch in plain output from its batch row:
// position, the API's index of the input counted from 0, email, contact ID,
// outcome and reason.
func writeBatchResult(w io.Writer, number int, row []string) error {
	if _, err := fmt.Fprintf(w, "\nResult %d:\n", number); err != nil {
		return err
	}
	return writeLabelLines(w, "  ", [][]string{
		{"Position", row[0]},
		{"Email", row[1]},
		{"Contact ID", row[2]},
		{"Outcome", row[3]},
		{"Reason", row[4]},
	})
}

// writeLabelLines writes each row as a "label: value" line after indent.
// Rows with an empty value are left out, as plain output skips unset fields.
// Each further line of a value of several lines is indented to start under
// the value's first line, so it cannot be read as a label line.
func writeLabelLines(w io.Writer, indent string, rows [][]string) error {
	for _, row := range rows {
		if row[1] == "" {
			continue
		}
		prefix := indent + row[0] + ": "
		value := strings.ReplaceAll(row[1], "\n", "\n"+strings.Repeat(" ", utf8.RuneCountInString(prefix)))
		if _, err := fmt.Fprintf(w, "%s%s\n", prefix, value); err != nil {
			return err
		}
	}
	return nil
}

// writeIndentedList writes a "label:" line followed by each item on its own
// indented line.
func writeIndentedList(w io.Writer, label string, items []string) error {
	if _, err := fmt.Fprintf(w, "%s:\n", label); err != nil {
		return err
	}
	for _, item := range items {
		if _, err := fmt.Fprintf(w, "  %s\n", item); err != nil {
			return err
		}
	}
	return nil
}

// stableJSON encodes free-form values, such as contact attributes, as compact
// JSON with sorted keys. A nil value prints as empty rather than "null".
func stableJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	if string(encoded) == "null" {
		return "", nil
	}
	return string(encoded), nil
}

// withoutField returns fields without field.
func withoutField(fields []string, field string) []string {
	return slices.DeleteFunc(slices.Clone(fields), func(f string) bool { return f == field })
}

// fieldHeaders returns the header of each field, as table.Header takes them.
func fieldHeaders(fields []string, headers map[string]string) []any {
	row := make([]any, len(fields))
	for i, field := range fields {
		row[i] = headers[field]
	}
	return row
}

// fieldRows returns a label and value row for each of fields, the layout of a
// table's field rows and of plain output.
func fieldRows(fields []string, headers, values map[string]string) [][]string {
	rows := make([][]string, len(fields))
	for i, field := range fields {
		rows[i] = []string{headers[field], values[field]}
	}
	return rows
}

// fieldValues returns the value of each field.
func fieldValues(fields []string, values map[string]string) []string {
	row := make([]string, len(fields))
	for i, field := range fields {
		row[i] = values[field]
	}
	return row
}
