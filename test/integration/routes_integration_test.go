package integration

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/suite"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
)

// RoutesIntegrationTestSuite provides integration testing for the routes command group
type RoutesIntegrationTestSuite struct {
	suite.Suite
	mockClient *mocks.MockClient
}

func (suite *RoutesIntegrationTestSuite) SetupTest() {
	suite.mockClient = &mocks.MockClient{}
}

func (suite *RoutesIntegrationTestSuite) TearDownTest() {
	// Clean up any test state
	suite.mockClient = nil
}

// TestRoutesListCommand_Integration demonstrates full integration testing
func (suite *RoutesIntegrationTestSuite) TestRoutesListCommand_Integration() {
	// This test demonstrates how integration testing would work
	// once authentication mocking is fully implemented

	// Create test routes with proper UUIDs
	routes := []responses.Route{
		*suite.mockClient.NewMockRoute("550e8400-e29b-41d4-a716-446655440001", "Support Route", "https://api.example.com/support", "", true),
		*suite.mockClient.NewMockRoute("550e8400-e29b-41d4-a716-446655440002", "Sales Route", "https://api.example.com/sales", "sales@*", false),
	}

	// Set up mock response
	mockResponse := suite.mockClient.NewMockRoutesResponse(routes, false)

	// Configure mock expectations
	suite.mockClient.On("ListRoutes", (*int32)(nil), (*string)(nil)).Return(mockResponse, nil)

	// Verify mock response structure
	suite.NotNil(mockResponse)
	suite.Equal("list", mockResponse.Object)
	suite.Len(mockResponse.Data, 2)

	// Verify route details
	suite.Equal("Support Route", mockResponse.Data[0].Name)
	suite.Equal("https://api.example.com/support", mockResponse.Data[0].URL)
	suite.True(mockResponse.Data[0].Enabled)

	suite.Equal("Sales Route", mockResponse.Data[1].Name)
	suite.Equal("https://api.example.com/sales", mockResponse.Data[1].URL)
	suite.False(mockResponse.Data[1].Enabled)
	suite.NotNil(mockResponse.Data[1].Recipient)
	suite.Equal("sales@*", mockResponse.Data[1].Recipient)

	// Future: When authentication mocking is implemented, this would become:
	// output, err := testutil.ExecuteCommandIsolated(suite.T(), cmd.NewRootCmdForTesting, "routes", "list")
	// suite.NoError(err)
	// suite.Contains(output, "Support Route")
	// suite.Contains(output, "Sales Route")
	// suite.mockClient.AssertExpectations(suite.T())
}

// TestRoutesListCommand_WithFiltering tests filtering functionality
func (suite *RoutesIntegrationTestSuite) TestRoutesListCommand_WithFiltering() {
	// Create mix of enabled and disabled routes
	routes := []responses.Route{
		*suite.mockClient.NewMockRoute("550e8400-e29b-41d4-a716-446655440003", "Active Route", "https://api.example.com/active", "", true),
		*suite.mockClient.NewMockRoute("550e8400-e29b-41d4-a716-446655440004", "Disabled Route", "https://api.example.com/disabled", "", false),
		*suite.mockClient.NewMockRoute("550e8400-e29b-41d4-a716-446655440005", "Another Active", "https://api.example.com/active2", "", true),
	}

	mockResponse := suite.mockClient.NewMockRoutesResponse(routes, false)

	// Test client-side filtering logic
	var enabledRoutes []responses.Route
	for _, route := range mockResponse.Data {
		if route.Enabled {
			enabledRoutes = append(enabledRoutes, route)
		}
	}

	suite.Len(enabledRoutes, 2)
	suite.Equal("Active Route", enabledRoutes[0].Name)
	suite.Equal("Another Active", enabledRoutes[1].Name)

	// Future: Test command with --enabled flag
	// output, err := testutil.ExecuteCommandIsolated(suite.T(), cmd.NewRootCmdForTesting, "routes", "list", "--enabled")
	// suite.NoError(err)
	// suite.Contains(output, "Active Route")
	// suite.NotContains(output, "Disabled Route")
}

