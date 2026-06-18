package cmd

import (
	"bytes"
	"context"
	"testing"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/api"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func newHandleErrorTestCommand(t *testing.T, format string) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	t.Cleanup(func() {
		globalExitCode = 0
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	handler := printer.GetResponseHandler(format, false, &stdout)
	ctx := context.WithValue(context.Background(), printer.ResponseHandlerKey, handler)
	cmd.SetContext(ctx)

	return cmd, &stdout, &stderr
}

func TestHandleErrorJSONRawAPIErrorLeavesExitCodeZero(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		raw        string
	}{
		{
			name:       "conflict",
			statusCode: 409,
			raw:        `{"error":"idempotency_conflict","message":"idempotency key is already in use"}`,
		},
		{
			name:       "unprocessable_entity",
			statusCode: 422,
			raw:        `{"error":"validation_failed","message":"request body is invalid"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, stdout, stderr := newHandleErrorTestCommand(t, "json")
			globalExitCode = 99

			handleError(cmd, &api.APIError{
				StatusCode: tt.statusCode,
				Message:    "api request failed",
				Code:       "api_error",
				Raw:        []byte(tt.raw),
			})

			assert.Equal(t, 0, globalExitCode)
			assert.JSONEq(t, tt.raw, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}

func TestHandleErrorNonJSONRawAPIErrorLeavesNonzeroExitCode(t *testing.T) {
	tests := []string{"table", "plain", "csv"}

	for _, format := range tests {
		t.Run(format, func(t *testing.T) {
			cmd, stdout, _ := newHandleErrorTestCommand(t, format)
			globalExitCode = 0

			handleError(cmd, &api.APIError{
				StatusCode: 409,
				Message:    "idempotency key is already in use",
				Code:       "idempotency_conflict",
				Raw:        []byte(`{"error":"idempotency_conflict"}`),
			})

			assert.NotZero(t, globalExitCode)
			assert.Contains(t, stdout.String(), "Error:")
		})
	}
}

func TestHandleErrorJSONValidationErrorUsesCLIExitCode(t *testing.T) {
	cmd, stdout, stderr := newHandleErrorTestCommand(t, "json")
	err := clierrors.NewValidationError("invalid input", nil)
	globalExitCode = 0

	handleError(cmd, err)

	assert.Equal(t, clierrors.GetExitCode(err), globalExitCode)
	assert.NotZero(t, globalExitCode)
	assert.JSONEq(t, `{"error":true,"message":"invalid input"}`, stdout.String())
	assert.Empty(t, stderr.String())
}
