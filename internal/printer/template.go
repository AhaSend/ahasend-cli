package printer

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
)

// templateFields are the fields of each view of a template before its
// content, which the table, plain and CSV output show in this order, except
// a table of templates, which shows templateTableColumns. The views end with
// the content: templateContentColumns in CSV output.
var templateFields = []string{
	"name", "id", "editor", "subject", "preheader", "from", "reply_to",
	"has_draft", "variables", "created_at", "updated_at",
}

// templateListFields are the fields of each template of a list, which plain
// and CSV output show.
var templateListFields = slices.Concat(templateFields, templateContentColumns)

// templateTableColumns are the fields a table of templates shows.
var templateTableColumns = []string{"name", "id", "editor", "subject", "has_draft", "updated_at"}

// templateHeaders names the table column or row of each template field.
var templateHeaders = map[string]string{
	"name":       "Name",
	"id":         "ID",
	"editor":     "Editor",
	"subject":    "Subject",
	"preheader":  "Preheader",
	"from":       "From",
	"reply_to":   "Reply To",
	"has_draft":  "Has Draft",
	"variables":  "Variables",
	"created_at": "Created",
	"updated_at": "Updated",
}

// templateContentColumns are the CSV columns of the content of a template, a
// campaign template, a campaign, a draft or a version.
var templateContentColumns = []string{"mjml", "html", "text", "text_is_custom"}

func (h *jsonHandler) HandleTemplateList(response *responses.PaginatedTemplatesResponse, config ListConfig) error {
	return h.printAPIResponse(response)
}

func (h *jsonHandler) HandleSingleTemplate(template *responses.Template, config SingleConfig) error {
	return h.printAPIResponse(template)
}

func (h *tableHandler) HandleTemplateList(response *responses.PaginatedTemplatesResponse, config ListConfig) error {
	return writeTableList(h, config, templatesPage(response), "templates", templateTableColumns, templateHeaders, templateDisplayFields)
}

func (h *tableHandler) HandleSingleTemplate(template *responses.Template, config SingleConfig) error {
	if template == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderFieldTable(append(templateDetailRows(template), templateContentRows(template.Content)...))
	return nil
}

func (h *plainHandler) HandleTemplateList(response *responses.PaginatedTemplatesResponse, config ListConfig) error {
	return writePlainList(h, config, templatesPage(response), "templates", h.renderTemplateDetails)
}

func (h *plainHandler) HandleSingleTemplate(template *responses.Template, config SingleConfig) error {
	if template == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	return h.renderTemplateDetails(template)
}

func (h *plainHandler) renderTemplateDetails(template *responses.Template) error {
	if err := writeLabelLines(h.writer, "", templateDetailRows(template)); err != nil {
		return err
	}
	return writeTemplateContent(h.writer, template.Content)
}

func (h *csvHandler) HandleTemplateList(response *responses.PaginatedTemplatesResponse, config ListConfig) error {
	return writeCSVList(h, templatesPage(response), templateListFields, templateCSVFields)
}

func (h *csvHandler) HandleSingleTemplate(template *responses.Template, config SingleConfig) error {
	if template == nil {
		return nil
	}
	return h.writeFieldMaps(templateListFields, templateCSVFields(template))
}

// templateCSVFields maps each template field to its CSV value.
func templateCSVFields(template *responses.Template) map[string]string {
	values := templateDisplayFields(template)
	maps.Copy(values, map[string]string{
		"has_draft":  formatBool(template.HasDraft),
		"created_at": formatCSVTime(template.CreatedAt),
		"updated_at": formatCSVTime(template.UpdatedAt),
	})
	maps.Copy(values, templateContentCSVFields(template.Content))
	return values
}

// templateDisplayFields maps each template field before the content to its
// table and plain value.
func templateDisplayFields(template *responses.Template) map[string]string {
	return map[string]string{
		"name":       template.Name,
		"id":         formatUUID(template.ID),
		"editor":     template.Editor,
		"subject":    template.Subject,
		"preheader":  template.Preheader,
		"from":       formatSenderAddress(template.From),
		"reply_to":   formatSenderAddress(template.ReplyTo),
		"has_draft":  formatBooleanStatus(template.HasDraft),
		"variables":  formatStringSlice(formatTemplateVariables(template.Variables)),
		"created_at": formatTime(template.CreatedAt),
		"updated_at": formatTime(template.UpdatedAt),
	}
}

