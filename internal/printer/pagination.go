package printer

import (
	"fmt"
	"io"
	"strings"

	"github.com/AhaSend/ahasend-go/models/common"
)

// writeCursorLines writes the flag that fetches each page the pagination
// points to, such as "--after <next_cursor>" for the next page and
// "--before <previous_cursor>" for the previous one. Table and plain list
// output end with these lines.
func writeCursorLines(w io.Writer, pagination common.PaginationInfo) error {
	if pagination.NextCursor != nil && *pagination.NextCursor != "" {
		if _, err := fmt.Fprintf(w, "--after %s\n", *pagination.NextCursor); err != nil {
			return err
		}
	}
	if pagination.PreviousCursor != nil && *pagination.PreviousCursor != "" {
		if _, err := fmt.Fprintf(w, "--before %s\n", *pagination.PreviousCursor); err != nil {
			return err
		}
	}
	return nil
}

// writeEmptyPage writes the empty message of a list, followed by the cursor
// lines, since an empty page can still point to another page.
func writeEmptyPage(w io.Writer, message string, pagination common.PaginationInfo) error {
	if _, err := fmt.Fprintln(w, message); err != nil {
		return err
	}
	return writeCursorLines(w, pagination)
}

// writeShowingLine writes how many items a page holds and whether more exist.
// itemName is a plural that ends in "s".
func writeShowingLine(w io.Writer, count int, itemName string, hasMore bool) error {
	more := ""
	if hasMore {
		more = " (more available)"
	}
	_, err := fmt.Fprintf(w, "\nShowing %s%s\n", formatCount(count, itemName), more)
	return err
}

// formatCount writes count followed by plural, a plural that ends in "s",
// which a count of one drops: "1 domain", "2 domains".
func formatCount(count int, plural string) string {
	if count == 1 {
		plural = strings.TrimSuffix(plural, "s")
	}
	return fmt.Sprintf("%d %s", count, plural)
}

// writeListFooter ends table and plain list output: the showing line, then
// the cursor lines.
func writeListFooter(w io.Writer, count int, itemName string, pagination common.PaginationInfo) error {
	if err := writeShowingLine(w, count, itemName, pagination.HasMore); err != nil {
		return err
	}
	return writeCursorLines(w, pagination)
}
