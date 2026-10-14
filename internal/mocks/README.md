# Mock Infrastructure

This package provides `MockClient`, a mock of the `client.AhaSendClient` interface for testing API operations without calling the API. It uses the [testify/mock](https://github.com/stretchr/testify#mock-package) framework.

## MockClient

A test sets the calls it expects a command to make, installs the mock as the
command's client with `testutil.RunWithClient` (or `testutil.InjectClient`),
runs the command and checks its output. A test that calls the mock directly
only checks the mock, so tests always go through a command.

```go
import (
    "testing"

    "github.com/AhaSend/ahasend-cli/cmd/groups/domains"
    "github.com/AhaSend/ahasend-cli/internal/mocks"
    "github.com/AhaSend/ahasend-cli/internal/testutil"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestDomainsGet(t *testing.T) {
    mockClient := &mocks.MockClient{}

    // Set up the call the command makes
    mockClient.On("GetDomain", "example.com").Return(mocks.NewDomain(), nil)

    // Run the command with the mock as its client
    output, err := testutil.RunWithClient(t, mockClient, domains.NewGetCommand(), "plain", "example.com")

    require.NoError(t, err)
    assert.Contains(t, output, "example.com")

    // Verify the command made every expected call
    mockClient.AssertExpectations(t)
}
```

### Methods

`MockClient` has every method of the
[`client.AhaSendClient` interface](../client/interface.go), with the same
signature; a test sets an expectation with the method's name and arguments.
A method that takes request options, such as an idempotency key, passes
each option to the mock as one more argument, so a test matches it with
`mock.MatchedBy` or `testutil.IdempotencyKeyArg`.

### Raw API Responses

JSON output prints the API's response body unchanged, so a test of JSON output
needs a result that carries its body. `mocks.APIResponse[T](t, body)` decodes
`body` into a new `T`, records `body` as the API response behind it, and
returns the `*T` for use in `Return`. Table, plain and CSV output then see the
decoded struct, and JSON output prints `body`:

```go
func TestListCommand_JSONPrintsBody(t *testing.T) {
    body := `{"object":"list","data":[{"domain":"example.com","future_field":1.50}],"pagination":{"has_more":false}}`
    mockClient := &mocks.MockClient{}
    mockClient.On("ListDomains", requests.GetDomainsParams{}).
        Return(mocks.APIResponse[responses.PaginatedDomainsResponse](t, body), nil)

    out, err := testutil.RunWithClient(t, mockClient, NewListCommand(), "json")

    require.NoError(t, err)
    assert.Equal(t, testutil.IndentedJSON(t, body), out)
    mockClient.AssertExpectations(t)
}
```

A body that does not decode into `T` fails the test. A result built as a Go
value, such as one of the fixtures below, has no recorded body, and JSON
output encodes the struct instead.

### Fixtures

`fixtures.go` holds the fixtures that the tests share, in one place. Their
values are fixed, so a test can compare output exactly:

- `NewSubAccount() *responses.SubAccount`: a sub-account with every field
  set; its ID is `FixtureSubAccountID` and its parent account
  `FixtureParentAccountID`
- `NewSubAccountUsage(removedReception int64) *responses.SubAccountUsageResponse`:
  the June 2026 usage of the parent account and one sub-account, with
  `removedReception` messages of removed sub-accounts
- `NewAPIKey() *responses.APIKey`: an API key of the sub-account of
  `NewSubAccount`, with the ID `FixtureAPIKeyID`, without scopes, IP allow
  list or secret key
- `NewMessageResponse(messageID string) *responses.CreateMessageResponse`:
  the response of a send to one recipient
- `NewDomain() *responses.Domain`: the domain `example.com` of the account
  `FixtureSubAccountID`, with the ID `FixtureDomainID` and a DMARC and a
  tracking DNS record

```go
apiKey := mocks.NewAPIKey()
apiKey.Scopes = []responses.APIKeyScope{{Scope: "messages:send:all"}}
mockClient.On("GetAPIKey", uuid.MustParse(mocks.FixtureAPIKeyID)).Return(apiKey, nil)
```

## Design Principles

### Interface Compliance

`MockClient` implements the client interface with compile-time verification:

```go
// Ensures MockClient implements AhaSendClient
var _ client.AhaSendClient = (*MockClient)(nil)
```

### Consistent Error Handling

Every `MockClient` method that returns `(*T, error)` reads its results with
the `result[T]` helper, which returns a nil `*T` when the first result is nil,
typed or untyped, so `Return(nil, err)` never panics:

```go
func (m *MockClient) CreateDomain(req requests.CreateDomainRequest, opts ...api.RequestOption) (*responses.Domain, error) {
    return result[responses.Domain](m.Called(withOptions([]interface{}{req}, opts)...))
}
```

## Testing Patterns

### Error Testing

```go
func TestDomainsGetNotFound(t *testing.T) {
    mockClient := &mocks.MockClient{}
    mockClient.On("GetDomain", "example.com").
        Return(nil, &api.APIError{StatusCode: 404, Message: "domain not found"})

    _, err := testutil.RunWithClient(t, mockClient, domains.NewGetCommand(), "plain", "example.com")

    require.Error(t, err)
    assert.Equal(t, 5, clierrors.GetExitCode(err))
    mockClient.AssertExpectations(t)
}
```

### Validation Before Authentication

`testutil.RefuseAuth` makes a command that asks for its client fail the test
by name and stop with an error, with no client to call, and returns a flag
that reports whether the command asked, so a test can check that bad input
fails before any request. A regression then fails that one test: a
`MockClient` without expectations would panic on the call instead, and in
`messages send`'s worker goroutines that ends the whole test binary.

```go
func TestDomainsCreateRejectsABadDomain(t *testing.T) {
    called := testutil.RefuseAuth(t)

    _, err := testutil.RunPlain(t, domains.NewCreateCommand(), "not a domain")

    require.Error(t, err)
    assert.False(t, *called)
}
```

## Best Practices

1. **Test a command, not the mock**: run the command with the mock as its client, and check its output, error and exit code
2. **Always call `AssertExpectations(t)`** at the end of tests to verify all expected calls were made
3. **Use the shared fixtures** when possible to create consistent test data
4. **Use `mocks.APIResponse`** for any test that checks JSON output
5. **Set up specific expectations** for each test rather than generic catch-all expectations
6. **Test error cases** explicitly with appropriate error expectations
7. **Keep mock setup close to usage** to improve test readability
8. **Use meaningful test data** that reflects realistic scenarios
