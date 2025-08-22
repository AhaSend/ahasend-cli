package printer

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResponseHandlerTypes(t *testing.T) {
	tests := []struct {
		name        string
		format      string
		expectError bool
	}{
		{"json handler", "json", false},
		{"table handler", "table", false},
		{"plain handler", "plain", false},
		{"csv handler", "csv", false},
		{"unsupported format", "xml", false}, // Should still create handler, just outputs error
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := GetResponseHandler(tt.format, false, &buf)

			assert.NotNil(t, handler)
			assert.Implements(t, (*ResponseHandler)(nil), handler)
		})
	}
}

func TestResponseHandlerInterfaceMethods(t *testing.T) {
	formats := []string{"json", "table", "plain", "csv"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			var buf bytes.Buffer
			handler := GetResponseHandler(format, false, &buf)

			// Test HandleSimpleSuccess
			err := handler.HandleSimpleSuccess("Operation completed")
			assert.NoError(t, err)
			if format == "csv" {
				// CSV format doesn't output success messages
				assert.Empty(t, buf.String())
			} else {
				assert.NotEmpty(t, buf.String())
			}

			// Reset buffer
			buf.Reset()

			// Test HandleSingleDomain
			domain := &responses.Domain{
				Domain:   "example.com",
				DNSValid: true,
			}
			err = handler.HandleSingleDomain(domain, SingleConfig{
				SuccessMessage: "Domain retrieved",
				FieldOrder:     []string{"domain", "dns_valid"},
			})
			assert.NoError(t, err)
			assert.NotEmpty(t, buf.String())
		})
	}
}

func TestJSONResponseHandler(t *testing.T) {
	var buf bytes.Buffer
	handler := GetResponseHandler("json", false, &buf)

	t.Run("Handle single domain", func(t *testing.T) {
		buf.Reset()
		domain := &responses.Domain{
			Domain:   "example.com",
			DNSValid: true,
		}
		err := handler.HandleSingleDomain(domain, SingleConfig{
			SuccessMessage: "Domain retrieved",
		})
		require.NoError(t, err)

		// Verify JSON is valid and contains domain data
		var result map[string]interface{}
		err = json.Unmarshal(buf.Bytes(), &result)
		assert.NoError(t, err)
		assert.Equal(t, "example.com", result["domain"])
		assert.Equal(t, true, result["dns_valid"])
	})

	t.Run("Handle domain list", func(t *testing.T) {
		buf.Reset()
		response := &responses.PaginatedDomainsResponse{
			Data: []responses.Domain{
				{Domain: "example.com", DNSValid: true},
				{Domain: "test.com", DNSValid: false},
			},
		}
		err := handler.HandleDomainList(response, ListConfig{
			EmptyMessage: "No domains found",
		})
		require.NoError(t, err)

		// Verify JSON structure
		var result map[string]interface{}
		err = json.Unmarshal(buf.Bytes(), &result)
		assert.NoError(t, err)
		assert.Contains(t, result, "data")
	})

	t.Run("Handle simple success", func(t *testing.T) {
		buf.Reset()
		err := handler.HandleSimpleSuccess("Operation completed")
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(buf.Bytes(), &result)
		assert.NoError(t, err)
		assert.Equal(t, "Operation completed", result["message"])
	})

	t.Run("Handle error", func(t *testing.T) {
		buf.Reset()
		err := handler.HandleError(assert.AnError)
		require.Error(t, err) // HandleError returns the error for non-API errors

		var result map[string]interface{}
		err = json.Unmarshal(buf.Bytes(), &result)
		assert.NoError(t, err)
		assert.Equal(t, true, result["error"])
		assert.Contains(t, result, "message")
	})
}

func TestResponseHandlerMessages(t *testing.T) {
	formats := []string{"json", "table", "plain"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			var buf bytes.Buffer
			handler := GetResponseHandler(format, false, &buf)

			// Test simple success
			err := handler.HandleSimpleSuccess("Success message")
			assert.NoError(t, err)
			assert.NotEmpty(t, buf.String())

			// Test error handling
			buf.Reset()
			err = handler.HandleError(assert.AnError)
			// HandleError returns the error for non-API errors (except in JSON mode for API errors)
			if format == "json" {
				assert.Error(t, err) // Non-API errors still return error in JSON mode
			} else {
				assert.Error(t, err) // Table/plain/csv always return the error
			}
			assert.NotEmpty(t, buf.String())
		})
	}
}

func TestResponseHandlerEmptyData(t *testing.T) {
	formats := []string{"json", "table", "plain", "csv"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			var buf bytes.Buffer
			handler := GetResponseHandler(format, false, &buf)

			// Test empty domain list
			response := &responses.PaginatedDomainsResponse{
				Data: []responses.Domain{},
			}
			err := handler.HandleDomainList(response, ListConfig{
				EmptyMessage: "No domains found",
			})
			assert.NoError(t, err)
		})
	}
}

func TestUnsupportedResponseHandler(t *testing.T) {
	var buf bytes.Buffer
	handler := GetResponseHandler("xml", false, &buf)

	t.Run("HandleSimpleSuccess returns error", func(t *testing.T) {
		err := handler.HandleSimpleSuccess("test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported output format: xml")
	})

	t.Run("HandleSingleDomain returns error", func(t *testing.T) {
		domain := &responses.Domain{Domain: "test.com", DNSValid: true}
		err := handler.HandleSingleDomain(domain, SingleConfig{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported output format: xml")
	})
}
