package printer

import (
	"bytes"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResponseHandler_CSV tests CSV output functionality
func TestResponseHandler_CSV(t *testing.T) {
	tests := []struct {
		name         string
		testFunc     func(handler ResponseHandler) error
		expectOutput bool
	}{
		{
			name: "Domain list CSV",
			testFunc: func(handler ResponseHandler) error {
				response := &responses.PaginatedDomainsResponse{
					Data: []responses.Domain{
						{Domain: "example.com", DNSValid: true},
						{Domain: "test.com", DNSValid: false},
					},
				}
				return handler.HandleDomainList(response, ListConfig{
					EmptyMessage: "No domains found",
					FieldOrder:   []string{"domain", "dns_valid"},
				})
			},
			expectOutput: true,
		},
		{
			name: "Simple success CSV",
			testFunc: func(handler ResponseHandler) error {
				return handler.HandleSimpleSuccess("Operation completed")
			},
			expectOutput: false, // CSV format doesn't output success messages
		},
		{
			name: "Empty domain list CSV",
			testFunc: func(handler ResponseHandler) error {
				response := &responses.PaginatedDomainsResponse{
					Data: []responses.Domain{},
				}
				return handler.HandleDomainList(response, ListConfig{
					EmptyMessage: "No domains found",
				})
			},
			expectOutput: false, // CSV format doesn't output anything for empty results
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := GetResponseHandler("csv", false, &buf)

			err := tt.testFunc(handler)

			require.NoError(t, err)
			if tt.expectOutput {
				assert.NotEmpty(t, buf.String())
			}
		})
	}
}

// TestResponseHandler_SingleItemCSV tests single item CSV output
func TestResponseHandler_SingleItemCSV(t *testing.T) {
	var buf bytes.Buffer
	handler := GetResponseHandler("csv", false, &buf)

	domain := &responses.Domain{
		Domain:   "example.com",
		DNSValid: true,
	}

	err := handler.HandleSingleDomain(domain, SingleConfig{
		SuccessMessage: "Domain retrieved",
		FieldOrder:     []string{"domain", "dns_valid"},
	})

	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

// TestResponseHandler_StatsCSV tests statistics CSV output
func TestResponseHandler_StatsCSV(t *testing.T) {
	var buf bytes.Buffer
	handler := GetResponseHandler("csv", false, &buf)

	response := &responses.DeliverabilityStatisticsResponse{
		Data: []responses.DeliverabilityStatistics{
			{
				FromTimestamp:  time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				ToTimestamp:    time.Date(2023, 1, 1, 1, 0, 0, 0, time.UTC),
				DeliveredCount: 100,
				BouncedCount:   5,
				FailedCount:    2,
			},
			{
				FromTimestamp:  time.Date(2023, 1, 1, 1, 0, 0, 0, time.UTC),
				ToTimestamp:    time.Date(2023, 1, 1, 2, 0, 0, 0, time.UTC),
				DeliveredCount: 95,
				BouncedCount:   3,
				FailedCount:    1,
			},
		},
	}

	err := handler.HandleDeliverabilityStats(response, StatsConfig{
		Title:      "Deliverability Statistics",
		ShowChart:  false,
		FieldOrder: []string{"from_timestamp", "to_timestamp", "delivered_count", "bounced_count", "failed_count"},
	})

	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

// TestResponseHandler_ErrorCSV tests error output in CSV format
func TestResponseHandler_ErrorCSV(t *testing.T) {
	var buf bytes.Buffer
	handler := GetResponseHandler("csv", false, &buf)

	err := handler.HandleError(assert.AnError)

	require.Error(t, err) // HandleError returns the error for proper exit codes
	assert.NotEmpty(t, buf.String())
}