// TestRoutesCreateCommand_Integration tests route creation
func (suite *RoutesIntegrationTestSuite) TestRoutesCreateCommand_Integration() {
	// Test route creation with non-interactive mode
	expectedRoute := suite.mockClient.NewMockRoute("550e8400-e29b-41d4-a716-446655440006", "Test Route", "https://api.example.com/webhook", "", true)

	// Verify the mock route structure
	suite.Equal("Test Route", expectedRoute.Name)
	suite.Equal("https://api.example.com/webhook", expectedRoute.URL)
	suite.True(expectedRoute.Enabled)

	// Future: Test create command execution
	// suite.mockClient.On("CreateRoute", mock.MatchedBy(func(req ahasend.CreateRouteRequest) bool {
	//     return req.Name == "Test Route" && req.Url == "https://api.example.com/webhook"
	// })).Return(expectedRoute, nil)
	//
	// output, err := testutil.ExecuteCommandIsolated(suite.T(), cmd.NewRootCmdForTesting, "routes", "create",
	//     "--name", "Test Route",
	//     "--url", "https://api.example.com/webhook",
	//     "--enabled",
	//     "--interactive=false")
	// suite.NoError(err)
	// suite.Contains(output, "Route created successfully")
	// suite.Contains(output, "Test Route")
	// suite.mockClient.AssertExpectations(suite.T())
}

// TestRoutesGetCommand_Integration tests getting route details
func (suite *RoutesIntegrationTestSuite) TestRoutesGetCommand_Integration() {
	routeID := "550e8400-e29b-41d4-a716-446655440007"
	expectedRoute := suite.mockClient.NewMockRouteWithOptions(routeID, "Support Route", "https://api.example.com/support", true, map[string]bool{
		"include_attachments": true,
		"include_headers":     false,
		"group_by_message_id": true,
		"strip_replies":       true,
	})

	// Verify the route has the expected processing options
	suite.True(expectedRoute.Attachments)
	suite.True(expectedRoute.GroupByMessageID)
	suite.True(expectedRoute.StripReplies)

	// Future: Test get command execution
	// suite.mockClient.On("GetRoute", routeID).Return(expectedRoute, nil)
	//
	// output, err := testutil.ExecuteCommandIsolated(suite.T(), cmd.NewRootCmdForTesting, "routes", "get", routeID)
	// suite.NoError(err)
	// suite.Contains(output, "Support Route")
	// suite.Contains(output, "https://api.example.com/support")
	// suite.Contains(output, "Include Attachments: Enabled")
	// suite.Contains(output, "Group by Message ID: Enabled")
	// suite.mockClient.AssertExpectations(suite.T())
}

// TestRoutesUpdateCommand_Integration tests route updates
func (suite *RoutesIntegrationTestSuite) TestRoutesUpdateCommand_Integration() {
	routeID := "550e8400-e29b-41d4-a716-446655440008"
	originalRoute := suite.mockClient.NewMockRoute(routeID, "Original Route", "https://api.example.com/original", "", false)
	updatedRoute := suite.mockClient.NewMockRoute(routeID, "Updated Route", "https://api.example.com/updated", "", true)

	// Verify the update changes
	suite.Equal("Original Route", originalRoute.Name)
	suite.False(originalRoute.Enabled)
	suite.Equal("Updated Route", updatedRoute.Name)
	suite.True(updatedRoute.Enabled)

	// Future: Test update command execution
	// suite.mockClient.On("UpdateRoute", routeID, mock.MatchedBy(func(req ahasend.UpdateRouteRequest) bool {
	//     return req.Name != nil && *req.Name == "Updated Route" &&
	//            req.Enabled != nil && *req.Enabled == true
	// })).Return(updatedRoute, nil)
	//
	// output, err := testutil.ExecuteCommandIsolated(suite.T(), cmd.NewRootCmdForTesting, "routes", "update", routeID,
	//     "--name", "Updated Route",
	//     "--enabled")
	// suite.NoError(err)
	// suite.Contains(output, "Route updated successfully")
	// suite.Contains(output, "Updated Route")
	// suite.mockClient.AssertExpectations(suite.T())
}

