package printer

import (
	"fmt"
	"maps"
	"slices"

	"github.com/AhaSend/ahasend-go/models/responses"
)

// The fields of the views of a campaign, which the table, plain and CSV
// output of each view show in this order, except a table of campaigns, which
// shows campaignTableColumns.
var (
	// campaignListFields are the fields of each campaign of a list.
	campaignListFields = []string{
		"name", "id", "status", "status_reason", "subject", "preheader",
		"from", "reply_to", "send_at", "send_resumes_at", "finished_at",
		"created_at", "updated_at",
	}
	// campaignTableColumns are the fields a table of campaigns shows.
	campaignTableColumns = []string{"name", "id", "status", "subject", "send_at", "updated_at"}
	// campaignDetailFields are the fields of a campaign's detail view before
	// its content. The view ends with the content: templateContentColumns in
	// CSV output.
	campaignDetailFields = slices.Concat(campaignListFields, []string{"list_ids", "editor", "ready", "issues"})
)

// campaignHeaders names the table column or row of each campaign field.
var campaignHeaders = map[string]string{
	"name":            "Name",
	"id":              "ID",
	"status":          "Status",
	"status_reason":   "Status Reason",
	"subject":         "Subject",
	"preheader":       "Preheader",
	"from":            "From",
	"reply_to":        "Reply To",
	"send_at":         "Send At",
	"send_resumes_at": "Send Resumes At",
	"finished_at":     "Finished At",
	"created_at":      "Created",
	"updated_at":      "Updated",
	"list_ids":        "Lists",
	"editor":          "Editor",
	"ready":           "Ready",
	"issues":          "Issues",
}

var campaignTestCSVHeaders = []string{"email", "outcome", "reason"}

// campaignTestUnknownNotice is shown when the mail server took a test but its
// answer could not be read.
const campaignTestUnknownNotice = "Result unknown: the mail server took the test but its answer could not be read. " +
	"Check the inboxes before sending another test."

func (h *jsonHandler) HandleCampaignList(response *responses.PaginatedCampaignsResponse, config ListConfig) error {
	return h.printAPIResponse(response)
}

func (h *jsonHandler) HandleSingleCampaign(campaign *responses.Campaign, config SingleConfig) error {
	return h.printAPIResponse(campaign)
}

func (h *jsonHandler) HandleCampaignSend(send *responses.CampaignSend, config SingleConfig) error {
	return h.printAPIResponse(send)
}

func (h *jsonHandler) HandleCampaignTest(test *responses.CampaignTest, config SingleConfig) error {
	return h.printAPIResponse(test)
}

func (h *tableHandler) HandleCampaignList(response *responses.PaginatedCampaignsResponse, config ListConfig) error {
	return writeTableList(h, config, campaignsPage(response), "campaigns", campaignTableColumns, campaignHeaders, campaignListItemDisplayFields)
}

func (h *tableHandler) HandleSingleCampaign(campaign *responses.Campaign, config SingleConfig) error {
	if campaign == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderCampaignDetails(campaign, nil)
	return nil
}

func (h *tableHandler) HandleCampaignSend(send *responses.CampaignSend, config SingleConfig) error {
	if send == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderCampaignDetails(&send.Campaign, send.Warnings)
	return nil
}

// renderCampaignDetails renders a campaign as field rows, followed by the
// warnings of a send when there are any.
func (h *tableHandler) renderCampaignDetails(campaign *responses.Campaign, warnings []string) {
	rows := append(campaignDetailRows(campaign), templateContentRows(campaign.Content)...)
	if len(warnings) > 0 {
		rows = append(rows, []string{"Warnings", formatStringSlice(warnings)})
	}
	h.renderFieldTable(rows)
}

func (h *tableHandler) HandleCampaignTest(test *responses.CampaignTest, config SingleConfig) error {
	if test == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if test.ResultUnknown {
		_, err := fmt.Fprintln(h.writer, campaignTestUnknownNotice)
		return err
	}

	if _, err := fmt.Fprintf(h.writer, "%s\nSent: %d  Failed: %d\n", config.SuccessMessage,
		len(test.SentTo), len(test.Failed)); err != nil {
		return err
	}
	rows := campaignTestRows(test)
	if len(rows) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(h.writer); err != nil {
		return err
	}
	h.renderTable([]any{"Email", "Outcome", "Reason"}, rows...)
	return nil
}

