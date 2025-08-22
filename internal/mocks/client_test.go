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