// templateDetailRows returns the rows of a template's detail view before its
// content.
func templateDetailRows(template *responses.Template) [][]string {
	return fieldRows(templateFields, templateHeaders, templateDisplayFields(template))
}

// templatesPage returns the page of a PaginatedTemplatesResponse, or nil for none.
func templatesPage(response *responses.PaginatedTemplatesResponse) *listPage[responses.Template] {
	if response == nil {
		return nil
	}
	return &listPage[responses.Template]{items: response.Data, pagination: response.Pagination}
}

// formatSenderAddress formats an address as "Name <email>", or the bare email
// when it has no name, or "" when there is no address.
func formatSenderAddress(address *common.SenderAddress) string {
	if address == nil {
		return ""
	}
	if address.Name == nil || *address.Name == "" {
		return address.Email
	}
	return fmt.Sprintf("%s <%s>", *address.Name, address.Email)
}

// formatTemplateVariables formats each variable as its name followed by
// whether a send must supply a value for it.
func formatTemplateVariables(variables []responses.TemplateVariable) []string {
	items := make([]string, len(variables))
	for i, variable := range variables {
		if variable.Required {
			items[i] = variable.Name + " (required)"
		} else {
			items[i] = variable.Name + " (optional)"
		}
	}
	return items
}

// templateContentRows returns a row for each content field the API returned,
// or none when the response holds no content.
func templateContentRows(content *responses.TemplateContent) [][]string {
	if content == nil {
		return nil
	}
	var rows [][]string
	for _, field := range templateContentFields(content) {
		rows = append(rows, field[:])
	}
	return append(rows, []string{"Text Is Custom", formatBooleanStatus(content.TextIsCustom)})
}

// templateVariablesRow returns the variables row of a template, a draft or a
// version.
func templateVariablesRow(variables []responses.TemplateVariable) []string {
	return []string{"Variables", formatStringSlice(formatTemplateVariables(variables))}
}

// templateBodyRows returns the variables row followed by the content rows of
// a template, a draft or a version.
func templateBodyRows(variables []responses.TemplateVariable, content *responses.TemplateContent) [][]string {
	return append([][]string{templateVariablesRow(variables)}, templateContentRows(content)...)
}

// writeTemplateBody writes the variables and then the content of a template,
// a draft or a version as plain lines.
func writeTemplateBody(w io.Writer, variables []responses.TemplateVariable, content *responses.TemplateContent) error {
	if err := writeLabelLines(w, "", [][]string{templateVariablesRow(variables)}); err != nil {
		return err
	}
	return writeTemplateContent(w, content)
}

// writeTemplateContent writes each content field the API returned as a
// "label:" line followed by its indented lines, or nothing when the response
// holds no content.
func writeTemplateContent(w io.Writer, content *responses.TemplateContent) error {
	if content == nil {
		return nil
	}
	for _, field := range templateContentFields(content) {
		if err := writeIndentedList(w, field[0], strings.Split(field[1], "\n")); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "Text Is Custom: %s\n", formatBooleanStatus(content.TextIsCustom))
	return err
}

// templateContentFields returns the label and value of each non-empty content
// field.
func templateContentFields(content *responses.TemplateContent) [][2]string {
	var fields [][2]string
	for _, field := range [][2]string{{"MJML", content.MJML}, {"HTML", content.HTML}, {"Text", content.Text}} {
		if field[1] != "" {
			fields = append(fields, field)
		}
	}
	return fields
}

// templateContentCSVFields maps each of templateContentColumns to its CSV
// value, empty when the response holds no content.
func templateContentCSVFields(content *responses.TemplateContent) map[string]string {
	cells := templateContentCSVCells(content)
	values := make(map[string]string, len(cells))
	for i, column := range templateContentColumns {
		values[column] = cells[i]
	}
	return values
}

// templateContentCSVCells returns the mjml, html, text and text_is_custom
// cells, empty when the response holds no content.
func templateContentCSVCells(content *responses.TemplateContent) []string {
	if content == nil {
		return []string{"", "", "", ""}
	}
	return []string{content.MJML, content.HTML, content.Text, formatBool(content.TextIsCustom)}
}
