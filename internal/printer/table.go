package printer

import (
	"fmt"
	"strings"

	"github.com/olekukonko/tablewriter"
)

// tableHandler handles table output formatting with complete type safety
type tableHandler struct {
	handlerBase
}

// GetFormat returns the format name
func (h *tableHandler) GetFormat() string {
	return "table"
}

func (h *tableHandler) HandleEmpty(message string) error {
	_, err := fmt.Fprintln(h.writer, message)
	return err
}

// renderTable renders a table with the headers and a row for each of rows.
func (h *tableHandler) renderTable(headers []any, rows ...[]string) {
	table := tablewriter.NewWriter(h.writer)
	table.Header(headers...)
	for _, row := range rows {
		addTableRow(table, row)
	}
	table.Render()
}

// writeFieldTable renders a table with a column for each of columns, named
// by headers, and a row for each field map; it is the table counterpart of
// the CSV writeFieldMaps. A column a field map lacks shows as missing.
func (h *tableHandler) writeFieldTable(columns []string, headers map[string]string, fieldMaps ...map[string]string) {
	rows := make([][]string, len(fieldMaps))
	for i, fields := range fieldMaps {
		rows[i] = fieldValues(columns, fields)
	}
	h.renderTable(fieldHeaders(columns, headers), rows...)
}

// renderFieldTable renders rows as a bordered Field/Value table, the layout
// of a single resource's details.
func (h *tableHandler) renderFieldTable(rows [][]string) {
	h.renderTable([]any{"Field", "Value"}, rows...)
}

// tableTab is what a tab in a table cell becomes: the table measures a tab
// as no width, while a terminal shows it as up to eight columns.
const tableTab = "    "

// addTableRow adds row to table, with "-" for each missing value. Each value
// is made fit to measure before the table measures it, so the columns line
// up: each CRLF line break becomes a newline, each tab four spaces, and then
// EscapeDisplayText escapes the other control characters, a lone carriage
// return included. The cell then holds no control character but the
// newline, at which the table wraps it, so the display writer's escaping
// leaves the table as measured.
func addTableRow(table *tablewriter.Table, row []string) {
	formattedRow := make([]string, len(row))
	for i, cell := range row {
		if cell == "" {
			formattedRow[i] = "-"
			continue
		}
		cell = strings.ReplaceAll(cell, "\r\n", "\n")
		cell = strings.ReplaceAll(cell, "\t", tableTab)
		formattedRow[i] = EscapeDisplayText(cell)
	}
	table.Append(formattedRow)
}
