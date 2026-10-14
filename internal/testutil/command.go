package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/api"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// RunCommand runs a leaf command with a response handler for format installed
// in context, as the root command does, stdin as its input, and stdout and
// stderr captured apart.
func RunCommand(t *testing.T, cmd *cobra.Command, format, stdin string, args ...string) (string, string, error) {
	t.Helper()
	return runCommand(t, cmd, format, bytes.NewBufferString(stdin), args...)
}

func runCommand(t *testing.T, cmd *cobra.Command, format string, stdin io.Reader, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd.SetContext(context.WithValue(context.Background(), printer.ResponseHandlerKey,
		ResponseHandler(t, format, &stdout, &stderr)))
	cmd.SetIn(stdin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

// ResponseHandler returns the response handler for format
// that prints results to writer and errors to errWriter.
func ResponseHandler(t *testing.T, format string, writer, errWriter io.Writer) printer.ResponseHandler {
	t.Helper()
	handler, err := printer.NewResponseHandler(format, writer, errWriter)
	require.NoError(t, err)
	return handler
}

// RunPlain runs a leaf command with plain output and no input, and returns
// its stdout.
func RunPlain(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	stdout, _, err := RunCommand(t, cmd, "plain", "", args...)
	return stdout, err
}

// RunCommandCombined runs a leaf command like RunCommand, with no input and
// stderr written into the same buffer as stdout.
func RunCommandCombined(t *testing.T, cmd *cobra.Command, format string, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetContext(context.WithValue(context.Background(), printer.ResponseHandlerKey,
		ResponseHandler(t, format, &buf, &buf)))
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

// RunWithClient installs c as the authenticated client and runs a leaf
// command like RunCommandCombined.
func RunWithClient(t *testing.T, c client.AhaSendClient, cmd *cobra.Command, format string, args ...string) (string, error) {
	t.Helper()
	InjectClient(t, c)
	return RunCommandCombined(t, cmd, format, args...)
}

// InjectClient installs c as the authenticated client for the rest of the
// test. The returned flag reports whether a command asked for the client,
// which lets tests assert that validation ran before authentication.
func InjectClient(t *testing.T, c client.AhaSendClient) *bool {
	t.Helper()
	called := false
	restore := auth.SetAuthenticatedClientResolverForTesting(func(*cobra.Command) (client.AhaSendClient, error) {
		called = true
		return c, nil
	})
	t.Cleanup(restore)
	return &called
}

// RefuseAuth makes a command that asks for the authenticated client, for the
// rest of the test, fail the test by name and stop with an error, with no
// client to call, so a command that reaches authentication fails the test
// that runs it rather than the whole test binary. The returned flag reports
// whether a command asked, for tests that check that validation runs before
// authentication.
func RefuseAuth(t *testing.T) *bool {
	t.Helper()
	called := false
	restore := auth.SetAuthenticatedClientResolverForTesting(func(cmd *cobra.Command) (client.AhaSendClient, error) {
		called = true
		t.Errorf("%s asked for the authenticated client; it must fail before that", cmd.CommandPath())
		return nil, errAuthRefused
	})
	t.Cleanup(restore)
	return &called
}

// errAuthRefused is the error of a command that asked for the authenticated
// client in a test that refuses it.
var errAuthRefused = stderrors.New("testutil: authentication refused")

// IdempotencyKeyArg matches a request option that sets the Idempotency-Key
// header to key.
func IdempotencyKeyArg(key string) interface{} {
	return mock.MatchedBy(func(option api.RequestOption) bool {
		config := api.RequestConfig{}
		option(&config)
		return config.Headers["Idempotency-Key"] == key
	})
}

// CaptureRequest makes mocked answer the call method, made with args and
// then a request of type R, with response and no error, and store the
// request in captured.
func CaptureRequest[R any](mocked interface {
	On(string, ...any) *mock.Call
}, method string, captured *R, response any, args ...any) {
	matchers := append(append([]any{}, args...), mock.AnythingOfType(fmt.Sprintf("%T", *captured)))
	mocked.On(method, matchers...).Run(func(arguments mock.Arguments) {
		*captured = arguments.Get(len(args)).(R)
	}).Return(response, nil)
}

// WriteTempFile writes content to name in a test directory and returns its
// path.
func WriteTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// WireJSON marshals v the way the SDK sends it and decodes it into a map.
func WireJSON(t *testing.T, v any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(v)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	return decoded
}

// IndentedJSON returns body in the form JSON output prints an API response:
// indented by two spaces and followed by a newline.
func IndentedJSON(t *testing.T, body string) string {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, json.Indent(&buffer, []byte(body), "", "  "))
	return buffer.String() + "\n"
}