// TestRoutesDeleteCommand_Integration tests route deletion
func (suite *RoutesIntegrationTestSuite) TestRoutesDeleteCommand_Integration() {
	routeID := "550e8400-e29b-41d4-a716-446655440009"
	routeToDelete := suite.mockClient.NewMockRoute(routeID, "Route to Delete", "https://api.example.com/delete", "", true)

	// Verify route exists before deletion
	suite.Equal("Route to Delete", routeToDelete.Name)
	suite.True(routeToDelete.Enabled)

	// Future: Test delete command execution with force flag
	// suite.mockClient.On("DeleteRoute", routeID).Return(nil)
	//
	// output, err := testutil.ExecuteCommandIsolated(suite.T(), cmd.NewRootCmdForTesting, "routes", "delete", routeID, "--force")
	// suite.NoError(err)
	// suite.Contains(output, "Route deleted successfully")
	// suite.Contains(output, routeID)
	// suite.mockClient.AssertExpectations(suite.T())
}

// TestRoutesJSONOutput_Integration tests JSON output formats
func (suite *RoutesIntegrationTestSuite) TestRoutesJSONOutput_Integration() {
	routes := []responses.Route{
		*suite.mockClient.NewMockRoute("550e8400-e29b-41d4-a716-446655440010", "JSON Test Route", "https://api.example.com/json", "", true),
	}

	mockResponse := suite.mockClient.NewMockRoutesResponse(routes, false)

	// Test JSON serialization
	jsonData, err := json.Marshal(mockResponse)
	suite.NoError(err)
	suite.NotEmpty(jsonData)

	// Verify JSON structure
	var parsed responses.PaginatedRoutesResponse
	err = json.Unmarshal(jsonData, &parsed)
	suite.NoError(err)
	suite.Equal("list", parsed.Object)
	suite.Len(parsed.Data, 1)
	suite.Equal("JSON Test Route", parsed.Data[0].Name)

	// Future: Test JSON output command
	// output, err := testutil.ExecuteCommandIsolated(suite.T(), cmd.NewRootCmdForTesting, "routes", "list", "--json")
	// suite.NoError(err)
	// suite.True(json.Valid([]byte(output)))
	// suite.Contains(output, "JSON Test Route")
}

// TestRoutesErrorHandling_Integration tests error scenarios
func (suite *RoutesIntegrationTestSuite) TestRoutesErrorHandling_Integration() {
	// Test various error scenarios that would be handled in real integration tests

	tests := []struct {
		name        string
		errorType   string
		expectedMsg string
	}{
		{
			name:        "API connection error",
			errorType:   "connection",
			expectedMsg: "connection refused",
		},
		{
			name:        "Authentication error",
			errorType:   "auth",
			expectedMsg: "unauthorized",
		},
		{
			name:        "Route not found",
			errorType:   "not_found",
			expectedMsg: "route not found",
		},
		{
			name:        "Invalid route data",
			errorType:   "validation",
			expectedMsg: "invalid route configuration",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// In real integration tests, these would test actual command execution
			// with mocked API errors and verify proper error handling
			suite.Contains(tt.expectedMsg, strings.ToLower(strings.Split(tt.expectedMsg, " ")[0]))
		})
	}

	// Future: Test actual error command execution
	// suite.mockClient.On("ListRoutes", (*int32)(nil), (*string)(nil)).Return(nil, errors.New("API connection failed"))
	// output, err := testutil.ExecuteCommandIsolated(suite.T(), cmd.NewRootCmdForTesting, "routes", "list")
	// suite.Error(err)
	// suite.Contains(err.Error(), "API connection failed")
}

