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

// templateDetailFields are the CSV columns of a single template: those of a
// template of a list, then the warnings of a write that carried AhaML.
var templateDetailFields = slices.Concat(templateListFields, []string{"markup_warnings"})

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
// campaign template, a campaign or a version.
var templateContentColumns = []string{"mjml", "markup", "markup_error", "html", "text", "text_is_custom"}

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
	h.renderFieldTable(slices.Concat(templateDetailRows(template), templateContentRows(template.Content),
		markupWarningsRows(template.MarkupWarnings)))
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
	if err := h.renderTemplateDetails(template); err != nil {
		return err
	}
	return writeMarkupWarnings(h.writer, template.MarkupWarnings)
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
	values := templateCSVFields(template)
	warnings, err := formatMarkupWarningsCSV(template.MarkupWarnings)
	if err != nil {
		return err
	}
	values["markup_warnings"] = warnings
	return h.writeFieldMaps(templateDetailFields, values)
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

// formatTemplateVariables formats each variable as its name followed, in
// parentheses, by its kind and whether a send must supply a value for it:
// "first_name (required)", "vip (condition; optional)" or
// "items (list of name/price; required)". The variables are joined with
// ", ", so a variable holds no comma.
func formatTemplateVariables(variables []responses.TemplateVariable) []string {
	items := make([]string, len(variables))
	for i, variable := range variables {
		required := "optional"
		if variable.Required {
			required = "required"
		}
		if kind := formatTemplateVariableKind(variable); kind != "" {
			items[i] = fmt.Sprintf("%s (%s; %s)", variable.Name, kind, required)
		} else {
			items[i] = fmt.Sprintf("%s (%s)", variable.Name, required)
		}
	}
	return items
}

// formatTemplateVariableKind returns the kind of a variable: "" for a value,
// and for a variable with no kind; "list of" its item fields joined with "/"
// for a list, or "list" when it has none; any other kind as the API names
// it.
func formatTemplateVariableKind(variable responses.TemplateVariable) string {
	switch variable.Kind {
	case "", responses.TemplateVariableKindValue:
		return ""
	case responses.TemplateVariableKindList:
		if len(variable.ItemFields) == 0 {
			return responses.TemplateVariableKindList
		}
		return "list of " + strings.Join(variable.ItemFields, "/")
	default:
		return variable.Kind
	}
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

// templateVariablesRow returns the variables row of a template version.
func templateVariablesRow(variables []responses.TemplateVariable) []string {
	return []string{"Variables", formatStringSlice(formatTemplateVariables(variables))}
}

// templateBodyRows returns the variables row followed by the content rows of
// a template version.
func templateBodyRows(variables []responses.TemplateVariable, content *responses.TemplateContent) [][]string {
	return append([][]string{templateVariablesRow(variables)}, templateContentRows(content)...)
}

// writeTemplateBody writes the variables and then the content of a template
// version as plain lines.
func writeTemplateBody(w io.Writer, variables []responses.TemplateVariable, content *responses.TemplateContent) error {
	if err := writeLabelLines(w, "", [][]string{templateVariablesRow(variables)}); err != nil {
		return err
	}
	return writeTemplateContent(w, content)
}

// writeTemplateContent writes each content field the API returned as a
// "label:" line followed by its indented lines, without a blank line for the
// newline that ends the value, or nothing when the response holds no content.
func writeTemplateContent(w io.Writer, content *responses.TemplateContent) error {
	if content == nil {
		return nil
	}
	for _, field := range templateContentFields(content) {
		if err := writeIndentedList(w, field[0], strings.Split(strings.TrimSuffix(field[1], "\n"), "\n")); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "Text Is Custom: %s\n", formatBooleanStatus(content.TextIsCustom))
	return err
}

// templateContentFields returns the label and value of each non-empty content
// field: the MJML, the markup, the markup error, the HTML and the text.
func templateContentFields(content *responses.TemplateContent) [][2]string {
	var fields [][2]string
	for _, field := range [][2]string{
		{"MJML", content.MJML},
		{"Markup", formatOptionalString(content.Markup)},
		{"Markup Error", formatMarkupError(content.MarkupError)},
		{"HTML", content.HTML},
		{"Text", content.Text},
	} {
		if field[1] != "" {
			fields = append(fields, field)
		}
	}
	return fields
}

// formatMarkupError formats why a design has no markup as "code: message" on
// one line, or "" when it has none.
func formatMarkupError(markupError *responses.MarkupError) string {
	if markupError == nil {
		return ""
	}
	return EscapeControl(markupError.Code) + ": " + EscapeControl(markupError.Message)
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

// templateContentCSVCells returns the cells of templateContentColumns, the
// mjml, markup, markup_error, html, text and text_is_custom cells, empty
// when the response holds no content.
func templateContentCSVCells(content *responses.TemplateContent) []string {
	if content == nil {
		return make([]string, len(templateContentColumns))
	}
	return []string{
		content.MJML, formatOptionalString(content.Markup), formatMarkupError(content.MarkupError),
		content.HTML, content.Text, formatBool(content.TextIsCustom),
	}
}
