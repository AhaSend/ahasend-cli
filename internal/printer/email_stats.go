package printer

import (
	"fmt"

	"github.com/AhaSend/ahasend-go/models/responses"
)

// emailStatsLabels name the counters of email statistics, in the order of
// the API's fields: the table's columns and plain output's lines.
var emailStatsLabels = []string{
	"Reception", "Delivered", "Bounced", "Suppressed", "Complained", "Deferred", "Opened", "Clicked",
}

// emailStatsCSVHeaders name the CSV columns of a row of email statistics,
// with the API's field names.
var emailStatsCSVHeaders = []string{
	"from_timestamp", "to_timestamp", "sender_domain",
	"reception_count", "delivered_count", "bounced_count", "suppressed_count",
	"complained_count", "deferred_count", "opened_count", "clicked_count",
}

// noEmailStatsRows is what table and plain output show after the totals
// when the response has no rows, as a breakdown has when no domain has
// counts in the range.
const noEmailStatsRows = "No domain has email statistics in this time range"

func (h *jsonHandler) HandleEmailStats(response *responses.EmailStatisticsResponse, config StatsConfig) error {
	return h.printAPIResponse(response)
}

func (h *tableHandler) HandleEmailStats(response *responses.EmailStatisticsResponse, config StatsConfig) error {
	if err := writeStatsTitle(h.writer, config); err != nil {
		return err
	}
	if _, err := fmt.Fprint(h.writer, "Totals:\n\n"); err != nil {
		return err
	}
	h.renderTable(emailStatsHeaders(false), append(
		[]string{formatTimePeriod(response.FromTime, response.ToTime)}, emailStatsValues(&response.Totals)...))

	if len(response.Data) == 0 {
		_, err := fmt.Fprintf(h.writer, "\n%s\n", noEmailStatsRows)
		return err
	}
	byDomain := response.Breakdown != nil
	heading := "By Period"
	if byDomain {
		heading = "By Domain and Period"
	}
	if _, err := fmt.Fprintf(h.writer, "\n%s:\n\n", heading); err != nil {
		return err
	}
	rows := make([][]string, len(response.Data))
	for i := range response.Data {
		row := &response.Data[i]
		cells := []string{formatTimePeriod(row.FromTimestamp, row.ToTimestamp)}
		if byDomain {
			cells = append(cells, formatOptionalString(row.SenderDomain))
		}
		rows[i] = append(cells, emailStatsValues(&row.EmailStatisticsTotals)...)
	}
	h.renderTable(emailStatsHeaders(byDomain), rows...)
	return nil
}

func (h *plainHandler) HandleEmailStats(response *responses.EmailStatisticsResponse, config StatsConfig) error {
	if err := writeStatsTitle(h.writer, config); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(h.writer, "Totals:"); err != nil {
		return err
	}
	totals := append([][]string{{"Time Period", formatTimePeriod(response.FromTime, response.ToTime)}},
		emailStatsLines(&response.Totals)...)
	if err := writeLabelLines(h.writer, "  ", totals); err != nil {
		return err
	}

	if len(response.Data) == 0 {
		_, err := fmt.Fprintf(h.writer, "\n%s\n", noEmailStatsRows)
		return err
	}
	for i := range response.Data {
		row := &response.Data[i]
		if _, err := fmt.Fprintln(h.writer); err != nil {
			return err
		}
		if err := writeTimePeriod(h.writer, row.FromTimestamp, row.ToTimestamp); err != nil {
			return err
		}
		// Sender Domain is left out without a breakdown, where it is nil
		lines := append([][]string{{"Sender Domain", formatOptionalString(row.SenderDomain)}},
			emailStatsLines(&row.EmailStatisticsTotals)...)
		if err := writeLabelLines(h.writer, "  ", lines); err != nil {
			return err
		}
	}
	return nil
}

func (h *csvHandler) HandleEmailStats(response *responses.EmailStatisticsResponse, config StatsConfig) error {
	rows := make([][]string, len(response.Data))
	for i := range response.Data {
		row := &response.Data[i]
		rows[i] = append([]string{
			formatCSVTime(row.FromTimestamp),
			formatCSVTime(row.ToTimestamp),
			formatOptionalString(row.SenderDomain),
		}, emailStatsValues(&row.EmailStatisticsTotals)...)
	}
	return h.writeRows(emailStatsCSVHeaders, rows...)
}

// emailStatsHeaders returns the table headers of email statistics: the time
// period, the sender domain of a breakdown by domain, and the counters.
func emailStatsHeaders(byDomain bool) []any {
	headers := []any{"Time Period"}
	if byDomain {
		headers = append(headers, "Sender Domain")
	}
	for _, label := range emailStatsLabels {
		headers = append(headers, label)
	}
	return headers
}

// emailStatsValues returns the value of each of emailStatsLabels.
func emailStatsValues(counts *responses.EmailStatisticsTotals) []string {
	return []string{
		formatInt64(counts.ReceptionCount),
		formatInt64(counts.DeliveredCount),
		formatInt64(counts.BouncedCount),
		formatInt64(counts.SuppressedCount),
		formatInt64(counts.ComplainedCount),
		formatInt64(counts.DeferredCount),
		formatInt64(counts.OpenedCount),
		formatInt64(counts.ClickedCount),
	}
}

// emailStatsLines returns the "Label: value" line of each counter for plain
// output.
func emailStatsLines(counts *responses.EmailStatisticsTotals) [][]string {
	values := emailStatsValues(counts)
	lines := make([][]string, len(emailStatsLabels))
	for i, label := range emailStatsLabels {
		lines[i] = []string{label, values[i]}
	}
	return lines
}
