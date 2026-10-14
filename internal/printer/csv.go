package printer

import (
	"encoding/csv"
	"slices"
	"strings"
)

// csvHandler handles CSV output formatting with complete type safety
type csvHandler struct {
	handlerBase
}

// GetFormat returns the format name
func (h *csvHandler) GetFormat() string {
	return "csv"
}

func (h *csvHandler) HandleEmpty(message string) error {
	// CSV format doesn't output empty messages
	return nil
}

// writeFieldMaps writes the columns as the header row, then one row per field
// map in the order of the columns, as writeRows does. A column a field map
// lacks is empty.
func (h *csvHandler) writeFieldMaps(columns []string, fieldMaps ...map[string]string) error {
	rows := make([][]string, len(fieldMaps))
	for i, fields := range fieldMaps {
		rows[i] = fieldValues(columns, fields)
	}
	return h.writeRows(columns, rows...)
}

// contentColumns are the CSV columns that hold content, a message's source.
// They keep their text exactly, line breaks included, which CSV quoting
// allows.
var contentColumns = []string{"content"}

// writeRows writes the columns as the header row, then each row; every CSV
// output goes through it. Each value is put on one line, except the values
// of contentColumns, which are written as they are.
func (h *csvHandler) writeRows(columns []string, rows ...[]string) error {
	writer := csv.NewWriter(h.writer)
	if err := writer.Write(columns); err != nil {
		return err
	}
	for _, row := range rows {
		cells := make([]string, len(row))
		for i, cell := range row {
			if i < len(columns) && slices.Contains(contentColumns, columns[i]) {
				cells[i] = cell
			} else {
				cells[i] = escapeCSVField(cell)
			}
		}
		if err := writer.Write(cells); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

// escapeCSVField puts a value on one line; the csv.Writer quotes the rest.
func escapeCSVField(field string) string {
	field = strings.ReplaceAll(field, "\r", "")
	field = strings.ReplaceAll(field, "\n", " ")
	return strings.ReplaceAll(field, "\t", " ")
}
