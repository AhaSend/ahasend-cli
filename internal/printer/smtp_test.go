package printer

import (
	"bytes"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSMTPCredential(password string) *responses.SMTPCredential {
	return &responses.SMTPCredential{
		Object:    "smtp_credential",
		ID:        uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Name:      "Test Credential",
		Username:  "smtp-user",
		Password:  password,
		Scope:     "global",
	}
}

// TestSMTPConnectionSettingsListSupportedPorts checks that both credential
// creation and retrieval output advertise AhaSend's STARTTLS ports and never
// port 465, which AhaSend does not support.
func TestSMTPConnectionSettingsListSupportedPorts(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		t.Run(format+"/create", func(t *testing.T) {
			var buf bytes.Buffer
			handler := GetResponseHandler(format, false, &buf)

			err := handler.HandleCreateSMTP(newTestSMTPCredential("secret-password"), CreateConfig{SuccessMessage: "created"})
			require.NoError(t, err)

			out := buf.String()
			assertSupportedSMTPPorts(t, out)
			assert.Contains(t, out, "secret-password", "password is shown once on creation")
		})

		t.Run(format+"/get", func(t *testing.T) {
			var buf bytes.Buffer
			handler := GetResponseHandler(format, false, &buf)

			err := handler.HandleSingleSMTP(newTestSMTPCredential(""), SingleConfig{SuccessMessage: "found"})
			require.NoError(t, err)

			out := buf.String()
			assertSupportedSMTPPorts(t, out)
			assert.Contains(t, out, "[Use the password provided during creation]")
		})
	}
}

func assertSupportedSMTPPorts(t *testing.T, out string) {
	t.Helper()
	assert.Contains(t, out, "587")
	assert.Contains(t, out, "recommended")
	assert.Contains(t, out, "25")
	assert.Contains(t, out, "2525")
	assert.Contains(t, out, "STARTTLS")
	assert.NotContains(t, out, "465")
	assert.NotContains(t, out, "SSL/TLS")
}
