package printer

import (
	"fmt"
	"maps"
	"slices"

	"github.com/AhaSend/ahasend-go/models/responses"
)

// campaignTemplateFields are the fields of each view of a campaign template
// before its content, which the table, plain and CSV output show in this
// order, except a table of campaign templates, which shows
// campaignTemplateTableColumns. The views end with the content:
// templateContentColumns in CSV output.
var campaignTemplateFields = []string{"name", "id", "editor", "created_at", "updated_at"}

// campaignTemplateListFields are the fields of each campaign template of a
// list, which plain and CSV output show.
var campaignTemplateListFields = slices.Concat(campaignTemplateFields, templateContentColumns)

// campaignTemplateTableColumns are the fields a table of campaign templates
// shows.
var campaignTemplateTableColumns = []string{"name", "id", "editor", "updated_at"}

// campaignTemplateHeaders names the table column or row of each campaign
// template field.
var campaignTemplateHeaders = map[string]string{
	"name":       "Name",
	"id":         "ID",
	"editor":     "Editor",
	"created_at": "Created",
	"updated_at": "Updated",
}

func (h *jsonHandler) HandleCampaignTemplateList(response *responses.PaginatedCampaignTemplatesResponse, config ListConfig) error {
	return h.printAPIResponse(response)
}

func (h *jsonHandler) HandleSingleCampaignTemplate(template *responses.CampaignTemplate, config SingleConfig) error {
	return h.printAPIResponse(template)
}

func (h *tableHandler) HandleCampaignTemplateList(response *responses.PaginatedCampaignTemplatesResponse, config ListConfig) error {
	return writeTableList(h, config, campaignTemplatesPage(response), "campaign templates",
		campaignTemplateTableColumns, campaignTemplateHeaders, campaignTemplateDisplayFields)
}

func (h *tableHandler) HandleSingleCampaignTemplate(template *responses.CampaignTemplate, config SingleConfig) error {
	if template == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderFieldTable(append(campaignTemplateDetailRows(template), templateContentRows(template.Content)...))
	return nil
}

func (h *plainHandler) HandleCampaignTemplateList(response *responses.PaginatedCampaignTemplatesResponse, config ListConfig) error {
	return writePlainList(h, config, campaignTemplatesPage(response), "campaign templates", h.renderCampaignTemplateDetails)
}

func (h *plainHandler) HandleSingleCampaignTemplate(template *responses.CampaignTemplate, config SingleConfig) error {
	if template == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	return h.renderCampaignTemplateDetails(template)
}

func (h *plainHandler) renderCampaignTemplateDetails(template *responses.CampaignTemplate) error {
	if err := writeLabelLines(h.writer, "", campaignTemplateDetailRows(template)); err != nil {
		return err
	}
	return writeTemplateContent(h.writer, template.Content)
}

func (h *csvHandler) HandleCampaignTemplateList(response *responses.PaginatedCampaignTemplatesResponse, config ListConfig) error {
	return writeCSVList(h, campaignTemplatesPage(response), campaignTemplateListFields, campaignTemplateCSVFields)
}

func (h *csvHandler) HandleSingleCampaignTemplate(template *responses.CampaignTemplate, config SingleConfig) error {
	if template == nil {
		return nil
	}
	return h.writeFieldMaps(campaignTemplateListFields, campaignTemplateCSVFields(template))
}

// campaignTemplateCSVFields maps each campaign template field to its CSV
// value.
func campaignTemplateCSVFields(template *responses.CampaignTemplate) map[string]string {
	values := campaignTemplateDisplayFields(template)
	maps.Copy(values, map[string]string{
		"created_at": formatCSVTime(template.CreatedAt),
		"updated_at": formatCSVTime(template.UpdatedAt),
	})
	maps.Copy(values, templateContentCSVFields(template.Content))
	return values
}

// campaignTemplateDisplayFields maps each campaign template field before the
// content to its table and plain value.
func campaignTemplateDisplayFields(template *responses.CampaignTemplate) map[string]string {
	return map[string]string{
		"name":       template.Name,
		"id":         formatUUID(template.ID),
		"editor":     template.Editor,
		"created_at": formatTime(template.CreatedAt),
		"updated_at": formatTime(template.UpdatedAt),
	}
}

// campaignTemplateDetailRows returns the rows of a campaign template's detail
// view before its content.
func campaignTemplateDetailRows(template *responses.CampaignTemplate) [][]string {
	return fieldRows(campaignTemplateFields, campaignTemplateHeaders, campaignTemplateDisplayFields(template))
}

// campaignTemplatesPage returns the page of a PaginatedCampaignTemplatesResponse, or nil for none.
func campaignTemplatesPage(response *responses.PaginatedCampaignTemplatesResponse) *listPage[responses.CampaignTemplate] {
	if response == nil {
		return nil
	}
	return &listPage[responses.CampaignTemplate]{items: response.Data, pagination: response.Pagination}
}
