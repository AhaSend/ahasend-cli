package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestErrorOutputFormat checks that a failing command exits with the code of
// its error and prints the error on stderr in table output, and as the JSON
// error object on stdout alone in JSON output.
func TestErrorOutputFormat(t *testing.T) {
	const noProfile = "no profiles found. Run 'ahasend auth login' to authenticate"
	tests := []struct {
		name    string
		args    []string
		code    int
		message string
	}{
		{"domains get without a profile", []string{"domains", "get", "example.com"}, 2, noProfile},
		{"domains verify without a profile", []string{"domains", "verify", "example.com"}, 2, noProfile},
		{"messages send without --from",
			[]string{"messages", "send", "--to", "test@example.com", "--subject", "Test", "--text", "Test"},
			4, "--from is required"},
		{"messages send with an invalid --from",
			[]string{"messages", "send", "--from", "invalid-email", "--to", "test@example.com", "--subject", "Test", "--text", "Test"},
			4, `invalid --from value "invalid-email": not an email address`},
		{"messages send without content",
			[]string{"messages", "send", "--from", "sender@example.com", "--to", "test@example.com", "--subject", "Test"},
			4, "one of --text, --html, --text-file or --html-file must be provided; --amp and --amp-file only add an AMP part"},
		{"messages send with a missing recipients file",
			[]string{"messages", "send", "--from", "sender@example.com", "--recipients", "nonexistent-file.json", "--subject", "Test", "--text", "Test"},
			4, "failed to read --recipients: " + testutil.OpenError(t, "nonexistent-file.json")},
		{"messages list with an invalid limit", []string{"messages", "list", "--limit", "500"}, 4, "--limit must be between 0 and 100"},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/table", func(t *testing.T) {
			stdout, stderr, code := runRoot(t, tt.args...)

			assert.Equal(t, tt.code, code)
			assert.Empty(t, stdout)
			assert.Equal(t, "Error: "+tt.message+"\n", stderr)
		})
		t.Run(tt.name+"/json", func(t *testing.T) {
			stdout, stderr, code := runRoot(t, append(tt.args, "--output", "json")...)

			assert.Equal(t, tt.code, code)
			want, err := json.Marshal(map[string]any{"error": true, "message": tt.message})
			require.NoError(t, err)
			assert.JSONEq(t, string(want), stdout)
			assert.Empty(t, stderr)
		})
	}
}

func newHandleErrorTestCommand(t *testing.T, format string) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	handler := testutil.ResponseHandler(t, format, &stdout, &stderr)
	ctx := context.WithValue(context.Background(), printer.ResponseHandlerKey, handler)
	cmd.SetContext(ctx)

	return cmd, &stdout, &stderr
}

// apiErrorExitCodes lists the exit code each API error status maps to.
var apiErrorExitCodes = []struct {
	statusCode int
	exitCode   int
}{
	{400, 4},
	{401, 2},
	{403, 6},
	{404, 5},
	{409, 1},
	{422, 4},
	{429, 8},
	{500, 1},
}

