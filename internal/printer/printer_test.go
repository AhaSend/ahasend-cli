package printer

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := testHandler(t, tt.format, &buf, os.Stderr)

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
			handler := testHandler(t, format, &buf, os.Stderr)

			// Test HandleSingleDomain
			domain := &responses.Domain{
				Domain:   "example.com",
				DNSValid: true,
			}
			err := handler.HandleSingleDomain(domain, SingleConfig{
				SuccessMessage: "Domain retrieved",
			})
			assert.NoError(t, err)
			assert.NotEmpty(t, buf.String())
		})
	}
}

func TestJSONResponseHandler(t *testing.T) {
	var buf bytes.Buffer
	handler := testHandler(t, "json", &buf, os.Stderr)

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
			var buf, errBuf bytes.Buffer
			handler := testHandler(t, format, &buf, &errBuf)

			// Test error handling: JSON prints the error object on stdout,
			// the other formats print the error on stderr.
			err := handler.HandleError(assert.AnError)
			assert.Error(t, err)
			if format == "json" {
				assert.NotEmpty(t, buf.String())
				assert.Empty(t, errBuf.String())
			} else {
				assert.Empty(t, buf.String())
				assert.NotEmpty(t, errBuf.String())
			}
		})
	}
}

func TestResponseHandlerEmptyData(t *testing.T) {
	formats := []string{"json", "table", "plain", "csv"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			var buf bytes.Buffer
			handler := testHandler(t, format, &buf, os.Stderr)

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

func TestNewResponseHandlerRejectsAnUnsupportedFormat(t *testing.T) {
	handler, err := NewResponseHandler("xml", &bytes.Buffer{}, &bytes.Buffer{})

	assert.Nil(t, handler)
	require.Error(t, err)
	assert.Equal(t, "--output must be one of: json, table, plain, csv", err.Error())
	assert.Equal(t, 4, clierrors.GetExitCode(err))
}

func TestGetResponseHandlerFromCommandFallsBackToPlain(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	assert.Equal(t, "plain", GetResponseHandlerFromCommand(cmd).GetFormat())
}
