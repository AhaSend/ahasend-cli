package printer

import (
	"fmt"
)

// plainHandler handles plain text output formatting with complete type safety.
// It writes each field as a "Label: value" line and leaves out a field whose
// value is missing.
type plainHandler struct {
	handlerBase
}

// GetFormat returns the format name
func (h *plainHandler) GetFormat() string {
	return "plain"
}

// writeItemSeparator writes the blank line between two list items.
func (h *plainHandler) writeItemSeparator(index int) error {
	if index == 0 {
		return nil
	}
	_, err := fmt.Fprintln(h.writer)
	return err
}

func (h *plainHandler) HandleEmpty(message string) error {
	_, err := fmt.Fprintln(h.writer, message)
	return err
}
