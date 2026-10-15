package printer

import (
	"fmt"

	"github.com/AhaSend/ahasend-go/models/common"
)

// listPage is one page of a list: its items and the cursors of the pages
// around it. A nil *listPage stands for no response.
type listPage[T any] struct {
	items      []T
	pagination common.PaginationInfo
}

// pageOf returns the page of a paginated response, or nil for none.
func pageOf[T any](response *common.PaginatedResponse[T]) *listPage[T] {
	if response == nil {
		return nil
	}
	return &listPage[T]{items: response.Data, pagination: response.Pagination}
}

// writeTableList writes a page as table output: the success message, a table
// with a column for each of columns, named by headers, and a row of each
// item's fields, then the list footer, which counts the items as noun. An
// empty page, or none, writes the empty message and the cursor lines.
func writeTableList[T any](h *tableHandler, config ListConfig, page *listPage[T], noun string,
	columns []string, headers map[string]string, fields func(*T) map[string]string) error {
	if page == nil || len(page.items) == 0 {
		return writeEmptyPage(h.writer, config.EmptyMessage, page.cursors())
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.writeFieldTable(columns, headers, fieldMapsOf(page.items, fields)...)
	return writeListFooter(h.writer, len(page.items), noun, page.pagination)
}

// writePlainList writes a page as plain output: the success message, each
// item written by writeItem, with a blank line between items, then the list
// footer, which counts the items as noun. An empty page, or none, writes the
// empty message and the cursor lines.
func writePlainList[T any](h *plainHandler, config ListConfig, page *listPage[T], noun string, writeItem func(*T) error) error {
	if page == nil || len(page.items) == 0 {
		return writeEmptyPage(h.writer, config.EmptyMessage, page.cursors())
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	for i := range page.items {
		if err := h.writeItemSeparator(i); err != nil {
			return err
		}
		if err := writeItem(&page.items[i]); err != nil {
			return err
		}
	}
	return writeListFooter(h.writer, len(page.items), noun, page.pagination)
}

// writeCSVList writes a page as CSV output: the columns as the header row,
// then a row of each item's fields. No page writes nothing.
func writeCSVList[T any](h *csvHandler, page *listPage[T], columns []string, fields func(*T) map[string]string) error {
	if page == nil {
		return nil
	}
	return h.writeFieldMaps(columns, fieldMapsOf(page.items, fields)...)
}

// writeFieldLines writes the label line of each of fields of one item, named
// by headers, leaving out the fields without a value.
func (h *plainHandler) writeFieldLines(fields []string, headers, values map[string]string) error {
	return writeLabelLines(h.writer, "", fieldRows(fields, headers, values))
}

// cursors returns the pagination of the page, or none when there is no page.
func (p *listPage[T]) cursors() common.PaginationInfo {
	if p == nil {
		return common.PaginationInfo{}
	}
	return p.pagination
}

// fieldMapsOf returns the field map of each item.
func fieldMapsOf[T any](items []T, fields func(*T) map[string]string) []map[string]string {
	rows := make([]map[string]string, len(items))
	for i := range items {
		rows[i] = fields(&items[i])
	}
	return rows
}
