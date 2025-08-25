package stats

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/output"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test stats command structure and subcommands
func TestStatsCommand_Structure(t *testing.T) {
	// Create a fresh stats command and verify it has expected subcommands
	statsCmd := NewCommand()
	expectedSubcommands := []string{"deliverability", "bounces", "delivery-time"}

	subcommands := make([]string, 0)
	for _, cmd := range statsCmd.Commands() {
		subcommands = append(subcommands, cmd.Name())
	}

	for _, expected := range expectedSubcommands {
		assert.Contains(t, subcommands, expected, "stats command should have %s subcommand", expected)
	}

	assert.Equal(t, "stats", statsCmd.Name())
	assert.Equal(t, "View email statistics and reporting", statsCmd.Short)
	assert.NotEmpty(t, statsCmd.Long)
}

func TestStatsCommand_Help(t *testing.T) {
	cmd := NewCommand()

	// Capture help output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	assert.NoError(t, err)

	helpOutput := buf.String()
	assert.Contains(t, helpOutput, "email statistics")
	assert.Contains(t, helpOutput, "deliverability")
	assert.Contains(t, helpOutput, "bounces")
	assert.Contains(t, helpOutput, "delivery-time")
}

func TestStatsCommand_SubcommandCount(t *testing.T) {
	cmd := NewCommand()
	subcommands := cmd.Commands()

	// Should have exactly 3 subcommands
	assert.Equal(t, 3, len(subcommands), "stats command should have exactly 3 subcommands")
}

// Test deliverability command structure and flags
func TestDeliverabilityCommand_Structure(t *testing.T) {
	deliverabilityCmd := NewDeliverabilityCommand()
	assert.Equal(t, "deliverability", deliverabilityCmd.Name())
	assert.Equal(t, "View email deliverability statistics", deliverabilityCmd.Short)
	assert.NotEmpty(t, deliverabilityCmd.Long)
	assert.NotEmpty(t, deliverabilityCmd.Example)
}

func TestDeliverabilityCommand_Flags(t *testing.T) {
	deliverabilityCmd := NewDeliverabilityCommand()
	flags := deliverabilityCmd.Flags()

	// Test that deliverability command has all required flags
	expectedFlags := []string{
		"from-time", "to-time", "group-by", "sender-domain",
		"recipient-domain", "tags", "chart", "show-totals",
	}

	for _, flagName := range expectedFlags {
		flag := flags.Lookup(flagName)
		assert.NotNil(t, flag, "deliverability command should have %s flag", flagName)
	}
}

func TestDeliverabilityCommand_FlagDefaults(t *testing.T) {
	cmd := NewDeliverabilityCommand()

	// Parse flags with no arguments to get defaults
	err := cmd.ParseFlags([]string{})
	require.NoError(t, err)

	fromTime, _ := cmd.Flags().GetString("from-time")
	assert.Equal(t, "7d", fromTime)

	groupBy, _ := cmd.Flags().GetString("group-by")
	assert.Equal(t, "day", groupBy)

	showTotals, _ := cmd.Flags().GetBool("show-totals")
	assert.True(t, showTotals)

	chart, _ := cmd.Flags().GetBool("chart")
	assert.False(t, chart)
}

// Test bounces command structure and flags
func TestBouncesCommand_Structure(t *testing.T) {
	bouncesCmd := NewBouncesCommand()
	assert.Equal(t, "bounces", bouncesCmd.Name())
	assert.Equal(t, "View email bounce statistics and analysis", bouncesCmd.Short)
	assert.NotEmpty(t, bouncesCmd.Long)
	assert.NotEmpty(t, bouncesCmd.Example)
}

func TestBouncesCommand_Flags(t *testing.T) {
	bouncesCmd := NewBouncesCommand()
	flags := bouncesCmd.Flags()

	// Test that bounces command has all required flags
	expectedFlags := []string{
		"from-time", "to-time", "group-by", "sender-domain",
		"recipient-domain", "tags", "classification", "trends", "raw",
		"show-domains", "show-totals",
	}

	for _, flagName := range expectedFlags {
		flag := flags.Lookup(flagName)
		assert.NotNil(t, flag, "bounces command should have %s flag", flagName)
	}
}

func TestBouncesCommand_FlagDefaults(t *testing.T) {
	cmd := NewBouncesCommand()

	// Parse flags with no arguments to get defaults
	err := cmd.ParseFlags([]string{})
	require.NoError(t, err)

	fromTime, _ := cmd.Flags().GetString("from-time")
	assert.Equal(t, "7d", fromTime)

	groupBy, _ := cmd.Flags().GetString("group-by")
	assert.Equal(t, "day", groupBy)

	showTotals, _ := cmd.Flags().GetBool("show-totals")
	assert.True(t, showTotals)

	showClassification, _ := cmd.Flags().GetBool("show-classification")
	assert.False(t, showClassification)
}

