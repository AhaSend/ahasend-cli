package mocks

import (
	"encoding/json"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/apiresponse"
)

// APIResponse decodes body into a new T, records body as the API response
// behind it, and returns it for use in a mock's Return. Table and plain
// output then see the decoded struct, and JSON output sees body.
func APIResponse[T any](t testing.TB, body string) *T {
	t.Helper()
	result := new(T)
	if err := json.Unmarshal([]byte(body), result); err != nil {
		t.Fatalf("mocks.APIResponse: decode %T: %v", result, err)
	}
	apiresponse.Remember(result, []byte(body))
	return result
}
