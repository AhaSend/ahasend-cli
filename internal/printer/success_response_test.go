package printer

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/apiresponse"
)

func renderSuccessResponse(t *testing.T, format string, response *common.SuccessResponse) string {
	t.Helper()
	var buf bytes.Buffer
	handler := testHandler(t, format, &buf, os.Stderr)
	require.NoError(t, handler.HandleSuccessResponse(response, ResultConfig{SuccessMessage: "Contact deleted"}))
	return buf.String()
}

func TestHandleSuccessResponse_JSONIsTheResponse(t *testing.T) {
	out := renderSuccessResponse(t, "json", &common.SuccessResponse{Message: "Deleted"})

	var got common.SuccessResponse
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, common.SuccessResponse{Message: "Deleted"}, got)
	assert.Equal(t, "{\n  \"message\": \"Deleted\"\n}\n", out)
}

func TestHandleSuccessResponse_JSONWithoutMessage(t *testing.T) {
	out := renderSuccessResponse(t, "json", &common.SuccessResponse{})
	assert.Equal(t, "{\n  \"message\": \"\"\n}\n", out)
}

func TestHandleSuccessResponse_TablePrintsAPIMessage(t *testing.T) {
	assert.Equal(t, "Deleted\n", renderSuccessResponse(t, "table", &common.SuccessResponse{Message: "Deleted"}))
}

func TestHandleSuccessResponse_TableFallsBackToSuccessMessage(t *testing.T) {
	assert.Equal(t, "Contact deleted\n", renderSuccessResponse(t, "table", &common.SuccessResponse{}))
}

func TestHandleSuccessResponse_PlainPrintsAPIMessage(t *testing.T) {
	assert.Equal(t, "Deleted\n", renderSuccessResponse(t, "plain", &common.SuccessResponse{Message: "Deleted"}))
}

func TestHandleSuccessResponse_PlainFallsBackToSuccessMessage(t *testing.T) {
	assert.Equal(t, "Contact deleted\n", renderSuccessResponse(t, "plain", &common.SuccessResponse{}))
}

func TestHandleSuccessResponse_CSVPrintsAPIMessage(t *testing.T) {
	assert.Equal(t, "message\nDeleted\n", renderSuccessResponse(t, "csv", &common.SuccessResponse{Message: "Deleted"}))
}

func TestHandleSuccessResponse_CSVFallsBackToSuccessMessage(t *testing.T) {
	assert.Equal(t, "message\nContact deleted\n", renderSuccessResponse(t, "csv", &common.SuccessResponse{}))
}

func TestHandleSuccessResponse_JSONPrintsNothingWhenTheAPISentNoBody(t *testing.T) {
	empty := &common.SuccessResponse{}
	apiresponse.Remember(empty, []byte{})

	assert.Empty(t, renderSuccessResponse(t, "json", empty))
	assert.Empty(t, renderSuccessResponse(t, "json", nil))
}

func TestHandleSuccessResponse_PlainPrintsSuccessMessageWhenTheAPISentNoBody(t *testing.T) {
	empty := &common.SuccessResponse{}
	apiresponse.Remember(empty, []byte{})

	assert.Equal(t, "Contact deleted\n", renderSuccessResponse(t, "plain", empty))
}
