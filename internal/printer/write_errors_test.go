package printer

import (
	stderrors "errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errWriteFailed = stderrors.New("write failed")

// failOnceWriter fails the one write that would take it past offset bytes,
// and accepts every other write, so a printer that ignores that error goes
// on without one.
type failOnceWriter struct {
	offset, written int
	failed          bool
}

func (w *failOnceWriter) Write(p []byte) (int, error) {
	if !w.failed && w.written+len(p) > w.offset {
		w.failed = true
		return 0, errWriteFailed
	}
	w.written += len(p)
	return len(p), nil
}

// TestPlainPrintersReturnEveryWriteError checks that the plain printers return
// the error of any write that fails, wherever in the output it fails.
func TestPlainPrintersReturnEveryWriteError(t *testing.T) {
	for name, r := range renders() {
		if r.sparse {
			continue
		}
		render := r.render
		t.Run(name, func(t *testing.T) {
			output := renderWith(t, "plain", render)
			require.NotEmpty(t, output)
			for offset := 0; offset < len(output); offset++ {
				err := render(testHandler(t, "plain", &failOnceWriter{offset: offset}, io.Discard))
				if !assert.ErrorIs(t, err, errWriteFailed, "the write at byte %d of %d failed", offset, len(output)) {
					return
				}
			}
		})
	}
}

// TestTablePrintersReturnTheFirstWriteError checks that the table printers
// stop with the error of a failed first write. A failure inside a table is
// not reported, since the table writer does not report it, so output that
// starts with a table is left out.
func TestTablePrintersReturnTheFirstWriteError(t *testing.T) {
	for name, r := range renders() {
		if r.sparse {
			continue
		}
		render := r.render
		t.Run(name, func(t *testing.T) {
			if strings.HasPrefix(renderWith(t, "table", render), "┌") {
				t.Skip("the output starts with a table")
			}
			err := render(testHandler(t, "table", &failOnceWriter{}, io.Discard))
			assert.ErrorIs(t, err, errWriteFailed)
		})
	}
}
