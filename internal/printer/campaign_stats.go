package printer

import (
	"fmt"

	"github.com/AhaSend/ahasend-go/models/responses"
)

var campaignStatsCSVHeaders = []string{
	"campaign_id", "recipients", "delivered", "opens", "clicks",
	"unsubscribes", "complaints", "open_rate", "click_rate", "bounce_rate",
	"deferral_rate", "recount_failed", "report_computed_at", "report_recipients",
	"report_openers", "report_clickers", "report_bounces", "report_deferrals",
	"report_by_status", "report_by_bounce_classification",
}

func (h *jsonHandler) HandleCampaignStats(stats *responses.CampaignStats, config SingleConfig) error {
	return h.printAPIResponse(stats)
}

func (h *tableHandler) HandleCampaignStats(stats *responses.CampaignStats, config SingleConfig) error {
	if stats == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	rows, err := campaignStatsRows(stats, "-")
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderFieldTable(rows)
	return nil
}

func (h *plainHandler) HandleCampaignStats(stats *responses.CampaignStats, config SingleConfig) error {
	if stats == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	rows, err := campaignStatsRows(stats, "")
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	return writeLabelLines(h.writer, "", rows)
}

func (h *csvHandler) HandleCampaignStats(stats *responses.CampaignStats, config SingleConfig) error {
	if stats == nil {
		return nil
	}

	row := []string{
		formatUUID(stats.CampaignID), formatInt(stats.Recipients),
		formatInt(stats.Delivered), formatInt(stats.Opens), formatInt(stats.Clicks),
		formatInt(stats.Unsubscribes), formatInt(stats.Complaints),
		formatOptionalCSVNumber(stats.Rates.OpenRate), formatOptionalCSVNumber(stats.Rates.ClickRate),
		formatOptionalCSVNumber(stats.Rates.BounceRate), formatOptionalCSVNumber(stats.Rates.DeferralRate),
		formatBool(stats.RecountFailed),
	}
	if report := stats.Report; report != nil {
		byStatus, byBounceClassification, err := campaignReportBreakdowns(report)
		if err != nil {
			return err
		}
		row = append(row, formatCSVTime(report.ComputedAt), formatInt(report.Recipients),
			formatInt(report.Openers), formatInt(report.Clickers), formatInt(report.Bounces),
			formatInt(report.Deferrals), byStatus, byBounceClassification)
	} else {
		row = append(row, "", "", "", "", "", "", "", "")
	}

	return h.writeRows(campaignStatsCSVHeaders, row)
}

// campaignStatsRows returns the label and value of each counter and rate,
// followed by the stored report when the campaign has one. A rate the API has
// not computed yet shows as missingRate.
func campaignStatsRows(stats *responses.CampaignStats, missingRate string) ([][]string, error) {
	rows := [][]string{
		{"Campaign ID", formatUUID(stats.CampaignID)},
		{"Recipients", formatInt(stats.Recipients)},
		{"Delivered", formatInt(stats.Delivered)},
		{"Opens", formatInt(stats.Opens)},
		{"Clicks", formatInt(stats.Clicks)},
		{"Unsubscribes", formatInt(stats.Unsubscribes)},
		{"Complaints", formatInt(stats.Complaints)},
		{"Open Rate", formatRate(stats.Rates.OpenRate, missingRate)},
		{"Click Rate", formatRate(stats.Rates.ClickRate, missingRate)},
		{"Bounce Rate", formatRate(stats.Rates.BounceRate, missingRate)},
		{"Deferral Rate", formatRate(stats.Rates.DeferralRate, missingRate)},
		{"Recount Failed", formatBooleanStatus(stats.RecountFailed)},
	}
	report := stats.Report
	if report == nil {
		return rows, nil
	}
	byStatus, byBounceClassification, err := campaignReportBreakdowns(report)
	if err != nil {
		return nil, err
	}
	return append(rows,
		[]string{"Report Computed At", formatTime(report.ComputedAt)},
		[]string{"Report Recipients", formatInt(report.Recipients)},
		[]string{"Report Openers", formatInt(report.Openers)},
		[]string{"Report Clickers", formatInt(report.Clickers)},
		[]string{"Report Bounces", formatInt(report.Bounces)},
		[]string{"Report Deferrals", formatInt(report.Deferrals)},
		[]string{"Report By Status", byStatus},
		[]string{"Report By Bounce Classification", byBounceClassification},
	), nil
}

// campaignReportBreakdowns returns the report's counts by status and by bounce
// classification as compact JSON, since their keys are free-form.
func campaignReportBreakdowns(report *responses.CampaignStatsReport) (byStatus, byBounceClassification string, err error) {
	if byStatus, err = stableJSON(report.ByStatus); err != nil {
		return "", "", err
	}
	if byBounceClassification, err = stableJSON(report.ByBounceClassification); err != nil {
		return "", "", err
	}
	return byStatus, byBounceClassification, nil
}
