package printer

import (
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

func (h *jsonHandler) HandleDeliverabilityStats(response *responses.DeliverabilityStatisticsResponse, config StatsConfig) error {
	return h.printAPIResponse(response)
}

func (h *jsonHandler) HandleBounceStats(response *responses.BounceStatisticsResponse, config StatsConfig) error {
	return h.printAPIResponse(response)
}

func (h *jsonHandler) HandleDeliveryTimeStats(response *responses.DeliveryTimeStatisticsResponse, config StatsConfig) error {
	return h.printAPIResponse(response)
}

func (h *tableHandler) HandleDeliverabilityStats(response *responses.DeliverabilityStatisticsResponse, config StatsConfig) error {
	if len(response.Data) == 0 {
		return h.HandleEmpty("No deliverability statistics found")
	}
	if err := writeStatsTitle(h.writer, config); err != nil {
		return err
	}

	headers := []any{"Time Period"}
	for _, label := range deliverabilityLabels {
		headers = append(headers, label)
	}
	rows := make([][]string, len(response.Data))
	for i := range response.Data {
		stat := &response.Data[i]
		rows[i] = append([]string{formatTimePeriod(stat.FromTimestamp, stat.ToTimestamp)}, deliverabilityValues(stat)...)
	}
	h.renderTable(headers, rows...)
	return nil
}

func (h *tableHandler) HandleBounceStats(response *responses.BounceStatisticsResponse, config StatsConfig) error {
	if len(response.Data) == 0 {
		return h.HandleEmpty("No bounce statistics found")
	}
	if err := writeStatsTitle(h.writer, config); err != nil {
		return err
	}

	// First a summary table of the time periods and their totals
	summary := make([][]string, len(response.Data))
	for i, stat := range response.Data {
		classifications := "-"
		if len(stat.Bounces) > 0 {
			classifications = formatCount(len(stat.Bounces), "types")
		}
		summary[i] = []string{
			formatTimePeriod(stat.FromTimestamp, stat.ToTimestamp),
			formatInt(totalBounces(stat.Bounces)),
			classifications,
		}
	}
	h.renderTable([]any{"Time Period", "Total Bounces", "Bounce Classifications"}, summary...)

	// Then a table of the bounce classifications of each time period
	for _, stat := range response.Data {
		if len(stat.Bounces) == 0 {
			continue
		}
		if _, err := fmt.Fprintf(h.writer, "\nBounce Classifications - %s:\n\n",
			formatTimePeriod(stat.FromTimestamp, stat.ToTimestamp)); err != nil {
			return err
		}

		total := totalBounces(stat.Bounces)
		rows := make([][]string, len(stat.Bounces))
		for j, bounce := range stat.Bounces {
			rows[j] = []string{
				bounce.Classification,
				formatInt(bounce.Count),
				formatBouncePercentage(bounce.Count, total),
			}
		}
		h.renderTable([]any{"Classification", "Count", "Percentage"}, rows...)
	}
	return nil
}

func (h *tableHandler) HandleDeliveryTimeStats(response *responses.DeliveryTimeStatisticsResponse, config StatsConfig) error {
	if len(response.Data) == 0 {
		return h.HandleEmpty("No delivery time statistics found")
	}
	if err := writeStatsTitle(h.writer, config); err != nil {
		return err
	}

	// Main summary table. Header auto-formatting would split "PER-DOMAIN"
	// at its hyphen, so the headers are written as they print.
	summaryTable := tablewriter.NewTable(h.writer, tablewriter.WithHeaderAutoFormat(tw.Off))
	summaryTable.Header("TIME PERIOD", "DELIVERED", "AVERAGE DELIVERY TIME", "PER-DOMAIN DELIVERY TIMES")
	for _, stat := range response.Data {
		perDomainCount := "-"
		if len(stat.DeliveryTimes) > 0 {
			perDomainCount = formatCount(len(stat.DeliveryTimes), "domains")
		}
		addTableRow(summaryTable, []string{
			formatTimePeriod(stat.FromTimestamp, stat.ToTimestamp),
			formatInt(stat.DeliveredCount),
			formatDeliveryTime(stat.AvgDeliveryTime),
			perDomainCount,
		})
	}
	summaryTable.Render()

	// Then the per-domain delivery times of each time period that has them
	for _, stat := range response.Data {
		if len(stat.DeliveryTimes) == 0 {
			continue
		}
		if _, err := fmt.Fprintf(h.writer, "\nPer-Domain Delivery Times - %s:\n\n",
			formatTimePeriod(stat.FromTimestamp, stat.ToTimestamp)); err != nil {
			return err
		}

		rows := make([][]string, len(stat.DeliveryTimes))
		for j, dt := range stat.DeliveryTimes {
			rows[j] = []string{formatOptionalString(dt.RecipientDomain), formatInt64(dt.Count), formatDomainDeliveryTime(dt)}
		}
		h.renderTable([]any{"Recipient Domain", "Delivered", "Delivery Time"}, rows...)
	}
	return nil
}

func (h *plainHandler) HandleDeliverabilityStats(response *responses.DeliverabilityStatisticsResponse, config StatsConfig) error {
	if len(response.Data) == 0 {
		return h.HandleEmpty("No deliverability statistics found")
	}
	if err := writeStatsTitle(h.writer, config); err != nil {
		return err
	}

	for i, stat := range response.Data {
		if err := h.writeItemSeparator(i); err != nil {
			return err
		}
		if err := writeTimePeriod(h.writer, stat.FromTimestamp, stat.ToTimestamp); err != nil {
			return err
		}
		values := deliverabilityValues(&stat)
		rows := make([][]string, len(deliverabilityLabels))
		for i, label := range deliverabilityLabels {
			rows[i] = []string{label, values[i]}
		}
		if err := writeLabelLines(h.writer, "  ", rows); err != nil {
			return err
		}
	}
	return nil
}

func (h *plainHandler) HandleBounceStats(response *responses.BounceStatisticsResponse, config StatsConfig) error {
	if len(response.Data) == 0 {
		return h.HandleEmpty("No bounce statistics found")
	}
	if err := writeStatsTitle(h.writer, config); err != nil {
		return err
	}

	for i, stat := range response.Data {
		if err := h.writeItemSeparator(i); err != nil {
			return err
		}
		if err := writeTimePeriod(h.writer, stat.FromTimestamp, stat.ToTimestamp); err != nil {
			return err
		}
		if len(stat.Bounces) == 0 {
			if _, err := fmt.Fprintf(h.writer, "  No bounces recorded\n"); err != nil {
				return err
			}
			continue
		}

		total := totalBounces(stat.Bounces)
		if _, err := fmt.Fprintf(h.writer, "  Total Bounces: %s\n  Bounce Classifications:\n", formatInt(total)); err != nil {
			return err
		}
		for _, bounce := range stat.Bounces {
			if _, err := fmt.Fprintf(h.writer, "    %s: %s (%s)\n",
				bounce.Classification, formatInt(bounce.Count), formatBouncePercentage(bounce.Count, total)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *plainHandler) HandleDeliveryTimeStats(response *responses.DeliveryTimeStatisticsResponse, config StatsConfig) error {
	if len(response.Data) == 0 {
		return h.HandleEmpty("No delivery time statistics found")
	}
	if err := writeStatsTitle(h.writer, config); err != nil {
		return err
	}

	for i, stat := range response.Data {
		if err := h.writeItemSeparator(i); err != nil {
			return err
		}
		if err := writeTimePeriod(h.writer, stat.FromTimestamp, stat.ToTimestamp); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(h.writer, "  Delivered: %s\n  Average Delivery Time: %s\n",
			formatInt(stat.DeliveredCount), formatDeliveryTime(stat.AvgDeliveryTime)); err != nil {
			return err
		}

		// Each entry the API sent shows, as in a table: a missing domain or
		// time as "-"
		if len(stat.DeliveryTimes) == 0 {
			continue
		}
		if _, err := fmt.Fprintf(h.writer, "  Per-Domain Delivery Times:\n"); err != nil {
			return err
		}
		for _, dt := range stat.DeliveryTimes {
			domain, deliveryTime := formatOptionalString(dt.RecipientDomain), formatDomainDeliveryTime(dt)
			if domain == "" {
				domain = "-"
			}
			if deliveryTime == "" {
				deliveryTime = "-"
			}
			if _, err := fmt.Fprintf(h.writer, "    %s: %s (%s delivered)\n", domain, deliveryTime, formatInt64(dt.Count)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *csvHandler) HandleDeliverabilityStats(response *responses.DeliverabilityStatisticsResponse, config StatsConfig) error {
	rows := make([][]string, len(response.Data))
	for i := range response.Data {
		stat := &response.Data[i]
		deliveryRate, openRate := deliverabilityRates(stat)
		rows[i] = []string{
			formatCSVTime(stat.FromTimestamp),
			formatCSVTime(stat.ToTimestamp),
			formatInt(stat.ReceptionCount),
			formatInt(stat.DeliveredCount),
			formatInt(stat.DeferredCount),
			formatInt(stat.BouncedCount),
			formatInt(stat.FailedCount),
			formatInt(stat.SuppressedCount),
			formatInt(stat.OpenedCount),
			formatInt(stat.ClickedCount),
			formatOptionalCSVRate(deliveryRate),
			formatOptionalCSVRate(openRate),
		}
	}
	return h.writeRows([]string{
		"from_timestamp", "to_timestamp", "reception_count", "delivered_count",
		"deferred_count", "bounced_count", "failed_count", "suppressed_count",
		"opened_count", "clicked_count", "delivery_rate", "open_rate",
	}, rows...)
}

func (h *csvHandler) HandleBounceStats(response *responses.BounceStatisticsResponse, config StatsConfig) error {
	// Each classification gets its own row, and a period without bounces one
	// row with a count of 0, as table and plain output show every period
	var rows [][]string
	for _, stat := range response.Data {
		if len(stat.Bounces) == 0 {
			rows = append(rows, []string{formatCSVTime(stat.FromTimestamp), formatCSVTime(stat.ToTimestamp), "", "0", ""})
			continue
		}
		total := totalBounces(stat.Bounces)
		for _, bounce := range stat.Bounces {
			rows = append(rows, []string{
				formatCSVTime(stat.FromTimestamp),
				formatCSVTime(stat.ToTimestamp),
				bounce.Classification,
				formatInt(bounce.Count),
				formatFloat64(bouncePercentage(bounce.Count, total)),
			})
		}
	}
	return h.writeRows([]string{"from_timestamp", "to_timestamp", "classification", "count", "percentage"}, rows...)
}

func (h *csvHandler) HandleDeliveryTimeStats(response *responses.DeliveryTimeStatisticsResponse, config StatsConfig) error {
	// One row per recipient domain, or one row for a period without any
	var rows [][]string
	for _, stat := range response.Data {
		base := []string{
			formatCSVTime(stat.FromTimestamp),
			formatCSVTime(stat.ToTimestamp),
			formatInt(stat.DeliveredCount),
			formatCSVNumber(stat.AvgDeliveryTime),
		}
		if len(stat.DeliveryTimes) == 0 {
			rows = append(rows, append(base, "", "", ""))
			continue
		}
		for _, dt := range stat.DeliveryTimes {
			rows = append(rows, append(slices.Clone(base), formatOptionalString(dt.RecipientDomain),
				formatInt64(dt.Count), formatOptionalCSVNumber(dt.DeliveryTime)))
		}
	}
	return h.writeRows([]string{
		"from_timestamp", "to_timestamp", "delivered_count", "avg_delivery_time",
		"recipient_domain", "domain_delivered_count", "domain_delivery_time",
	}, rows...)
}

// writeStatsTitle writes the title of a statistics report, when it has one.
func writeStatsTitle(w io.Writer, config StatsConfig) error {
	if config.Title == "" {
		return nil
	}
	_, err := fmt.Fprintf(w, "%s\n\n", config.Title)
	return err
}

// formatTimePeriod formats the time period of a statistics row.
func formatTimePeriod(from, to time.Time) string {
	return fmt.Sprintf("%s to %s", formatTime(from), formatTime(to))
}

// writeTimePeriod writes the time period line that starts a period of plain
// statistics.
func writeTimePeriod(w io.Writer, from, to time.Time) error {
	_, err := fmt.Fprintf(w, "Time Period: %s\n", formatTimePeriod(from, to))
	return err
}

// deliverabilityLabels name the fields of a period of deliverability
// statistics after its time period: the table's columns and plain output's
// lines.
var deliverabilityLabels = []string{
	"Reception", "Delivered", "Deferred", "Bounced", "Failed", "Suppressed", "Opened", "Clicked",
	"Delivery Rate", "Open Rate",
}

// deliverabilityValues returns the table and plain value of each of
// deliverabilityLabels.
func deliverabilityValues(stat *responses.DeliverabilityStatistics) []string {
	deliveryRate, openRate := deliverabilityRates(stat)
	return []string{
		formatInt(stat.ReceptionCount),
		formatInt(stat.DeliveredCount),
		formatInt(stat.DeferredCount),
		formatInt(stat.BouncedCount),
		formatInt(stat.FailedCount),
		formatInt(stat.SuppressedCount),
		formatInt(stat.OpenedCount),
		formatInt(stat.ClickedCount),
		formatRate(deliveryRate, ""),
		formatRate(openRate, ""),
	}
}

// deliverabilityRates returns the percentage of received messages that were
// delivered and of delivered messages that were opened, each nil when
// nothing was received or delivered.
func deliverabilityRates(stat *responses.DeliverabilityStatistics) (deliveryRate, openRate *float64) {
	if stat.ReceptionCount > 0 {
		rate := float64(stat.DeliveredCount) / float64(stat.ReceptionCount) * 100
		deliveryRate = &rate
	}
	if stat.DeliveredCount > 0 {
		rate := float64(stat.OpenedCount) / float64(stat.DeliveredCount) * 100
		openRate = &rate
	}
	return deliveryRate, openRate
}

// totalBounces returns the number of bounces of all classifications.
func totalBounces(bounces []responses.Bounce) int {
	total := 0
	for _, bounce := range bounces {
		total += bounce.Count
	}
	return total
}

// bouncePercentage returns count as a percentage of total, or 0 when total
// is 0.
func bouncePercentage(count, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(count) / float64(total) * 100
}

// formatBouncePercentage formats count as a percentage of total.
func formatBouncePercentage(count, total int) string {
	return formatPercent(bouncePercentage(count, total))
}

// formatDomainDeliveryTime formats the delivery time of a recipient domain
// like formatDeliveryTime, or "" when the API sent none.
func formatDomainDeliveryTime(dt responses.DeliveryTime) string {
	if dt.DeliveryTime == nil {
		return ""
	}
	return formatDeliveryTime(*dt.DeliveryTime)
}

// formatDeliveryTime formats a delivery time in seconds, minutes or hours,
// whichever reads best, with a unit letter, such as 1.50s or 2.25m.
func formatDeliveryTime(seconds float64) string {
	if seconds < 60 {
		return fmt.Sprintf("%.2fs", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%.2fm", minutes)
	}
	return fmt.Sprintf("%.2fh", minutes/60)
}