func (h *plainHandler) HandleCampaignList(response *responses.PaginatedCampaignsResponse, config ListConfig) error {
	return writePlainList(h, config, campaignsPage(response), "campaigns", func(campaign *responses.CampaignListItem) error {
		return h.writeFieldLines(campaignListFields, campaignHeaders, campaignListItemDisplayFields(campaign))
	})
}

func (h *plainHandler) HandleSingleCampaign(campaign *responses.Campaign, config SingleConfig) error {
	if campaign == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	return h.renderCampaignDetails(campaign)
}

func (h *plainHandler) HandleCampaignSend(send *responses.CampaignSend, config SingleConfig) error {
	if send == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	if err := h.renderCampaignDetails(&send.Campaign); err != nil {
		return err
	}
	return writeLabelLines(h.writer, "", [][]string{{"Warnings", formatStringSlice(send.Warnings)}})
}

func (h *plainHandler) renderCampaignDetails(campaign *responses.Campaign) error {
	if err := writeLabelLines(h.writer, "", campaignDetailRows(campaign)); err != nil {
		return err
	}
	return writeTemplateContent(h.writer, campaign.Content)
}

func (h *plainHandler) HandleCampaignTest(test *responses.CampaignTest, config SingleConfig) error {
	if test == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if test.ResultUnknown {
		_, err := fmt.Fprintln(h.writer, campaignTestUnknownNotice)
		return err
	}

	if _, err := fmt.Fprintf(h.writer, "%s\nSent: %d\nFailed: %d\n", config.SuccessMessage,
		len(test.SentTo), len(test.Failed)); err != nil {
		return err
	}
	failures := make([]string, len(test.Failed))
	for i, failure := range test.Failed {
		failures[i] = failure.Email + ": " + failure.Message
	}
	return writeLabelLines(h.writer, "", [][]string{
		{"Sent To", formatStringSlice(test.SentTo)},
		{"Failures", formatStringSlice(failures)},
	})
}

func (h *csvHandler) HandleCampaignList(response *responses.PaginatedCampaignsResponse, config ListConfig) error {
	return writeCSVList(h, campaignsPage(response), campaignListFields, campaignListItemCSVFields)
}

func (h *csvHandler) HandleSingleCampaign(campaign *responses.Campaign, config SingleConfig) error {
	if campaign == nil {
		return nil
	}
	return h.writeCampaignsCSV([]responses.Campaign{*campaign}, nil)
}

func (h *csvHandler) HandleCampaignSend(send *responses.CampaignSend, config SingleConfig) error {
	if send == nil {
		return nil
	}
	// A send always has the warnings column, whether the API sent an empty
	// list, null or no warnings key.
	warnings := send.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return h.writeCampaignsCSV([]responses.Campaign{send.Campaign}, warnings)
}

// writeCampaignsCSV writes one row per campaign. A send's warnings add a
// warnings column.
func (h *csvHandler) writeCampaignsCSV(campaigns []responses.Campaign, warnings []string) error {
	columns := slices.Concat(campaignDetailFields, templateContentColumns)
	if warnings != nil {
		columns = append(columns, "warnings")
	}

	rows := make([]map[string]string, len(campaigns))
	for i := range campaigns {
		campaign := &campaigns[i]
		rows[i] = campaignListItemCSVFields(&campaign.CampaignListItem)
		maps.Copy(rows[i], map[string]string{
			"list_ids": formatStringSlice(formatCampaignListIDs(campaign)),
			"editor":   formatOptionalString(campaign.Editor),
			"ready":    formatOptionalBool(campaign.Ready),
			"issues":   formatStringSlice(formatCampaignIssues(campaign.Issues)),
			"warnings": formatStringSlice(warnings),
		})
		maps.Copy(rows[i], templateContentCSVFields(campaign.Content))
	}
	return h.writeFieldMaps(columns, rows...)
}

