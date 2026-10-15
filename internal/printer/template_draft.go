package printer

import (
	"fmt"

	"github.com/AhaSend/ahasend-go/models/responses"
)

var templateDraftCSVHeaders = []string{
	"subject", "template_id", "preheader", "from", "reply_to",
	"updated_at", "variables", "mjml", "html", "text", "text_is_custom",
}

func (h *jsonHandler) HandleTemplateDraft(draft *responses.TemplateDraft, config SingleConfig) error {
	return h.printAPIResponse(draft)
}

func (h *tableHandler) HandleTemplateDraft(draft *responses.TemplateDraft, config SingleConfig) error {
	if draft == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderFieldTable(append(templateDraftDetailRows(draft), templateBodyRows(draft.Variables, draft.Content)...))
	return nil
}

func (h *plainHandler) HandleTemplateDraft(draft *responses.TemplateDraft, config SingleConfig) error {
	if draft == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	if err := writeLabelLines(h.writer, "", templateDraftDetailRows(draft)); err != nil {
		return err
	}
	return writeTemplateBody(h.writer, draft.Variables, draft.Content)
}

func (h *csvHandler) HandleTemplateDraft(draft *responses.TemplateDraft, config SingleConfig) error {
	if draft == nil {
		return nil
	}
	row := []string{
		draft.Subject, formatUUID(draft.TemplateID), draft.Preheader,
		formatSenderAddress(draft.From), formatSenderAddress(draft.ReplyTo),
		formatCSVTime(draft.UpdatedAt), formatStringSlice(formatTemplateVariables(draft.Variables)),
	}
	return h.writeRows(templateDraftCSVHeaders, append(row, templateContentCSVCells(draft.Content)...))
}

func templateDraftDetailRows(draft *responses.TemplateDraft) [][]string {
	return [][]string{
		{"Subject", draft.Subject},
		{"Template ID", formatUUID(draft.TemplateID)},
		{"Preheader", draft.Preheader},
		{"From", formatSenderAddress(draft.From)},
		{"Reply To", formatSenderAddress(draft.ReplyTo)},
		{"Updated", formatTime(draft.UpdatedAt)},
	}
}