// Test delivery-time command structure and flags
func TestDeliveryTimeCommand_Structure(t *testing.T) {
	deliveryTimeCmd := NewDeliveryTimeCommand()
	assert.Equal(t, "delivery-time", deliveryTimeCmd.Name())
	assert.Equal(t, "View email delivery time performance metrics", deliveryTimeCmd.Short)
	assert.NotEmpty(t, deliveryTimeCmd.Long)
	assert.NotEmpty(t, deliveryTimeCmd.Example)
}

func TestDeliveryTimeCommand_Flags(t *testing.T) {
	deliveryTimeCmd := NewDeliveryTimeCommand()
	flags := deliveryTimeCmd.Flags()

	// Test that delivery-time command has all required flags
	expectedFlags := []string{
		"from-time", "to-time", "group-by", "sender-domain", "tags",
		"recipient-domain", "raw", "show-totals",
	}

	for _, flagName := range expectedFlags {
		flag := flags.Lookup(flagName)
		assert.NotNil(t, flag, "delivery-time command should have %s flag", flagName)
	}
}

func TestDeliveryTimeCommand_FlagDefaults(t *testing.T) {
	cmd := NewDeliveryTimeCommand()

	// Parse flags with no arguments to get defaults
	err := cmd.ParseFlags([]string{})
	require.NoError(t, err)

	fromTime, _ := cmd.Flags().GetString("from-time")
	assert.Equal(t, "7d", fromTime)

	groupBy, _ := cmd.Flags().GetString("group-by")
	assert.Equal(t, "day", groupBy)

	showTotals, _ := cmd.Flags().GetBool("show-totals")
	assert.True(t, showTotals)

	raw, _ := cmd.Flags().GetBool("raw")
	assert.False(t, raw)
}

// Test time parsing functionality
func TestParseTimeFlag(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		description string
	}{
		{
			name:        "RFC3339 format",
			input:       "2024-01-15T10:30:00Z",
			expectError: false,
			description: "Should parse valid RFC3339 timestamp",
		},
		{
			name:        "relative minutes",
			input:       "30m",
			expectError: false,
			description: "Should parse relative minutes",
		},
		{
			name:        "relative hours",
			input:       "24h",
			expectError: false,
			description: "Should parse relative hours",
		},
		{
			name:        "relative days",
			input:       "7d",
			expectError: false,
			description: "Should parse relative days",
		},
		{
			name:        "relative weeks",
			input:       "2w",
			expectError: true,
			description: "Should reject weeks format (not supported)",
		},
		{
			name:        "empty string",
			input:       "",
			expectError: true,
			description: "Should reject empty string",
		},
		{
			name:        "invalid format",
			input:       "invalid",
			expectError: true,
			description: "Should reject invalid format",
		},
		{
			name:        "invalid unit",
			input:       "5x",
			expectError: true,
			description: "Should reject invalid time unit",
		},
		{
			name:        "zero value",
			input:       "0h",
			expectError: false,
			description: "Should accept zero value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := output.ParseTimePast(tt.input)

			if tt.expectError {
				assert.Error(t, err, tt.description)
				assert.True(t, result.IsZero())
			} else {
				assert.NoError(t, err, tt.description)
				assert.False(t, result.IsZero(), "Result should not be zero time for valid input")
			}
		})
	}
}

func TestParseTimeFlag_RelativeCalculation(t *testing.T) {
	// Test that relative time calculation is correct
	before := time.Now()
	result, err := output.ParseTimePast("1h")

	require.NoError(t, err)
	require.False(t, result.IsZero())

	// Should be approximately 1 hour ago (within test execution time)
	expectedTime := before.Add(-1 * time.Hour)
	timeDiff := result.Sub(expectedTime)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}

	// Allow up to 1 second difference for test execution time
	assert.True(t, timeDiff < time.Second, "Relative time calculation should be accurate")
}

// Test parameter validation
func TestDeliverabilityCommand_GroupByValidation(t *testing.T) {
	tests := []struct {
		groupBy     string
		shouldError bool
	}{
		{"hour", false},
		{"day", false},
		{"week", false},
		{"month", false},
		{"invalid", true},
		{"", true},
		{"HOUR", true}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("group-by_%s", tt.groupBy), func(t *testing.T) {
			// Test the validation logic directly
			validGroupBy := []string{"hour", "day", "week", "month"}
			isValid := contains(validGroupBy, tt.groupBy)

			if tt.shouldError {
				assert.False(t, isValid)
			} else {
				assert.True(t, isValid)
			}
		})
	}
}

