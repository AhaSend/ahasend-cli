package printer

import (
	"fmt"

	"github.com/AhaSend/ahasend-go/models/responses"
)

// templateVersionFields are the fields of each version of a list, which the
// table, plain and CSV output show in this order.
var templateVersionFields = []string{"version", "id", "published_at", "published_by"}

// templateVersionTableColumns are the fields a table of versions shows: all
// of them, as they fit.
var templateVersionTableColumns = templateVersionFields

// templateVersionHeaders names the table column or row of each version field.
var templateVersionHeaders = map[string]string{
	"version":      "Version",
	"id":           "ID",
	"published_at": "Published At",
	"published_by": "Published By",
}

var templateVersionDetailCSVHeaders = []string{
	"version", "id", "published_at", "published_by", "subject",
	"preheader", "from", "reply_to", "variables", "mjml", "html", "text", "text_is_custom",
}

func (h *jsonHandler) HandleTemplateVersionList(response *responses.TemplateVersionsResponse, config ListConfig) error {
	return h.printAPIResponse(response)
}

func (h *jsonHandler) HandleSingleTemplateVersion(version *responses.TemplateVersionDetail, config SingleConfig) error {
	return h.printAPIResponse(version)
}

func (h *tableHandler) HandleTemplateVersionList(response *responses.TemplateVersionsResponse, config ListConfig) error {
	return writeTableList(h, config, templateVersionsPage(response), "versions",
		templateVersionTableColumns, templateVersionHeaders, templateVersionDisplayFields)
}

func (h *tableHandler) HandleSingleTemplateVersion(version *responses.TemplateVersionDetail, config SingleConfig) error {
	if version == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderFieldTable(append(templateVersionDetailRows(version), templateBodyRows(version.Variables, version.Content)...))
	return nil
}

func (h *plainHandler) HandleTemplateVersionList(response *responses.TemplateVersionsResponse, config ListConfig) error {
	return writePlainList(h, config, templateVersionsPage(response), "versions", func(version *responses.TemplateVersion) error {
		return h.writeFieldLines(templateVersionFields, templateVersionHeaders, templateVersionDisplayFields(version))
	})
}

func (h *plainHandler) HandleSingleTemplateVersion(version *responses.TemplateVersionDetail, config SingleConfig) error {
	if version == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	if err := writeLabelLines(h.writer, "", templateVersionDetailRows(version)); err != nil {
		return err
	}
	return writeTemplateBody(h.writer, version.Variables, version.Content)
}

func (h *csvHandler) HandleTemplateVersionList(response *responses.TemplateVersionsResponse, config ListConfig) error {
	return writeCSVList(h, templateVersionsPage(response), templateVersionFields, templateVersionCSVFields)
}

func (h *csvHandler) HandleSingleTemplateVersion(version *responses.TemplateVersionDetail, config SingleConfig) error {
	if version == nil {
		return nil
	}
	row := []string{
		formatInt(version.Version), formatUUID(version.ID),
		formatCSVTime(version.PublishedAt), formatTemplatePublisher(version.PublishedBy),
		version.Subject, version.Preheader, formatSenderAddress(version.From),
		formatSenderAddress(version.ReplyTo), formatStringSlice(formatTemplateVariables(version.Variables)),
	}
	return h.writeRows(templateVersionDetailCSVHeaders, append(row, templateContentCSVCells(version.Content)...))
}

// templateVersionsPage returns the versions of a template as one page, or nil
// for no response. The API returns every version on one page.
func templateVersionsPage(response *responses.TemplateVersionsResponse) *listPage[responses.TemplateVersion] {
	if response == nil {
		return nil
	}
	return &listPage[responses.TemplateVersion]{items: response.Data}
}

// templateVersionCSVFields maps each field of a version of a list to its CSV
// value.
func templateVersionCSVFields(version *responses.TemplateVersion) map[string]string {
	values := templateVersionDisplayFields(version)
	values["published_at"] = formatCSVTime(version.PublishedAt)
	return values
}

// templateVersionDisplayFields maps each field of a version of a list to its
// table and plain value.
func templateVersionDisplayFields(version *responses.TemplateVersion) map[string]string {
	return map[string]string{
		"version":      formatInt(version.Version),
		"id":           formatUUID(version.ID),
		"published_at": formatTime(version.PublishedAt),
		"published_by": formatTemplatePublisher(version.PublishedBy),
	}
}

func templateVersionDetailRows(version *responses.TemplateVersionDetail) [][]string {
	return [][]string{
		{"Version", formatInt(version.Version)},
		{"ID", formatUUID(version.ID)},
		{"Published At", formatTime(version.PublishedAt)},
		{"Published By", formatTemplatePublisher(version.PublishedBy)},
		{"Subject", version.Subject},
		{"Preheader", version.Preheader},
		{"From", formatSenderAddress(version.From)},
		{"Reply To", formatSenderAddress(version.ReplyTo)},
	}
}

// formatTemplatePublisher formats who published a version as "type:id", or
// "" when the publisher is not known.
func formatTemplatePublisher(publisher *responses.TemplatePublisher) string {
	if publisher == nil {
		return ""
	}
	return publisher.Type + ":" + publisher.ID.String()
}
