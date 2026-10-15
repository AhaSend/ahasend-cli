package printer

import (
	"io"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/testenv"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	testenv.Main(m)
}

// testHandler returns the response handler for format that prints results
// to writer and errors to errWriter.
func testHandler(t *testing.T, format string, writer, errWriter io.Writer) ResponseHandler {
	t.Helper()
	handler, err := NewResponseHandler(format, writer, errWriter)
	require.NoError(t, err)
	return handler
}