func TestHandleErrorJSONRawAPIErrorPrintsRawBodyAndExitsByStatus(t *testing.T) {
	for _, tt := range apiErrorExitCodes {
		t.Run(strconv.Itoa(tt.statusCode), func(t *testing.T) {
			cmd, stdout, stderr := newHandleErrorTestCommand(t, "json")
			raw := `{"error":"api_error","message":"api request failed"}`

			code := handleError(cmd, &api.APIError{
				StatusCode: tt.statusCode,
				Message:    "api request failed",
				Code:       "api_error",
				Raw:        []byte(raw),
			})

			assert.Equal(t, tt.exitCode, code)
			assert.JSONEq(t, raw, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}

func TestHandleErrorPlainAPIErrorPrintsErrorAndExitsByStatus(t *testing.T) {
	for _, tt := range apiErrorExitCodes {
		t.Run(strconv.Itoa(tt.statusCode), func(t *testing.T) {
			cmd, stdout, stderr := newHandleErrorTestCommand(t, "plain")

			code := handleError(cmd, &api.APIError{
				StatusCode: tt.statusCode,
				Message:    "api request failed",
				Code:       "api_error",
				Raw:        []byte(`{"error":"api_error"}`),
			})

			assert.Equal(t, tt.exitCode, code)
			assert.Empty(t, stdout.String())
			assert.Contains(t, stderr.String(), "Error:")
		})
	}
}

func TestHandleErrorWrappedAPIErrorExitsByStatus(t *testing.T) {
	cmd, _, _ := newHandleErrorTestCommand(t, "plain")

	code := handleError(cmd, clierrors.NewAPIError("failed to get bounce statistics", &api.APIError{StatusCode: 404}))

	assert.Equal(t, 5, code)
}

func TestRunExitsWithNotFoundCodeWhenAPIReturns404(t *testing.T) {
	for _, format := range []string{"json", "plain"} {
		t.Run(format, func(t *testing.T) {
			listID := "123e4567-e89b-12d3-a456-426614174000"
			mockClient := &mocks.MockClient{}
			mockClient.On("GetList", uuid.MustParse(listID)).Return(nil, &api.APIError{
				StatusCode: 404,
				Message:    "list not found",
				Raw:        []byte(`{"message":"list not found"}`),
			})
			testutil.InjectClient(t, mockClient)

			root := NewRootCmdForTesting()
			var output bytes.Buffer
			root.SetOut(&output)
			root.SetErr(&output)

			assert.Equal(t, 5, run(root, []string{"lists", "get", listID, "--output", format}))
			mockClient.AssertExpectations(t)
		})
	}
}

func TestHandleErrorJSONValidationErrorUsesCLIExitCode(t *testing.T) {
	cmd, stdout, stderr := newHandleErrorTestCommand(t, "json")
	err := clierrors.NewValidationError("invalid input", nil)

	code := handleError(cmd, err)

	assert.Equal(t, clierrors.GetExitCode(err), code)
	assert.NotZero(t, code)
	assert.JSONEq(t, `{"error":true,"message":"invalid input"}`, stdout.String())
	assert.Empty(t, stderr.String())
}

func TestHandleErrorJSONPreservesContactErrorEnvelope(t *testing.T) {
	cmd, stdout, stderr := newHandleErrorTestCommand(t, "json")
	raw := `{"object":"error","error":"contact_invalid","result":{"position":3,"email":"bad@example.com","outcome":"failed","reason":"invalid attributes","attributes":{"nested":{"values":[true,null,7]}}}}`

	code := handleError(cmd, &api.APIError{
		StatusCode: 422,
		Message:    "contact is invalid",
		Code:       "validation_failed",
		Raw:        []byte(raw),
	})

	assert.Equal(t, 4, code)
	assert.Empty(t, stderr.String())
	var actual map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &actual))
	assert.Equal(t, "error", actual["object"])
	result := actual["result"].(map[string]any)
	assert.Equal(t, float64(3), result["position"])
	assert.Equal(t, "bad@example.com", result["email"])
	assert.Equal(t, "failed", result["outcome"])
	assert.Equal(t, "invalid attributes", result["reason"])
	assert.Equal(t, []any{true, nil, float64(7)}, result["attributes"].(map[string]any)["nested"].(map[string]any)["values"])
}

func TestHandleErrorItemsFailedJSONPrintsNothingAndExitsOne(t *testing.T) {
	cmd, stdout, stderr := newHandleErrorTestCommand(t, "json")

	code := handleError(cmd, clierrors.NewItemsFailedError(2, 5, "contacts"))

	assert.Equal(t, 1, code)
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
}

func TestHandleErrorItemsFailedPrintsOneStderrLineAndExitsOne(t *testing.T) {
	for _, format := range []string{"table", "plain", "csv"} {
		t.Run(format, func(t *testing.T) {
			cmd, stdout, stderr := newHandleErrorTestCommand(t, format)

			code := handleError(cmd, clierrors.NewItemsFailedError(2, 5, "contacts"))

			assert.Equal(t, 1, code)
			assert.Empty(t, stdout.String())
			assert.Equal(t, "2 of 5 contacts failed\n", stderr.String())
		})
	}
}

// TestAPIErrorsPrintAsTheAPISentThem checks that commands return API errors
// unwrapped, so each prints the API's message alone, as apikeys list does.
func TestAPIErrorsPrintAsTheAPISentThem(t *testing.T) {
	run := func(t *testing.T, method string, args ...string) (string, int) {
		mockClient := &mocks.MockClient{}
		expectAnyArgs(mockClient, method, nil, rawAPIError(400))
		testutil.InjectClient(t, mockClient)
		stdout, stderr, code := runRoot(t, args...)
		assert.Empty(t, stdout)
		return stderr, code
	}
	want, wantCode := run(t, "ListAPIKeys", "apikeys", "list")
	require.Contains(t, want, "Refused")

	tests := []struct {
		method string
		args   []string
	}{
		{"GetBounceStatistics", []string{"stats", "bounces"}},
		{"GetDeliverabilityStatistics", []string{"stats", "deliverability"}},
		{"GetDeliveryTimeStatistics", []string{"stats", "delivery-time"}},
		{"InitiateWebhookStream", []string{"webhooks", "listen"}},
		{"InitiateRouteStream", []string{"routes", "listen", "--recipient", "*@example.com"}},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			stderr, code := run(t, tt.method, tt.args...)

			assert.Equal(t, want, stderr)
			assert.Equal(t, wantCode, code)
		})
	}
}