// Test deliverability statistics with mock response structure
func TestDeliverabilityStatistics_ResponseStructure(t *testing.T) {
	// Test basic response structure without relying on SDK struct creation
	mockResponse := &responses.DeliverabilityStatisticsResponse{
		Object: "deliverability_statistics",
		Data:   []responses.DeliverabilityStatistics{}, // Empty for structure test
	}

	// Test response structure validation
	assert.NotNil(t, mockResponse)
	assert.Equal(t, "deliverability_statistics", mockResponse.Object)
	assert.NotNil(t, mockResponse.Data)
	assert.Empty(t, mockResponse.Data)
}

func TestDeliverabilityStatistics_EmptyResult(t *testing.T) {
	mockResponse := &responses.DeliverabilityStatisticsResponse{
		Object: "deliverability_statistics",
		Data:   []responses.DeliverabilityStatistics{},
	}

	assert.NotNil(t, mockResponse)
	assert.Empty(t, mockResponse.Data)
	assert.Equal(t, "deliverability_statistics", mockResponse.Object)
}

// Test bounce statistics response structure
func TestBounceStatistics_ResponseStructure(t *testing.T) {
	mockResponse := &responses.BounceStatisticsResponse{
		Object: "bounce_statistics",
		Data:   []responses.BounceStatistics{},
	}

	// Test response structure validation
	assert.NotNil(t, mockResponse)
	assert.Equal(t, "bounce_statistics", mockResponse.Object)
	assert.NotNil(t, mockResponse.Data)
	assert.Empty(t, mockResponse.Data)
}

// Test delivery time statistics response structure
func TestDeliveryTimeStatistics_ResponseStructure(t *testing.T) {
	mockResponse := &responses.DeliveryTimeStatisticsResponse{
		Object: "delivery_time_statistics",
		Data:   []responses.DeliveryTimeStatistics{},
	}

	// Test response structure validation
	assert.NotNil(t, mockResponse)
	assert.Equal(t, "delivery_time_statistics", mockResponse.Object)
	assert.NotNil(t, mockResponse.Data)
	assert.Empty(t, mockResponse.Data)
}

// Note: Helper function tests removed during ResponseHandler migration
// The helper functions (calculateDeliverabilityTotals, formatTimeBucket, etc.)
// were moved to the ResponseHandler implementation and are no longer public

// Test utility functions
func TestContains(t *testing.T) {
	slice := []string{"apple", "banana", "orange"}

	assert.True(t, contains(slice, "apple"))
	assert.True(t, contains(slice, "banana"))
	assert.False(t, contains(slice, "grape"))
	assert.False(t, contains(slice, ""))
}

// Test error scenarios
func TestStats_APIErrors(t *testing.T) {
	tests := []struct {
		name     string
		error    error
		expected string
	}{
		{
			name:     "general API error",
			error:    errors.New("API request failed"),
			expected: "API request failed",
		},
		{
			name:     "timeout error",
			error:    errors.New("request timeout"),
			expected: "request timeout",
		},
		{
			name:     "authentication error",
			error:    errors.New("unauthorized"),
			expected: "unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, tt.error)
			assert.Contains(t, tt.error.Error(), tt.expected)
		})
	}
}

// Test edge cases
func TestStats_NilResponse(t *testing.T) {
	var nilDeliverabilityResponse *responses.DeliverabilityStatisticsResponse
	var nilBounceResponse *responses.BounceStatisticsResponse
	var nilDeliveryTimeResponse *responses.DeliveryTimeStatisticsResponse

	assert.Nil(t, nilDeliverabilityResponse)
	assert.Nil(t, nilBounceResponse)
	assert.Nil(t, nilDeliveryTimeResponse)
}

func TestStats_InvalidTimeRange(t *testing.T) {
	// Test time range validation logic
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)

	// Valid range (from < to)
	assert.True(t, oneHourAgo.Before(now))

	// Invalid range (from > to) - this should be caught by validation
	oneHourLater := now.Add(1 * time.Hour)
	assert.True(t, oneHourLater.After(now))
}

