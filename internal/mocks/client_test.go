package mocks

import (
	"context"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"

	"github.com/AhaSend/ahasend-cli/internal/client"
)

func TestMockClient_InterfaceCompliance(t *testing.T) {
	// This test verifies that MockClient implements the AhaSendClient interface
	var _ client.AhaSendClient = (*MockClient)(nil)
}

func TestMockClient_AuthenticationMethods(t *testing.T) {
	mockClient := &MockClient{}

	// Test GetAccountID
	expectedAccountID := "12345678-1234-1234-1234-123456789012"
	mockClient.On("GetAccountID").Return(expectedAccountID)

	accountID := mockClient.GetAccountID()
	assert.Equal(t, expectedAccountID, accountID)
	mockClient.AssertExpectations(t)

	// Test GetAuthContext
	mockClient = &MockClient{}
	expectedCtx := context.WithValue(context.Background(), "test", "value")
	mockClient.On("GetAuthContext").Return(expectedCtx)

	ctx := mockClient.GetAuthContext()
	assert.Equal(t, expectedCtx, ctx)
	mockClient.AssertExpectations(t)

	// Test Ping success
	mockClient = &MockClient{}
	mockClient.On("Ping").Return(nil)

	err := mockClient.Ping()
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)

	// Test Ping failure
	mockClient = &MockClient{}
	mockClient.On("Ping").Return(assert.AnError)

	err = mockClient.Ping()
	assert.Error(t, err)
	mockClient.AssertExpectations(t)
}

func TestMockClient_DomainMethods(t *testing.T) {
	mockClient := &MockClient{}

	// Test CreateDomain
	domain := "example.com"
	expectedDomain := &responses.Domain{
		Domain:    domain,
		DNSValid:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mockClient.On("CreateDomain", domain).Return(expectedDomain, nil)

	result, err := mockClient.CreateDomain(domain)
	assert.NoError(t, err)
	assert.Equal(t, expectedDomain, result)
	mockClient.AssertExpectations(t)

	// Test ListDomains
	mockClient = &MockClient{}
	var limit int32 = 10
	cursor := "test-cursor"
	expectedResponse := &responses.PaginatedDomainsResponse{
		Object: "list",
		Data:   []responses.Domain{*expectedDomain},
		Pagination: common.PaginationInfo{
			HasMore:    false,
			NextCursor: nil,
		},
	}
	mockClient.On("ListDomains", &limit, &cursor).Return(expectedResponse, nil)

	response, err := mockClient.ListDomains(&limit, &cursor)
	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, response)
	mockClient.AssertExpectations(t)
}

func TestMockClient_MessageMethods(t *testing.T) {
	mockClient := &MockClient{}

	// Test SendMessage
	from := common.SenderAddress{Email: "sender@example.com"}
	recipients := []common.Recipient{{Email: "recipient@example.com"}}
	textContent := "Test message"
	req := requests.CreateMessageRequest{
		From:        from,
		Recipients:  recipients,
		Subject:     "Test Subject",
		TextContent: &textContent,
	}
	id := "msg-123"
	expectedResponse := &responses.CreateMessageResponse{
		Object: "message.create",
		Data: []responses.CreateSingleMessageResponse{
			{
				Object: "message",
				ID:     &id,
				Recipient: common.Recipient{
					Email: "recipient@example.com",
				},
				Status: "sent",
			},
		},
	}
	mockClient.On("SendMessage", req).Return(expectedResponse, nil)

	response, err := mockClient.SendMessage(req)
	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, response)
	mockClient.AssertExpectations(t)

	// Test CancelMessage
	mockClient = &MockClient{}
	accountID := "account-123"
	messageID := "msg-123"
	mockClient.On("CancelMessage", accountID, messageID).Return(&common.SuccessResponse{}, nil)

	r, err := mockClient.CancelMessage(accountID, messageID)
	assert.NoError(t, err)
	assert.Equal(t, &common.SuccessResponse{}, r)
	mockClient.AssertExpectations(t)
}