// fullWriter fails every write as writing to a full disk, such as
// /dev/full, does.
type fullWriter struct{}

func (fullWriter) Write([]byte) (int, error) {
	return 0, &os.PathError{Op: "write", Path: "/dev/stdout", Err: syscall.ENOSPC}
}

// TestAStdoutWriteFailureExitsOne checks that a command whose output cannot
// be written exits 1, not 7, and that in JSON output, whose errors go to
// stdout, the error is written to stderr instead.
func TestAStdoutWriteFailureExitsOne(t *testing.T) {
	for _, format := range []string{"json", "plain"} {
		t.Run(format, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			expectAnyArgs(mockClient, "ListDomains",
				mocks.APIResponse[responses.PaginatedDomainsResponse](t, `{"object":"list","data":[{"domain":"example.com"}]}`), nil)
			testutil.InjectClient(t, mockClient)
			var stderr bytes.Buffer
			root := NewRootCmdForTesting()
			root.SetOut(fullWriter{})
			root.SetErr(&stderr)

			code := run(root, []string{"domains", "list", "--output", format})

			assert.Equal(t, 1, code)
			assert.Contains(t, stderr.String(), "Error: ")
			assert.Contains(t, stderr.String(), "write /dev/stdout: no space left on device\n")
		})
	}
}

// TestAPIErrorTextIsEscapedOnStderr checks that an API error message holding
// ESC, BEL and a bidirectional override is printed with them escaped in
// table and plain output, and that JSON output prints the API body exactly.
func TestAPIErrorTextIsEscapedOnStderr(t *testing.T) {
	const listID = "123e4567-e89b-12d3-a456-426614174000"
	const raw = `{"message":"list \u001b]0;pwned\u0007 not \u202efound"}`
	apiErr := &api.APIError{StatusCode: 404, Message: "list \x1b]0;pwned\x07 not \u202efound", Raw: []byte(raw)}

	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("GetList", uuid.MustParse(listID)).Return(nil, apiErr)
			testutil.InjectClient(t, mockClient)

			stdout, stderr, code := runRoot(t, "lists", "get", listID, "--output", format)

			assert.Equal(t, 5, code)
			assert.Empty(t, stdout)
			assert.Contains(t, stderr, `list \u001b]0;pwned\u0007 not \u202efound`)
			assert.NotContains(t, stderr, "\x1b")
			assert.NotContains(t, stderr, "\x07")
			assert.NotContains(t, stderr, "\u202e")
		})
	}
	t.Run("json", func(t *testing.T) {
		mockClient := &mocks.MockClient{}
		mockClient.On("GetList", uuid.MustParse(listID)).Return(nil, apiErr)
		testutil.InjectClient(t, mockClient)

		stdout, stderr, code := runRoot(t, "lists", "get", listID, "--output", "json")

		assert.Equal(t, 5, code)
		assert.Equal(t, testutil.IndentedJSON(t, raw), stdout)
		assert.Empty(t, stderr)
	})
}

// TestItemsFailedLineIsEscaped checks that the stderr line of a result that
// was printed escapes its control characters.
func TestItemsFailedLineIsEscaped(t *testing.T) {
	cmd, _, stderr := newHandleErrorTestCommand(t, "plain")

	handleError(cmd, clierrors.NewResultPrintedError("sent \x1b[2J", 1))

	assert.Equal(t, `sent \u001b[2J`+"\n", stderr.String())
}
