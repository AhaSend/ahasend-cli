package printer

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/apiresponse"
)

func TestJSONHandleAPIResults_PrintsEachAPIBodyInOrder(t *testing.T) {
	first := &common.SuccessResponse{}
	apiresponse.Remember(first, []byte(`{"message":"Message canceled","zeta":1.50}`))
	third := &common.SuccessResponse{}
	apiresponse.Remember(third, []byte(`{"message":"Message canceled","big":12345678901234567890}`))
	failure := &api.APIError{StatusCode: 404, Message: "not found",
		Raw: []byte(`{"message":"Message not found","alpha":true}`)}

	var output bytes.Buffer
	err := testHandler(t, "json", &output, os.Stderr).HandleAPIResults([]APIResult{
		{Item: "a", Response: first},
		{Item: "b", Err: fmt.Errorf("cancel b: %w", failure)},
		{Item: "c", Response: third},
	}, ResultConfig{})

	require.NoError(t, err)
	assert.Equal(t, indentedBody(t, `[{"message":"Message canceled","zeta":1.50},`+
		`{"message":"Message not found","alpha":true},`+
		`{"message":"Message canceled","big":12345678901234567890}]`), output.String())
}

func TestJSONHandleAPIResults_EmptyAPIBodyIsNull(t *testing.T) {
	empty := &common.SuccessResponse{}
	apiresponse.Remember(empty, []byte{})

	var output bytes.Buffer
	err := testHandler(t, "json", &output, os.Stderr).HandleAPIResults([]APIResult{
		{Item: "a", Response: empty},
	}, ResultConfig{})

	require.NoError(t, err)
	assert.Equal(t, indentedBody(t, `[null]`), output.String())
}

func TestJSONHandleAPIResults_ErrorWithoutJSONBodyPrintsErrorObject(t *testing.T) {
	var output bytes.Buffer
	err := testHandler(t, "json", &output, os.Stderr).HandleAPIResults([]APIResult{
		{Item: "a", Err: errors.New("connection refused")},
	}, ResultConfig{})

	require.NoError(t, err)
	assert.Equal(t, indentedBody(t, `[{"error":true,"message":"connection refused"}]`), output.String())
}

func TestTableHandleAPIResults_ListsEveryCallWithItsOutcome(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleAPIResults([]APIResult{
		{Item: "a", Response: &common.SuccessResponse{}},
		{Item: "b", Err: errors.New("not found")},
	}, ResultConfig{SuccessMessage: "Message cancellation completed"}))

	assert.Equal(t, "Message cancellation completed\nSucceeded: 1  Failed: 1\n\n"+
		"┌──────┬─────────┬───────────┐\n"+
		"│ ITEM │ OUTCOME │   ERROR   │\n"+
		"├──────┼─────────┼───────────┤\n"+
		"│ a    │ success │ -         │\n"+
		"│ b    │ failed  │ not found │\n"+
		"└──────┴─────────┴───────────┘\n", output.String())
}

func TestPlainHandleAPIResults_ListsEveryCallWithItsOutcome(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleAPIResults([]APIResult{
		{Item: "a", Response: &common.SuccessResponse{}},
		{Item: "b", Err: errors.New("not found")},
	}, ResultConfig{SuccessMessage: "Message cancellation completed"}))

	assert.Equal(t, "Message cancellation completed\nSucceeded: 1\nFailed: 1\n"+
		"\nResult 1:\n  Item: a\n  Outcome: success\n"+
		"\nResult 2:\n  Item: b\n  Outcome: failed\n  Error: not found\n", output.String())
}

func TestCSVHandleAPIResults_PrintsOneRowPerCall(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleAPIResults([]APIResult{
		{Item: "a", Response: &common.SuccessResponse{}},
		{Item: "b", Err: errors.New("not found")},
	}, ResultConfig{}))

	assert.Equal(t, "item,outcome,error\na,success,\nb,failed,not found\n", output.String())
}

func TestHandleAPIResults_ShowsTheFailuresOfASuccessfulCallAsPartial(t *testing.T) {
	results := []APIResult{
		{Item: "a", Response: &common.SuccessResponse{}, Failures: []string{"x@example.com: suppressed", "y@example.com: invalid"}},
	}
	tests := map[string]string{
		"table": "│ a    │ partial │ x@example.com: suppressed; y@example.com: invalid │",
		"plain": "Succeeded: 0\nFailed: 1\n\nResult 1:\n  Item: a\n  Outcome: partial\n  Error: x@example.com: suppressed; y@example.com: invalid\n",
		"csv":   "a,partial,x@example.com: suppressed; y@example.com: invalid\n",
	}
	for format, want := range tests {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleAPIResults(results, ResultConfig{}))
			assert.Contains(t, output.String(), want)
		})
	}
}

func TestHandleAPIResults_CountsUnknownOutcomesApart(t *testing.T) {
	results := []APIResult{
		{Item: "a", Response: &common.SuccessResponse{}},
		{Item: "b", Err: errors.New("bad gateway"), Unknown: true},
		{Item: "c", Err: errors.New("bad request")},
	}
	tests := map[string]string{
		"table": "Sent\nSucceeded: 1  Failed: 1  Unknown: 1\n\n",
		"plain": "Sent\nSucceeded: 1\nFailed: 1\nUnknown: 1\n\n",
	}
	for format, want := range tests {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleAPIResults(results, ResultConfig{SuccessMessage: "Sent"}))
			assert.True(t, strings.HasPrefix(output.String(), want), output.String())
		})
	}
}
