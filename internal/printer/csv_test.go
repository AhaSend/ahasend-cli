package printer

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go"
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
				})
			},
			expectOutput: true,
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
			handler := testHandler(t, "csv", &buf, os.Stderr)

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
	handler := testHandler(t, "csv", &buf, os.Stderr)

	domain := &responses.Domain{
		Domain:   "example.com",
		DNSValid: true,
	}

	err := handler.HandleSingleDomain(domain, SingleConfig{
		SuccessMessage: "Domain retrieved",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

// TestResponseHandler_StatsCSV tests statistics CSV output
func TestResponseHandler_StatsCSV(t *testing.T) {
	var buf bytes.Buffer
	handler := testHandler(t, "csv", &buf, os.Stderr)

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

	err := handler.HandleDeliverabilityStats(response, StatsConfig{Title: "Deliverability Statistics"})

	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

// TestResponseHandler_ErrorCSV tests that CSV output prints errors to stderr
func TestResponseHandler_ErrorCSV(t *testing.T) {
	var stdout, stderr bytes.Buffer
	handler := testHandler(t, "csv", &stdout, &stderr)

	err := handler.HandleError(assert.AnError)

	require.Error(t, err) // HandleError returns the error for proper exit codes
	assert.Empty(t, stdout.String())
	assert.Equal(t, "Error: "+assert.AnError.Error()+"\n", stderr.String())
}

// TestCSVBooleansAreTrueOrFalse checks that every CSV writes booleans as true
// or false, never as the Yes or No of table and plain output.
func TestCSVBooleansAreTrueOrFalse(t *testing.T) {
	renders := map[string]func(ResponseHandler) error{
		"webhook": func(h ResponseHandler) error {
			return h.HandleSingleWebhook(&responses.Webhook{Name: "hook", Enabled: true}, SingleConfig{})
		},
		"auth result": func(h ResponseHandler) error { return h.HandleAuthResult(AuthResult{Profile: "default"}) },
		"auth status": func(h ResponseHandler) error {
			opens := true
			return h.HandleAuthStatus(&AuthStatus{Profile: "default", Valid: ahasend.Bool(true),
				Account: &responses.Account{TrackOpens: &opens}})
		},
	}
	for name, render := range renders {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			require.NoError(t, render(testHandler(t, "csv", &buf, os.Stderr)))

			assert.NotRegexp(t, `(^|,)(Yes|No)(,|\n)`, buf.String())
			assert.Regexp(t, `(^|,)(true|false)(,|\n)`, buf.String())
		})
	}
}

// TestCSVTimesKeepTheOffsetTheAPISent checks that CSV output writes a time
// with the offset of the API's response, not in the local zone or in UTC.
func TestCSVTimesKeepTheOffsetTheAPISent(t *testing.T) {
	previous := time.Local
	time.Local = time.FixedZone("UTC+5", 5*60*60)
	t.Cleanup(func() { time.Local = previous })
	var domain responses.Domain
	require.NoError(t, json.Unmarshal([]byte(`{"domain":"example.com","created_at":"2026-09-10T10:30:00+02:00"}`), &domain))

	output := renderWith(t, "csv", func(h ResponseHandler) error { return h.HandleSingleDomain(&domain, SingleConfig{}) })

	_, rows := readCSV(t, output)
	require.Len(t, rows, 1)
	assert.Equal(t, "2026-09-10T10:30:00+02:00", rows[0]["created_at"])
}

// TestCSVOfAnEmptyResultIsTheHeader checks that a result with no rows still
// prints its header, so a reader of the CSV sees its columns.
func TestCSVOfAnEmptyResultIsTheHeader(t *testing.T) {
	tests := map[string]func(ResponseHandler) error{
		"messages send": func(h ResponseHandler) error {
			return h.HandleCreateMessage(&responses.CreateMessageResponse{}, ResultConfig{})
		},
		"suppressions create": func(h ResponseHandler) error {
			return h.HandleCreateSuppression(&responses.CreateSuppressionResponse{}, ResultConfig{})
		},
		"auth status --all": func(h ResponseHandler) error { return h.HandleAuthProfiles(nil) },
	}
	for name, render := range tests {
		t.Run(name, func(t *testing.T) {
			output := renderWith(t, "csv", render)

			assert.Equal(t, 1, strings.Count(output, "\n"), output)
		})
	}
}

// TestCSVKeepsMessageContentExactly checks that a message's content keeps
// its line breaks and tabs, while its subject goes on one line.
func TestCSVKeepsMessageContentExactly(t *testing.T) {
	content := "Subject: Hi\r\n\r\n\tHello\nthere"
	message := &responses.Message{Subject: "first\nsecond", Content: &content}

	output := renderWith(t, "csv", func(h ResponseHandler) error {
		return h.HandleSingleMessage(message, SingleConfig{})
	})

	records, err := csv.NewReader(strings.NewReader(output)).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 2)
	fields := map[string]string{}
	for i, column := range records[0] {
		fields[column] = records[1][i]
	}
	assert.Equal(t, "first second", fields["subject"])
	assert.Contains(t, output, `"`+content+`"`)
}
