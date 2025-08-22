package mocks

import (
	"testing"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"

	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/config"
)

// TestExampleMockUsage demonstrates how to use the standardized mocks in command tests
func TestExampleMockUsage(t *testing.T) {
	// Create mock client following interface pattern
	mockClient := &MockClient{}

	// Set expectations for domain operations
	domain := mockClient.NewMockDomain("example.com", true)
	mockClient.On("GetDomain", "example.com").Return(domain, nil)

	// Use the mock in your code
	result, err := mockClient.GetDomain("example.com")

	// Verify results
	if err != nil {
		panic("unexpected error")
	}
	if result.Domain != "example.com" {
		panic("unexpected domain")
	}

	// Verify all expectations were met
	mockClient.AssertExpectations(t)
}

// TestComplexCommandScenario shows a complex test scenario using multiple mocks
func TestComplexCommandScenario(t *testing.T) {
	// Setup mocks
	mockClient := &MockClient{}
	mockConfig := &MockConfigManager{}

	// Setup authentication expectations
	profile := mockConfig.NewMockProfile("test", "api-key-123", "account-123")
	mockConfig.On("GetCurrentProfile").Return(&profile, nil)

	// Setup client expectations
	mockClient.On("GetAccountID").Return("account-123")
	mockClient.On("Ping").Return(nil)

	// Setup domain creation expectations
	domain := mockClient.NewMockDomain("newdomain.com", false) // Not verified yet
	mockClient.On("CreateDomain", "newdomain.com").Return(domain, nil)

	// Execute the scenario (this would be your actual command logic)
	currentProfile, err := mockConfig.GetCurrentProfile()
	assert.NoError(t, err)
	assert.Equal(t, "account-123", currentProfile.AccountID)

	// Verify authentication
	accountID := mockClient.GetAccountID()
	assert.Equal(t, "account-123", accountID)

	err = mockClient.Ping()
	assert.NoError(t, err)

	// Create domain
	createdDomain, err := mockClient.CreateDomain("newdomain.com")
	assert.NoError(t, err)
	assert.Equal(t, "newdomain.com", createdDomain.Domain)
	assert.False(t, createdDomain.DNSValid) // Not verified yet

	// Verify all expectations were met
	mockClient.AssertExpectations(t)
	mockConfig.AssertExpectations(t)
}

// TestErrorHandlingScenario demonstrates error handling with mocks
func TestErrorHandlingScenario(t *testing.T) {
	mockClient := &MockClient{}

	// Setup error expectations
	mockClient.On("Ping").Return(assert.AnError)
	mockClient.On("CreateDomain", "invalid.domain").Return(nil, assert.AnError)

	// Test ping error
	err := mockClient.Ping()
	assert.Error(t, err)

	// Test domain creation error
	domain, err := mockClient.CreateDomain("invalid.domain")
	assert.Error(t, err)
	assert.Nil(t, domain)

	mockClient.AssertExpectations(t)
}

// TestBatchOperationsWithMocks demonstrates testing batch operations
func TestBatchOperationsWithMocks(t *testing.T) {
	mockClient := &MockClient{}

	// Setup expectations for multiple operations
	domains := []string{"domain1.com", "domain2.com", "domain3.com"}

	for _, domainName := range domains {
		domain := mockClient.NewMockDomain(domainName, true)
		mockClient.On("CreateDomain", domainName).Return(domain, nil)
	}

	// Execute batch operations
	var createdDomains []*responses.Domain
	for _, domainName := range domains {
		domain, err := mockClient.CreateDomain(domainName)
		assert.NoError(t, err)
		createdDomains = append(createdDomains, domain)
	}

	// Verify results
	assert.Len(t, createdDomains, 3)
	for i, domain := range createdDomains {
		assert.Equal(t, domains[i], domain.Domain)
		assert.True(t, domain.DNSValid)
	}

	mockClient.AssertExpectations(t)
}

// TestInterfaceDecoupling demonstrates how mocks enable interface-based testing
func TestInterfaceDecoupling(t *testing.T) {
	// This function accepts any client that implements the interface
	testableFunction := func(client client.AhaSendClient) error {
		return client.Ping()
	}

	// Test with mock
	mockClient := &MockClient{}
	mockClient.On("Ping").Return(nil)

	err := testableFunction(mockClient)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)

	// The same function could also accept a real client in production
	// This demonstrates proper dependency injection and testability
}

// TestConfigManagerInterface demonstrates config manager interface testing
func TestConfigManagerInterface(t *testing.T) {
	// Function that accepts any config manager implementing the interface
	testableConfigFunction := func(configMgr config.ConfigManager) (*config.Profile, error) {
		return configMgr.GetCurrentProfile()
	}

	// Test with mock
	mockConfig := &MockConfigManager{}
	profile := mockConfig.NewMockProfile("test", "api-key", "account-id")
	mockConfig.On("GetCurrentProfile").Return(&profile, nil)

	result, err := testableConfigFunction(mockConfig)
	assert.NoError(t, err)
	assert.Equal(t, "test", result.Name)
	mockConfig.AssertExpectations(t)
}