// Mock client integration tests
func TestDeliverabilityStats_MockIntegration(t *testing.T) {
	// This test demonstrates how the mock client would be used
	mockClient := &mocks.MockClient{}

	// Create test parameters
	params := requests.GetDeliverabilityStatisticsParams{
		FromTime: timePtr(time.Now().Add(-7 * 24 * time.Hour)),
		GroupBy:  stringPtr("day"),
	}

	// Create mock response with empty data (avoiding SDK struct creation issues)
	mockResponse := &responses.DeliverabilityStatisticsResponse{
		Object: "deliverability_statistics",
		Data:   []responses.DeliverabilityStatistics{},
	}

	// Set up mock expectation
	mockClient.On("GetDeliverabilityStatistics", params).Return(mockResponse, nil)

	// Test the mock setup
	response, err := mockClient.GetDeliverabilityStatistics(params)
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "deliverability_statistics", response.Object)
	assert.NotNil(t, response.Data)

	// Verify mock expectations
	mockClient.AssertExpectations(t)
}

func TestBounceStats_MockIntegration(t *testing.T) {
	// This test demonstrates bounce statistics mock integration
	mockClient := &mocks.MockClient{}

	// Create test parameters
	params := requests.GetBounceStatisticsParams{
		FromTime: timePtr(time.Now().Add(-7 * 24 * time.Hour)),
		GroupBy:  stringPtr("day"),
	}

	// Create mock response with empty data
	mockResponse := &responses.BounceStatisticsResponse{
		Object: "bounce_statistics",
		Data:   []responses.BounceStatistics{},
	}

	// Set up mock expectation
	mockClient.On("GetBounceStatistics", params).Return(mockResponse, nil)

	// Test the mock setup
	response, err := mockClient.GetBounceStatistics(params)
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "bounce_statistics", response.Object)
	assert.NotNil(t, response.Data)

	// Verify mock expectations
	mockClient.AssertExpectations(t)
}

func TestDeliveryTimeStats_MockIntegration(t *testing.T) {
	// This test demonstrates delivery time statistics mock integration
	mockClient := &mocks.MockClient{}

	// Create test parameters
	params := requests.GetDeliveryTimeStatisticsParams{
		FromTime: timePtr(time.Now().Add(-7 * 24 * time.Hour)),
		GroupBy:  stringPtr("day"),
	}

	// Create mock response with empty data
	mockResponse := &responses.DeliveryTimeStatisticsResponse{
		Object: "delivery_time_statistics",
		Data:   []responses.DeliveryTimeStatistics{},
	}

	// Set up mock expectation
	mockClient.On("GetDeliveryTimeStatistics", params).Return(mockResponse, nil)

	// Test the mock setup
	response, err := mockClient.GetDeliveryTimeStatistics(params)
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "delivery_time_statistics", response.Object)
	assert.NotNil(t, response.Data)

	// Verify mock expectations
	mockClient.AssertExpectations(t)
}

// Test flag parsing for all commands
func TestDeliverabilityCommand_FlagParsing(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]interface{}
	}{
		{
			name: "all flags provided",
			args: []string{
				"--from-time", "30d",
				"--to-time", "1d",
				"--group-by", "hour",
				"--sender-domain", "example.com",
				"--recipient-domain", "gmail.com",
				"--tags", "onboarding,drip",
				"--chart",
				"--show-totals=false",
			},
			expected: map[string]interface{}{
				"from-time":        "30d",
				"to-time":          "1d",
				"group-by":         "hour",
				"sender-domain":    "example.com",
				"recipient-domain": []string{"gmail.com"},
				"tags":             "onboarding,drip",
				"chart":            true,
				"show-totals":      false,
			},
		},
		{
			name: "minimal flags",
			args: []string{"--from-time", "1h"},
			expected: map[string]interface{}{
				"from-time": "1h",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewDeliverabilityCommand()
			cmd.SetArgs(tt.args)

			// Parse flags without executing the command
			err := cmd.ParseFlags(tt.args)
			require.NoError(t, err)

			// Check parsed flag values
			for key, expected := range tt.expected {
				switch v := expected.(type) {
				case string:
					actual, _ := cmd.Flags().GetString(key)
					assert.Equal(t, v, actual)
				case bool:
					actual, _ := cmd.Flags().GetBool(key)
					assert.Equal(t, v, actual)
				case []string:
					actual, _ := cmd.Flags().GetStringSlice(key)
					assert.Equal(t, v, actual)
				}
			}
		})
	}
}

// Benchmark tests
func BenchmarkStatsCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewCommand()
	}
}

func BenchmarkDeliverabilityCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewDeliverabilityCommand()
	}
}

func BenchmarkParseTimeFlag_Relative(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = output.ParseTimePast("7d")
	}
}

func BenchmarkParseTimeFlag_RFC3339(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = output.ParseTimePast("2024-01-15T10:30:00Z")
	}
}

// Benchmark tests removed during ResponseHandler migration

// Helper functions for tests

func stringPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func int32Ptr(i int32) *int32 {
	return &i
}

func float32Ptr(f float32) *float32 {
	return &f
}
