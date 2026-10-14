package cmd

import (
	"bytes"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-go/api"
	"github.com/stretchr/testify/mock"
)

// runRoot runs args through a fresh root command and returns stdout, stderr
// and the exit code.
func runRoot(t *testing.T, args ...string) (string, string, int) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	root := NewRootCmdForTesting()
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	code := run(root, args)
	return stdout.String(), stderr.String(), code
}

// The IDs tests give commands.
const (
	rawTestID      = "11111111-1111-4111-8111-111111111111"
	rawTestOtherID = "22222222-2222-4222-8222-222222222222"
)

// apiCall is one client method a command calls and the result it returns.
type apiCall struct {
	method string
	result func(t *testing.T) any
}

// returns builds an apiCall whose result is body decoded into T, with body
// recorded as the API response behind it.
func returns[T any](method, body string) apiCall {
	return apiCall{method: method, result: func(t *testing.T) any { return mocks.APIResponse[T](t, body) }}
}

// expectAnyArgs makes method return result for any arguments. Mocked client
// methods take a varying number of arguments, so one expectation is set per
// argument count.
func expectAnyArgs(mockClient *mocks.MockClient, method string, returnValues ...any) {
	for count := 0; count <= 6; count++ {
		args := make([]any, count)
		for i := range args {
			args[i] = mock.Anything
		}
		mockClient.On(method, args...).Return(returnValues...).Maybe()
	}
}

// rawErrorBody is an API error body with the same traits as the success
// bodies above.
const rawErrorBody = `{"zeta":1.50,"message":"Refused","future_field":{"nested":[1,"x"]},"big":12345678901234567890}`

func rawAPIError(status int) *api.APIError {
	return &api.APIError{StatusCode: status, Message: "Refused", Raw: []byte(rawErrorBody)}
}