func TestMockClient_SubAccountMethods(t *testing.T) {
	parentID := "9d0cf9d0-4f5e-4674-bcf1-8ec39968b6e1"
	subAccountID := "2f3c5d2a-9ef8-4c91-a5f4-79990c8c1d3a"
	keyID := "13b3aa8e-78d3-48a1-92d2-4b8b1228c2dd"
	idempotencyKey := "sub-create-key"
	keyIdempotencyKey := "key-create-key"
	limit := int32(10)
	cursor := "cursor-1"
	subAccount := (&MockClient{}).NewMockSubAccount(subAccountID, parentID, "Acme Subsidiary", "acme.example.com", "active")
	subAccounts := (&MockClient{}).NewMockSubAccountsResponse([]responses.SubAccount{*subAccount}, false)
	usage := (&MockClient{}).NewMockSubAccountUsageResponse(parentID, subAccountID, "Acme Subsidiary")
	apiKey := (&MockClient{}).NewMockAPIKey(keyID, subAccountID, "Bootstrap key", []string{"messages:send:all"})
	apiKeys := (&MockClient{}).NewMockAPIKeysResponse([]responses.APIKey{*apiKey}, false)
	success := &common.SuccessResponse{Message: "ok"}
	createSubAccountReq := requests.CreateSubAccountRequest{
		Name:    "Acme Subsidiary",
		Website: "acme.example.com",
	}
	updateSubAccountReq := requests.UpdateSubAccountRequest{
		Name: stringPointer("Updated Subsidiary"),
	}
	suspendReq := requests.SuspendSubAccountRequest{Reason: "Customer requested temporary pause"}
	createAPIKeyReq := requests.CreateAPIKeyRequest{
		Label:  "Bootstrap key",
		Scopes: []string{"messages:send:all"},
	}
	updateAPIKeyReq := requests.UpdateAPIKeyRequest{
		Label: stringPointer("Updated bootstrap key"),
	}

	tests := []struct {
		name     string
		setup    func(*MockClient)
		call     func(*MockClient) (any, error)
		expected any
	}{
		{
			name: "ListSubAccounts",
			setup: func(m *MockClient) {
				m.On("ListSubAccounts", &limit, &cursor).Return(subAccounts, nil)
			},
			call: func(m *MockClient) (any, error) {
				return m.ListSubAccounts(&limit, &cursor)
			},
			expected: subAccounts,
		},
		{
			name: "CreateSubAccount",
			setup: func(m *MockClient) {
				m.On("CreateSubAccount", createSubAccountReq, idempotencyKey).Return(subAccount, nil)
			},
			call: func(m *MockClient) (any, error) {
				return m.CreateSubAccount(createSubAccountReq, idempotencyKey)
			},
			expected: subAccount,
		},
		{
			name: "GetSubAccountsUsage",
			setup: func(m *MockClient) {
				m.On("GetSubAccountsUsage").Return(usage, nil)
			},
			call: func(m *MockClient) (any, error) {
				return m.GetSubAccountsUsage()
			},
			expected: usage,
		},
		{
			name: "GetSubAccount",
			setup: func(m *MockClient) {
				m.On("GetSubAccount", subAccountID).Return(subAccount, nil)
			},
			call: func(m *MockClient) (any, error) {
				return m.GetSubAccount(subAccountID)
			},
			expected: subAccount,
		},
		{
			name: "UpdateSubAccount",
			setup: func(m *MockClient) {
				m.On("UpdateSubAccount", subAccountID, updateSubAccountReq).Return(subAccount, nil)
			},
			call: func(m *MockClient) (any, error) {
				return m.UpdateSubAccount(subAccountID, updateSubAccountReq)
			},
			expected: subAccount,
		},
		{
			name: "DeleteSubAccount",
			setup: func(m *MockClient) {
				m.On("DeleteSubAccount", subAccountID).Return(success, nil)
			},
			call: func(m *MockClient) (any, error) {
				return m.DeleteSubAccount(subAccountID)
			},
			expected: success,
		},
		{
			name: "SuspendSubAccount",
			setup: func(m *MockClient) {
				m.On("SuspendSubAccount", subAccountID, suspendReq).Return(subAccount, nil)
			},
			call: func(m *MockClient) (any, error) {
				return m.SuspendSubAccount(subAccountID, suspendReq)
			},
			expected: subAccount,
		},
		{
			name: "UnsuspendSubAccount",
			setup: func(m *MockClient) {
				m.On("UnsuspendSubAccount", subAccountID).Return(subAccount, nil)
			},
			call: func(m *MockClient) (any, error) {
				return m.UnsuspendSubAccount(subAccountID)
			},
			expected: subAccount,
		},
		{
			name: "ListSubAccountAPIKeys",
			setup: func(m *MockClient) {
				m.On("ListSubAccountAPIKeys", subAccountID, &limit, &cursor).Return(apiKeys, nil)
			},
			call: func(m *MockClient) (any, error) {
				return m.ListSubAccountAPIKeys(subAccountID, &limit, &cursor)
			},
			expected: apiKeys,
		},
		{
			name: "CreateSubAccountAPIKey",
			setup: func(m *MockClient) {
				m.On("CreateSubAccountAPIKey", subAccountID, createAPIKeyReq, keyIdempotencyKey).Return(apiKey, nil)
			},
			call: func(m *MockClient) (any, error) {
				return m.CreateSubAccountAPIKey(subAccountID, createAPIKeyReq, keyIdempotencyKey)
			},
			expected: apiKey,
		},
		{
			name: "GetSubAccountAPIKey",
			setup: func(m *MockClient) {
				m.On("GetSubAccountAPIKey", subAccountID, keyID).Return(apiKey, nil)
			},
			call: func(m *MockClient) (any, error) {
				return m.GetSubAccountAPIKey(subAccountID, keyID)
			},
			expected: apiKey,
		},
		{
			name: "UpdateSubAccountAPIKey",
			setup: func(m *MockClient) {
				m.On("UpdateSubAccountAPIKey", subAccountID, keyID, updateAPIKeyReq).Return(apiKey, nil)
			},
			call: func(m *MockClient) (any, error) {
				return m.UpdateSubAccountAPIKey(subAccountID, keyID, updateAPIKeyReq)
			},
			expected: apiKey,
		},
		{
			name: "DeleteSubAccountAPIKey",
			setup: func(m *MockClient) {
				m.On("DeleteSubAccountAPIKey", subAccountID, keyID).Return(success, nil)
			},
			call: func(m *MockClient) (any, error) {
				return m.DeleteSubAccountAPIKey(subAccountID, keyID)
			},
			expected: success,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockClient{}
			tt.setup(mockClient)

			result, err := tt.call(mockClient)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestMockClient_SubAccountMethods_NilReturns(t *testing.T) {
	subAccountID := "2f3c5d2a-9ef8-4c91-a5f4-79990c8c1d3a"
	keyID := "13b3aa8e-78d3-48a1-92d2-4b8b1228c2dd"
	idempotencyKey := "sub-create-key"
	keyIdempotencyKey := "key-create-key"
	limit := int32(10)
	cursor := "cursor-1"
	createSubAccountReq := requests.CreateSubAccountRequest{
		Name:    "Acme Subsidiary",
		Website: "acme.example.com",
	}
	updateSubAccountReq := requests.UpdateSubAccountRequest{
		Name: stringPointer("Updated Subsidiary"),
	}
	suspendReq := requests.SuspendSubAccountRequest{Reason: "Customer requested temporary pause"}
	createAPIKeyReq := requests.CreateAPIKeyRequest{
		Label:  "Bootstrap key",
		Scopes: []string{"messages:send:all"},
	}
	updateAPIKeyReq := requests.UpdateAPIKeyRequest{
		Label: stringPointer("Updated bootstrap key"),
	}

	tests := []struct {
		name  string
		setup func(*MockClient)
		call  func(*MockClient) (any, error)
	}{
		{
			name: "ListSubAccounts",
			setup: func(m *MockClient) {
				m.On("ListSubAccounts", &limit, &cursor).Return(nil, assert.AnError)
			},
			call: func(m *MockClient) (any, error) {
				return m.ListSubAccounts(&limit, &cursor)
			},
		},
		{
			name: "CreateSubAccount",
			setup: func(m *MockClient) {
				m.On("CreateSubAccount", createSubAccountReq, idempotencyKey).Return(nil, assert.AnError)
			},
			call: func(m *MockClient) (any, error) {
				return m.CreateSubAccount(createSubAccountReq, idempotencyKey)
			},
		},
		{
			name: "GetSubAccountsUsage",
			setup: func(m *MockClient) {
				m.On("GetSubAccountsUsage").Return(nil, assert.AnError)
			},
			call: func(m *MockClient) (any, error) {
				return m.GetSubAccountsUsage()
			},
		},
		{
			name: "GetSubAccount",
			setup: func(m *MockClient) {
				m.On("GetSubAccount", subAccountID).Return(nil, assert.AnError)
			},
			call: func(m *MockClient) (any, error) {
				return m.GetSubAccount(subAccountID)
			},
		},
		{
			name: "UpdateSubAccount",
			setup: func(m *MockClient) {
				m.On("UpdateSubAccount", subAccountID, updateSubAccountReq).Return(nil, assert.AnError)
			},
			call: func(m *MockClient) (any, error) {
				return m.UpdateSubAccount(subAccountID, updateSubAccountReq)
			},
		},
		{
			name: "DeleteSubAccount",
			setup: func(m *MockClient) {
				m.On("DeleteSubAccount", subAccountID).Return(nil, assert.AnError)
			},
			call: func(m *MockClient) (any, error) {
				return m.DeleteSubAccount(subAccountID)
			},
		},
		{
			name: "SuspendSubAccount",
			setup: func(m *MockClient) {
				m.On("SuspendSubAccount", subAccountID, suspendReq).Return(nil, assert.AnError)
			},
			call: func(m *MockClient) (any, error) {
				return m.SuspendSubAccount(subAccountID, suspendReq)
			},
		},
		{
			name: "UnsuspendSubAccount",
			setup: func(m *MockClient) {
				m.On("UnsuspendSubAccount", subAccountID).Return(nil, assert.AnError)
			},
			call: func(m *MockClient) (any, error) {
				return m.UnsuspendSubAccount(subAccountID)
			},
		},
		{
			name: "ListSubAccountAPIKeys",
			setup: func(m *MockClient) {
				m.On("ListSubAccountAPIKeys", subAccountID, &limit, &cursor).Return(nil, assert.AnError)
			},
			call: func(m *MockClient) (any, error) {
				return m.ListSubAccountAPIKeys(subAccountID, &limit, &cursor)
			},
		},
		{
			name: "CreateSubAccountAPIKey",
			setup: func(m *MockClient) {
				m.On("CreateSubAccountAPIKey", subAccountID, createAPIKeyReq, keyIdempotencyKey).Return(nil, assert.AnError)
			},
			call: func(m *MockClient) (any, error) {
				return m.CreateSubAccountAPIKey(subAccountID, createAPIKeyReq, keyIdempotencyKey)
			},
		},
		{
			name: "GetSubAccountAPIKey",
			setup: func(m *MockClient) {
				m.On("GetSubAccountAPIKey", subAccountID, keyID).Return(nil, assert.AnError)
			},
			call: func(m *MockClient) (any, error) {
				return m.GetSubAccountAPIKey(subAccountID, keyID)
			},
		},
		{
			name: "UpdateSubAccountAPIKey",
			setup: func(m *MockClient) {
				m.On("UpdateSubAccountAPIKey", subAccountID, keyID, updateAPIKeyReq).Return(nil, assert.AnError)
			},
			call: func(m *MockClient) (any, error) {
				return m.UpdateSubAccountAPIKey(subAccountID, keyID, updateAPIKeyReq)
			},
		},
		{
			name: "DeleteSubAccountAPIKey",
			setup: func(m *MockClient) {
				m.On("DeleteSubAccountAPIKey", subAccountID, keyID).Return(nil, assert.AnError)
			},
			call: func(m *MockClient) (any, error) {
				return m.DeleteSubAccountAPIKey(subAccountID, keyID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockClient{}
			tt.setup(mockClient)

			result, err := tt.call(mockClient)

			assert.Error(t, err)
			assert.Nil(t, result)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestMockClient_HelperMethods(t *testing.T) {
	mockClient := &MockClient{}

	// Test helper method for creating mock domain
	domain := mockClient.NewMockDomain("example.com", true)
	assert.Equal(t, "example.com", domain.Domain)
	assert.True(t, domain.DNSValid)
	assert.False(t, domain.CreatedAt.IsZero())

	// Test helper method for creating domains response
	domains := []responses.Domain{*domain}
	response := mockClient.NewMockDomainsResponse(domains, false)
	assert.Len(t, response.Data, 1)
	assert.False(t, response.Pagination.HasMore)

	// Test helper method for creating message response
	messageResponse := mockClient.NewMockMessageResponse("msg-123")
	assert.Equal(t, "msg-123", *messageResponse.Data[0].ID)

	parentID := "9d0cf9d0-4f5e-4674-bcf1-8ec39968b6e1"
	subAccountID := "2f3c5d2a-9ef8-4c91-a5f4-79990c8c1d3a"
	keyID := "13b3aa8e-78d3-48a1-92d2-4b8b1228c2dd"

	// Test helper method for creating a mock sub-account
	subAccount := mockClient.NewMockSubAccount(subAccountID, parentID, "Acme Subsidiary", "acme.example.com", "active")
	assert.Equal(t, subAccountID, subAccount.ID.String())
	assert.Equal(t, parentID, subAccount.ParentAccountID.String())
	assert.Equal(t, "Acme Subsidiary", subAccount.Name)

	// Test helper method for creating sub-accounts response
	subAccounts := mockClient.NewMockSubAccountsResponse([]responses.SubAccount{*subAccount}, false)
	assert.Len(t, subAccounts.Data, 1)
	assert.False(t, subAccounts.Pagination.HasMore)

	// Test helper method for creating sub-account usage response
	usage := mockClient.NewMockSubAccountUsageResponse(parentID, subAccountID, "Acme Subsidiary")
	assert.Equal(t, "usd", usage.Currency)
	assert.Len(t, usage.SubAccounts, 1)
	assert.Equal(t, "Acme Subsidiary", *usage.SubAccounts[0].Name)

	// Test helper method for creating API key fixtures
	apiKey := mockClient.NewMockAPIKey(keyID, subAccountID, "Bootstrap key", []string{"messages:send:all"})
	assert.Equal(t, keyID, apiKey.ID.String())
	assert.Equal(t, subAccountID, apiKey.AccountID.String())
	assert.Len(t, apiKey.Scopes, 1)

	apiKeys := mockClient.NewMockAPIKeysResponse([]responses.APIKey{*apiKey}, false)
	assert.Len(t, apiKeys.Data, 1)
	assert.False(t, apiKeys.Pagination.HasMore)
}

func TestMockClient_ErrorHandling(t *testing.T) {
	mockClient := &MockClient{}

	// Test error cases return nil for pointers and proper errors
	mockClient.On("CreateDomain", "invalid.domain").Return(nil, assert.AnError)

	result, err := mockClient.CreateDomain("invalid.domain")
	assert.Error(t, err)
	assert.Nil(t, result)
	mockClient.AssertExpectations(t)
}

func stringPointer(v string) *string {
	return &v
}

// BenchmarkMockClient_BasicOperations benchmarks basic mock operations
func BenchmarkMockClient_BasicOperations(b *testing.B) {
	mockClient := &MockClient{}
	mockClient.On("GetAccountID").Return("test-account")
	mockClient.On("Ping").Return(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mockClient.GetAccountID()
		_ = mockClient.Ping()
	}
}