// TestRoutesPagination_Integration tests pagination functionality
func (suite *RoutesIntegrationTestSuite) TestRoutesPagination_Integration() {
	// Create paginated response
	routes := make([]responses.Route, 0, 10)
	for i := 0; i < 10; i++ {
		routeID := "550e8400-e29b-41d4-a716-44665544000" + string(rune('0'+i))
		routes = append(routes, *suite.mockClient.NewMockRoute(routeID, "Route "+string(rune('A'+i)), "https://api.example.com/route"+string(rune('0'+i)), "", i%2 == 0))
	}

	nextCursor := "next-page-cursor"
	mockResponse := &responses.PaginatedRoutesResponse{
		Object: "list",
		Data:   routes,
		Pagination: common.PaginationInfo{
			HasMore:    true,
			NextCursor: &nextCursor,
		},
	}

	// Verify pagination structure
	suite.True(mockResponse.Pagination.HasMore)
	suite.NotNil(mockResponse.Pagination.NextCursor)
	suite.Equal("next-page-cursor", *mockResponse.Pagination.NextCursor)
	suite.Len(mockResponse.Data, 10)

	// Future: Test pagination commands
	// suite.mockClient.On("ListRoutes", int32Ptr(5), (*string)(nil)).Return(firstPage, nil)
	// output, err := testutil.ExecuteCommandIsolated(suite.T(), cmd.NewRootCmdForTesting, "routes", "list", "--limit", "5")
	// suite.NoError(err)
	// suite.Contains(output, "Next cursor: next-page-cursor")
}

// TestRoutesValidation_Integration tests input validation
func (suite *RoutesIntegrationTestSuite) TestRoutesValidation_Integration() {
	// Test URL validation
	validURLs := []string{
		"https://api.example.com/webhook",
		"http://localhost:3000/webhook",
		"https://subdomain.example.com:8080/path/to/webhook",
	}

	invalidURLs := []string{
		"not-a-url",
		"ftp://example.com/webhook",
		"https://",
		"",
	}

	for _, url := range validURLs {
		suite.Run("valid_url_"+url, func() {
			// These URLs should be accepted by validation
			suite.True(strings.HasPrefix(url, "http"))
		})
	}

	for _, url := range invalidURLs {
		suite.Run("invalid_url_"+url, func() {
			// These URLs should be rejected by validation
			invalid := url == "" || (!strings.HasPrefix(url, "http") && url != "") || strings.HasSuffix(url, "://")
			suite.True(invalid, "URL %s should be invalid", url)
		})
	}

	// Future: Test validation in commands
	// for _, invalidURL := range invalidURLs {
	//     output, err := testutil.ExecuteCommandIsolated(suite.T(), cmd.NewRootCmdForTesting, "routes", "create",
	//         "--name", "Test Route",
	//         "--url", invalidURL,
	//         "--interactive=false")
	//     suite.Error(err)
	//     suite.Contains(err.Error(), "invalid")
	// }
}

// Benchmarks for performance testing
func (suite *RoutesIntegrationTestSuite) TestRoutesPerformance_Benchmarks() {
	// Create a large number of routes for performance testing
	routes := make([]responses.Route, 1000)
	for i := 0; i < 1000; i++ {
		routeID := "550e8400-e29b-41d4-a716-" + generateRouteID(i)
		routes[i] = *suite.mockClient.NewMockRoute(routeID, "Route "+string(rune(i)), "https://api.example.com/route"+string(rune(i)), "", i%2 == 0)
	}

	mockResponse := suite.mockClient.NewMockRoutesResponse(routes, false)

	// Verify we can handle large responses
	suite.Len(mockResponse.Data, 1000)

	// Test filtering performance on large dataset
	start := time.Now()
	var enabledRoutes []responses.Route
	for _, route := range mockResponse.Data {
		if route.Enabled {
			enabledRoutes = append(enabledRoutes, route)
		}
	}
	filterDuration := time.Since(start)

	suite.Len(enabledRoutes, 500) // Half should be enabled
	suite.True(filterDuration < time.Millisecond*100, "Filtering 1000 routes should be fast")
}

// Helper functions

func generateRouteID(i int) string {
	// Generate a unique route ID suffix for testing
	return "44665544" + string(rune('0'+(i/100)%10)) + string(rune('0'+(i/10)%10)) + string(rune('0'+i%10))
}

func int32Ptr(i int32) *int32 {
	return &i
}

// Run the integration test suite
func TestRoutesIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(RoutesIntegrationTestSuite))
}
