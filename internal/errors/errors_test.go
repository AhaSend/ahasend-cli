package errors

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"os"
	"syscall"
	"testing"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/stretchr/testify/assert"
)

func TestGetExitCode_NetworkFailuresExitSeven(t *testing.T) {
	refused := &url.Error{Op: "Get", URL: "http://127.0.0.1:1/v2/ping",
		Err: &net.OpError{Op: "dial", Net: "tcp", Err: fmt.Errorf("connection refused")}}
	tests := []struct {
		name string
		err  error
	}{
		{"refused connection", refused},
		{"wrapped refused connection", fmt.Errorf("failed to get domain: %w", refused)},
		{"SDK network error", &api.NetworkError{Op: "server selection", Err: fmt.Errorf("no server")}},
		{"deadline", fmt.Errorf("request: %w", context.DeadlineExceeded)},
		{"connection deadline", fmt.Errorf("read reply: %w", os.ErrDeadlineExceeded)},
		{"refused dial", fmt.Errorf("connect: %w", &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED})},
		{"failed DNS lookup", fmt.Errorf("connect: %w", &net.DNSError{Err: "no such host", Name: "api.example"})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, 7, GetExitCode(tt.err))
		})
	}
}

// TestGetExitCode_LocalFailuresAreNotNetworkFailures checks that a failure to
// read stdin or write stdout, whose system error also satisfies net.Error,
// exits 1 and not 7.
func TestGetExitCode_LocalFailuresAreNotNetworkFailures(t *testing.T) {
	tests := map[string]error{
		"full disk":     &os.PathError{Op: "write", Path: "/dev/stdout", Err: syscall.ENOSPC},
		"closed pipe":   fmt.Errorf("print: %w", &os.PathError{Op: "write", Path: "/dev/stdout", Err: syscall.EPIPE}),
		"bare errno":    syscall.EIO,
		"plain failure": fmt.Errorf("something failed"),
	}
	for name, err := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, 1, GetExitCode(err))
		})
	}
}

func TestGetExitCode_APIErrorExitsByStatus(t *testing.T) {
	assert.Equal(t, 5, GetExitCode(&api.APIError{StatusCode: 404}))
}

// certificateFailure is the error of a request whose TLS handshake failed
// the certificate check with err.
func certificateFailure(err error) error {
	return &url.Error{Op: "Post", URL: "https://127.0.0.1:1/v2/messages",
		Err: &tls.CertificateVerificationError{Err: err}}
}

// bareCertificateFailure is the error of a request that failed the
// certificate check with err, not wrapped in a
// tls.CertificateVerificationError, as a TLS connection made outside the
// HTTP client, such as an SMTP one, can fail.
func bareCertificateFailure(err error) error {
	return &net.OpError{Op: "remote error", Net: "tcp", Err: err}
}

func TestGetExitCode_CertificateFailuresAreNetworkFailures(t *testing.T) {
	assert.Equal(t, 7, GetExitCode(certificateFailure(x509.UnknownAuthorityError{})))
}

func TestIsRetryable(t *testing.T) {
	refused := &url.Error{Op: "Post", URL: "http://127.0.0.1:1/v2/messages",
		Err: &net.OpError{Op: "dial", Net: "tcp", Err: fmt.Errorf("connection refused")}}
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"nil", nil, false},
		{"429", &api.APIError{StatusCode: 429}, true},
		{"500", &api.APIError{StatusCode: 500}, true},
		{"wrapped 503", fmt.Errorf("send: %w", &api.APIError{StatusCode: 503}), true},
		{"400", &api.APIError{StatusCode: 400}, false},
		{"401", &api.APIError{StatusCode: 401}, false},
		{"404", &api.APIError{StatusCode: 404}, false},
		{"409", &api.APIError{StatusCode: 409, Type: api.ErrorTypeConflict}, false},
		{"409 idempotent request in progress", &api.APIError{StatusCode: 409, Type: api.ErrorTypeIdempotencyConflict, RetryAfter: 2}, true},
		{"refused connection", refused, true},
		{"untrusted certificate", certificateFailure(x509.UnknownAuthorityError{}), false},
		{"certificate for another host", certificateFailure(x509.HostnameError{Host: "127.0.0.1"}), false},
		{"expired certificate", certificateFailure(x509.CertificateInvalidError{Reason: x509.Expired}), false},
		{"failed verification", certificateFailure(fmt.Errorf("no valid chain")), false},
		{"bare untrusted certificate", bareCertificateFailure(x509.UnknownAuthorityError{}), false},
		{"bare certificate for another host", bareCertificateFailure(x509.HostnameError{Host: "127.0.0.1"}), false},
		{"bare expired certificate", bareCertificateFailure(x509.CertificateInvalidError{Reason: x509.Expired}), false},
		{"SDK network error", &api.NetworkError{Op: "server selection", Err: fmt.Errorf("no server")}, true},
		{"deadline", context.DeadlineExceeded, true},
		{"plain error that names a retryable status", fmt.Errorf("service unavailable (503)"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.retryable, IsRetryable(tt.err))
		})
	}
}
