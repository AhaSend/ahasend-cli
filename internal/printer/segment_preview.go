package printer

import (
	"fmt"
	"io"

	"github.com/AhaSend/ahasend-go/models/responses"
)

// A preview shows its count, then each of its sample contacts with
// segmentPreviewContactFields. Its CSV output has a row per sample contact,
// each with the count, under segmentPreviewCSVColumns.
var (
	segmentPreviewContactFields = []string{"email", "contact_id"}
	segmentPreviewCSVColumns    = []string{"count", "email", "contact_id"}
)

// segmentPreviewHeaders names the table column or plain label of each
// preview field.
var segmentPreviewHeaders = map[string]string{
	"count":      "Count",
	"email":      "Email",
	"contact_id": "Contact ID",
}

// uncountedPreview stands in table and plain output for the count of a
// preview the API could not count within its time limit.
const uncountedPreview = "not counted in time"

func (h *jsonHandler) HandleSegmentPreview(preview *responses.SegmentPreview, config SingleConfig) error {
	return h.printAPIResponse(preview)
}

func (h *tableHandler) HandleSegmentPreview(preview *responses.SegmentPreview, config SingleConfig) error {
	if preview == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if err := writePreviewCount(h.writer, preview); err != nil {
		return err
	}
	if len(preview.Contacts) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(h.writer); err != nil {
		return err
	}
	h.writeFieldTable(segmentPreviewContactFields, segmentPreviewHeaders,
		fieldMapsOf(preview.Contacts, segmentPreviewContactValues)...)
	return nil
}

func (h *plainHandler) HandleSegmentPreview(preview *responses.SegmentPreview, config SingleConfig) error {
	if preview == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if err := writePreviewCount(h.writer, preview); err != nil {
		return err
	}
	for i := range preview.Contacts {
		if _, err := fmt.Fprintln(h.writer); err != nil {
			return err
		}
		if err := h.writeFieldLines(segmentPreviewContactFields, segmentPreviewHeaders,
			segmentPreviewContactValues(&preview.Contacts[i])); err != nil {
			return err
		}
	}
	return nil
}

func (h *csvHandler) HandleSegmentPreview(preview *responses.SegmentPreview, config SingleConfig) error {
	if preview == nil {
		return nil
	}
	count := formatOptionalInt(preview.Count)
	rows := fieldMapsOf(preview.Contacts, segmentPreviewContactValues)
	if len(rows) == 0 {
		rows = []map[string]string{{}}
	}
	for _, row := range rows {
		row["count"] = count
	}
	return h.writeFieldMaps(segmentPreviewCSVColumns, rows...)
}

// writePreviewCount writes the Count line of a preview, which says when the
// API could not count the contacts in time.
func writePreviewCount(w io.Writer, preview *responses.SegmentPreview) error {
	count := uncountedPreview
	if preview.Count != nil {
		count = formatInt(*preview.Count)
	}
	_, err := fmt.Fprintf(w, "%s: %s\n", segmentPreviewHeaders["count"], count)
	return err
}

// segmentPreviewContactValues maps each field of a sample contact to its
// value, the same in every format.
func segmentPreviewContactValues(contact *responses.SegmentPreviewContact) map[string]string {
	return map[string]string{
		"email":      contact.Email,
		"contact_id": formatUUID(contact.ContactID),
	}
}
