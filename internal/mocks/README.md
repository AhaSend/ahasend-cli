# Mock Infrastructure

This package provides comprehensive mock implementations for testing AhaSend CLI components. All mocks use the [testify/mock](https://github.com/stretchr/testify#mock-package) framework and implement the same interfaces as production code.

## Available Mocks

### MockClient

Mock implementation of `client.AhaSendClient` interface for testing API operations.

```go
import "github.com/AhaSend/ahasend-cli/internal/mocks"

func TestMyCommand(t *testing.T) {
    mockClient := &mocks.MockClient{}
    
    // Setup expectations
    mockClient.On("Ping").Return(nil)
    mockClient.On("GetAccountID").Return("test-account-id")
    
    // Use in your test
    err := mockClient.Ping()
    assert.NoError(t, err)
    
    accountID := mockClient.GetAccountID()
    assert.Equal(t, "test-account-id", accountID)
    
    // Verify all expectations were met
    mockClient.AssertExpectations(t)
}
```

#### Supported Methods

**Authentication & Account:**
- `GetAccountID() string`
- `GetAuthContext() context.Context`
- `Ping() error`
- `ValidateConfiguration() error`

**Message Operations:**
- `SendMessage(req ahasend.CreateMessageRequest) (*ahasend.CreateMessageResponse, error)`
- `SendMessageWithIdempotencyKey(req ahasend.CreateMessageRequest, idempotencyKey string) (*ahasend.CreateMessageResponse, error)`
- `CancelMessage(accountID, messageID string) error`
- `GetMessages(params client.GetMessagesParams) (*ahasend.PaginatedMessagesResponse, error)`

**Domain Operations:**
- `ListDomains(limit *int32, cursor *string) (*ahasend.PaginatedDomainsResponse, error)`
- `CreateDomain(domain string) (*ahasend.Domain, error)`
- `GetDomain(domain string) (*ahasend.Domain, error)`
- `DeleteDomain(domain string) error`

**Webhook Operations:**
- `CreateWebhookVerifier(secret string) (*ahasend.WebhookVerifier, error)`

**Sub-account Operations:**
- `ListSubAccounts(limit *int32, cursor *string) (*responses.PaginatedSubAccountsResponse, error)`
- `CreateSubAccount(req requests.CreateSubAccountRequest, idempotencyKey string) (*responses.SubAccount, error)`
- `GetSubAccountsUsage() (*responses.SubAccountUsageResponse, error)`
- `GetSubAccount(subAccountID string) (*responses.SubAccount, error)`
- `UpdateSubAccount(subAccountID string, req requests.UpdateSubAccountRequest) (*responses.SubAccount, error)`
- `DeleteSubAccount(subAccountID string) (*common.SuccessResponse, error)`
- `SuspendSubAccount(subAccountID string, req requests.SuspendSubAccountRequest) (*responses.SubAccount, error)`
- `UnsuspendSubAccount(subAccountID string) (*responses.SubAccount, error)`

**Sub-account API Key Operations:**
- `ListSubAccountAPIKeys(subAccountID string, limit *int32, cursor *string) (*responses.PaginatedAPIKeysResponse, error)`
- `CreateSubAccountAPIKey(subAccountID string, req requests.CreateAPIKeyRequest, idempotencyKey string) (*responses.APIKey, error)`
- `GetSubAccountAPIKey(subAccountID, keyID string) (*responses.APIKey, error)`
- `UpdateSubAccountAPIKey(subAccountID, keyID string, req requests.UpdateAPIKeyRequest) (*responses.APIKey, error)`
- `DeleteSubAccountAPIKey(subAccountID, keyID string) (*common.SuccessResponse, error)`

#### Helper Methods

MockClient provides helper methods for creating common test data:

```go
// Create a mock domain
domain := mockClient.NewMockDomain("example.com", true)

// Create a mock domains response
domains := []*ahasend.Domain{domain}
response := mockClient.NewMockDomainsResponse(domains, false)

// Create a mock message response
messageResponse := mockClient.NewMockMessageResponse("msg-123")

// Create sub-account fixtures
subAccount := mockClient.NewMockSubAccount(
    "2f3c5d2a-9ef8-4c91-a5f4-79990c8c1d3a",
    "9d0cf9d0-4f5e-4674-bcf1-8ec39968b6e1",
    "Acme Subsidiary",
    "acme.example.com",
    "active",
)
subAccounts := mockClient.NewMockSubAccountsResponse([]responses.SubAccount{*subAccount}, false)
usage := mockClient.NewMockSubAccountUsageResponse(
    "9d0cf9d0-4f5e-4674-bcf1-8ec39968b6e1",
    "2f3c5d2a-9ef8-4c91-a5f4-79990c8c1d3a",
    "Acme Subsidiary",
)

// Create API-key fixtures
apiKey := mockClient.NewMockAPIKey(
    "13b3aa8e-78d3-48a1-92d2-4b8b1228c2dd",
    "2f3c5d2a-9ef8-4c91-a5f4-79990c8c1d3a",
    "Bootstrap key",
    []string{"messages:send:all"},
)
apiKeys := mockClient.NewMockAPIKeysResponse([]responses.APIKey{*apiKey}, false)
```

### MockConfigManager

Mock implementation of `config.ConfigManager` interface for testing configuration operations.

```go
func TestConfigOperation(t *testing.T) {
    mockConfig := &mocks.MockConfigManager{}
    
    // Setup expectations
    profile := config.Profile{
        Name:      "test",
        APIKey:    "test-key",
        AccountID: "test-account",
    }
    mockConfig.On("GetCurrentProfile").Return(&profile, nil)
    mockConfig.On("SetPreference", "output_format", "json").Return(nil)
    
    // Use in your test
    currentProfile, err := mockConfig.GetCurrentProfile()
    assert.NoError(t, err)
    assert.Equal(t, "test", currentProfile.Name)
    
    err = mockConfig.SetPreference("output_format", "json")
    assert.NoError(t, err)
    
    mockConfig.AssertExpectations(t)
}
```

#### Supported Methods

**Configuration File Operations:**
- `Load() error`
- `Save() error`
- `GetConfig() *config.Config`

**Profile Management:**
- `GetCurrentProfile() (*config.Profile, error)`
- `SetProfile(name string, profile config.Profile) error`
- `RemoveProfile(name string) error`
- `ListProfiles() []string`
- `SetDefaultProfile(name string) error`

**Preference Management:**
- `SetPreference(key, value string) error`
- `GetPreference(key string) (string, error)`

#### Helper Methods

MockConfigManager provides helper methods for creating test data:

```go
// Create a mock profile
profile := mockConfig.NewMockProfile("test", "api-key", "account-123")

// Create a mock config
cfg := mockConfig.NewMockConfig()
```

## Design Principles

### Interface Compliance

All mocks implement their corresponding interfaces with compile-time verification:

```go
// Ensures MockClient implements AhaSendClient
var _ client.AhaSendClient = (*MockClient)(nil)

// Ensures MockConfigManager implements ConfigManager
var _ config.ConfigManager = (*MockConfigManager)(nil)
```

### Consistent Error Handling

Mocks handle nil returns properly to avoid panics:

```go
func (m *MockClient) CreateDomain(domain string) (*ahasend.Domain, error) {
    args := m.Called(domain)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*ahasend.Domain), args.Error(1)
}
```

### Helper Methods

Each mock provides helper methods for creating common test data structures, reducing test boilerplate and ensuring consistent test data.

## Testing Patterns

### Basic Mock Setup

```go
func TestCommand(t *testing.T) {
    // Create mock
    mockClient := &mocks.MockClient{}
    
    // Set expectations
    mockClient.On("MethodName", arg1, arg2).Return(returnValue, nil)
    
    // Execute test
    result, err := mockClient.MethodName(arg1, arg2)
    
    // Assert results
    assert.NoError(t, err)
    assert.Equal(t, returnValue, result)
    
    // Verify expectations
    mockClient.AssertExpectations(t)
}
```

### Error Testing

```go
func TestCommandError(t *testing.T) {
    mockClient := &mocks.MockClient{}
    
    // Setup error expectation
    mockClient.On("Ping").Return(errors.New("connection failed"))
    
    err := mockClient.Ping()
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "connection failed")
    
    mockClient.AssertExpectations(t)
}
```

### Complex Scenarios

```go
func TestComplexScenario(t *testing.T) {
    mockClient := &mocks.MockClient{}
    
    // Setup multiple expectations
    domain := mockClient.NewMockDomain("example.com", true)
    mockClient.On("GetDomain", "example.com").Return(domain, nil)
    mockClient.On("DeleteDomain", "example.com").Return(nil)
    
    // Test the scenario
    retrievedDomain, err := mockClient.GetDomain("example.com")
    assert.NoError(t, err)
    assert.Equal(t, "example.com", retrievedDomain.Domain)
    
    err = mockClient.DeleteDomain("example.com")
    assert.NoError(t, err)
    
    mockClient.AssertExpectations(t)
}
```

## Migration Guide

### From Old Mock Patterns

If you're updating existing tests that used the old mock patterns:

#### Old Pattern (Inconsistent)
```go
// Various inconsistent mock approaches
type CustomMock struct {
    // Custom implementation
}
```

#### New Pattern (Standardized)
```go
// Use standardized interface-based mocks
mockClient := &mocks.MockClient{}
mockClient.On("MethodName", args...).Return(returnValue, nil)
```

### Benefits of New Pattern

1. **Interface Compliance**: Compile-time verification that mocks match production interfaces
2. **Consistency**: All mocks follow the same patterns and conventions
3. **Maintainability**: Easy to update when interfaces change
4. **Feature Completeness**: All interface methods are implemented
5. **Helper Methods**: Convenient test data creation
6. **Error Handling**: Proper nil checking to prevent test panics

## Best Practices

1. **Always call `AssertExpectations(t)`** at the end of tests to verify all expected calls were made
2. **Use helper methods** when possible to create consistent test data
3. **Set up specific expectations** for each test rather than generic catch-all expectations
4. **Test error cases** explicitly with appropriate error expectations
5. **Keep mock setup close to usage** to improve test readability
6. **Use meaningful test data** that reflects realistic scenarios