func (h *csvHandler) HandleCampaignTest(test *responses.CampaignTest, config SingleConfig) error {
	if test == nil {
		return nil
	}

	rows := campaignTestRows(test)
	if test.ResultUnknown {
		rows = [][]string{{"", "unknown", ""}}
	}
	return h.writeRows(campaignTestCSVHeaders, rows...)
}

// campaignListItemCSVFields maps each field of a campaign of a list to its
// CSV value.
func campaignListItemCSVFields(campaign *responses.CampaignListItem) map[string]string {
	values := campaignListItemDisplayFields(campaign)
	maps.Copy(values, map[string]string{
		"send_at":         formatOptionalCSVTime(campaign.SendAt),
		"send_resumes_at": formatOptionalCSVTime(campaign.SendResumesAt),
		"finished_at":     formatOptionalCSVTime(campaign.FinishedAt),
		"created_at":      formatCSVTime(campaign.CreatedAt),
		"updated_at":      formatCSVTime(campaign.UpdatedAt),
	})
	return values
}

// campaignListItemDisplayFields maps each field of a campaign of a list to
// its table and plain value.
func campaignListItemDisplayFields(campaign *responses.CampaignListItem) map[string]string {
	return map[string]string{
		"name":            campaign.Name,
		"id":              formatUUID(campaign.ID),
		"status":          campaign.Status,
		"status_reason":   formatOptionalString(campaign.StatusReason),
		"subject":         formatOptionalString(campaign.Subject),
		"preheader":       formatOptionalString(campaign.Preheader),
		"from":            formatSenderAddress(campaign.From),
		"reply_to":        formatOptionalString(campaign.ReplyTo),
		"send_at":         formatOptionalTime(campaign.SendAt),
		"send_resumes_at": formatOptionalTime(campaign.SendResumesAt),
		"finished_at":     formatOptionalTime(campaign.FinishedAt),
		"created_at":      formatTime(campaign.CreatedAt),
		"updated_at":      formatTime(campaign.UpdatedAt),
	}
}

// campaignDetailRows returns the rows of a campaign's detail view before its
// content.
func campaignDetailRows(campaign *responses.Campaign) [][]string {
	values := campaignListItemDisplayFields(&campaign.CampaignListItem)
	maps.Copy(values, map[string]string{
		"list_ids": formatStringSlice(formatCampaignListIDs(campaign)),
		"editor":   formatOptionalString(campaign.Editor),
		"issues":   formatStringSlice(formatCampaignIssues(campaign.Issues)),
	})
	if campaign.Ready != nil {
		values["ready"] = formatBooleanStatus(*campaign.Ready)
	}
	return fieldRows(campaignDetailFields, campaignHeaders, values)
}

// campaignTestRows returns an email, outcome and reason row for each address
// of a test: "sent" for the accepted ones, then "failed" with the API's
// message for the refused ones.
func campaignTestRows(test *responses.CampaignTest) [][]string {
	rows := make([][]string, 0, len(test.SentTo)+len(test.Failed))
	for _, email := range test.SentTo {
		rows = append(rows, []string{email, "sent", ""})
	}
	for _, failure := range test.Failed {
		rows = append(rows, []string{failure.Email, "failed", failure.Message})
	}
	return rows
}

func formatCampaignListIDs(campaign *responses.Campaign) []string {
	ids := make([]string, len(campaign.ListIDs))
	for i, id := range campaign.ListIDs {
		ids[i] = formatUUID(id)
	}
	return ids
}

// formatCampaignIssues formats each issue as "code: message".
func formatCampaignIssues(issues []responses.CampaignIssue) []string {
	items := make([]string, len(issues))
	for i, issue := range issues {
		items[i] = issue.Code + ": " + issue.Message
	}
	return items
}

// campaignsPage returns the page of a PaginatedCampaignsResponse, or nil for none.
func campaignsPage(response *responses.PaginatedCampaignsResponse) *listPage[responses.CampaignListItem] {
	if response == nil {
		return nil
	}
	return &listPage[responses.CampaignListItem]{items: response.Data, pagination: response.Pagination}
}
