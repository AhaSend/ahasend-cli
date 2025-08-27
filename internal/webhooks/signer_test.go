package webhooks

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateWebhookSecret(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "generates valid webhook secret",
		},
		{
			name: "generates unique secrets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "generates valid webhook secret" {
				secret, err := GenerateWebhookSecret()
				require.NoError(t, err)
				
				// Check format: aha-whsec-<64 chars>
				assert.True(t, strings.HasPrefix(secret, "aha-whsec-"))
				assert.Equal(t, 74, len(secret)) // "aha-whsec-" (10) + 64 chars
				
				// Check that the random part only contains valid characters
				randomPart := strings.TrimPrefix(secret, "aha-whsec-")
				for _, c := range randomPart {
					assert.True(t, isValidRandomChar(c), "Invalid character in secret: %c", c)
				}
			} else if tt.name == "generates unique secrets" {
				// Generate multiple secrets and ensure they're different
				secrets := make(map[string]bool)
				for i := 0; i < 10; i++ {
					secret, err := GenerateWebhookSecret()
					require.NoError(t, err)
					assert.False(t, secrets[secret], "Duplicate secret generated")
					secrets[secret] = true
				}
			}
		})
	}
}

func TestGenerateMsgID(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "generates valid UUID v7",
		},
		{
			name: "generates unique IDs",
		},
		{
			name: "generates time-ordered IDs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "generates valid UUID v7" {
				msgID := GenerateMsgID()
				
				// Verify it's a valid UUID
				parsed, err := uuid.Parse(msgID)
				require.NoError(t, err)
				
				// Verify it's version 7 (time-ordered)
				assert.Equal(t, uuid.Version(7), parsed.Version())
			} else if tt.name == "generates unique IDs" {
				// Generate multiple IDs and ensure they're different
				ids := make(map[string]bool)
				for i := 0; i < 10; i++ {
					id := GenerateMsgID()
					assert.False(t, ids[id], "Duplicate ID generated")
					ids[id] = true
				}
			} else if tt.name == "generates time-ordered IDs" {
				// Generate IDs with small delays and verify they're ordered
				id1 := GenerateMsgID()
				time.Sleep(10 * time.Millisecond)
				id2 := GenerateMsgID()
				
				// Parse UUIDs
				uuid1, err := uuid.Parse(id1)
				require.NoError(t, err)
				uuid2, err := uuid.Parse(id2)
				require.NoError(t, err)
				
				// For UUIDv7, earlier timestamps result in lexicographically smaller UUIDs
				assert.True(t, id1 < id2, "IDs should be time-ordered")
				assert.Equal(t, uuid.Version(7), uuid1.Version())
				assert.Equal(t, uuid.Version(7), uuid2.Version())
			}
		})
	}
}

func TestSigner_Sign(t *testing.T) {
	tests := []struct {
		name      string
		secret    string
		msgID     string
		timestamp time.Time
		payload   []byte
		wantErr   bool
	}{
		{
			name:      "signs payload correctly",
			secret:    "aha-whsec-test1234567890",
			msgID:     "msg-123",
			timestamp: time.Unix(1234567890, 0),
			payload:   []byte(`{"test": "data"}`),
			wantErr:   false,
		},
		{
			name:      "signs empty payload",
			secret:    "aha-whsec-test1234567890",
			msgID:     "msg-456",
			timestamp: time.Unix(1234567890, 0),
			payload:   []byte{},
			wantErr:   false,
		},
		{
			name:      "signs large payload",
			secret:    "aha-whsec-test1234567890",
			msgID:     "msg-789",
			timestamp: time.Unix(1234567890, 0),
			payload:   []byte(strings.Repeat("a", 10000)),
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signer := NewSigner(tt.secret)
			signature, err := signer.Sign(tt.msgID, tt.timestamp, tt.payload)
			
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			
			require.NoError(t, err)
			
			// Signature format: v1,<base64-encoded-signature>
			assert.True(t, strings.HasPrefix(signature, "v1,"))
			
			// Verify the signature is valid base64
			signaturePart := strings.TrimPrefix(signature, "v1,")
			_, err = base64.StdEncoding.DecodeString(signaturePart)
			assert.NoError(t, err, "Signature should be valid base64")
			
			// Verify deterministic: same inputs produce same signature
			signature2, err := signer.Sign(tt.msgID, tt.timestamp, tt.payload)
			require.NoError(t, err)
			assert.Equal(t, signature, signature2, "Signatures should be deterministic")
			
			// Verify different inputs produce different signatures
			if tt.name == "signs payload correctly" {
				// Different message ID
				diffSig1, err := signer.Sign("different-id", tt.timestamp, tt.payload)
				require.NoError(t, err)
				assert.NotEqual(t, signature, diffSig1, "Different message IDs should produce different signatures")
				
				// Different timestamp
				diffSig2, err := signer.Sign(tt.msgID, time.Unix(9999999999, 0), tt.payload)
				require.NoError(t, err)
				assert.NotEqual(t, signature, diffSig2, "Different timestamps should produce different signatures")
				
				// Different payload
				diffSig3, err := signer.Sign(tt.msgID, tt.timestamp, []byte(`{"different": "data"}`))
				require.NoError(t, err)
				assert.NotEqual(t, signature, diffSig3, "Different payloads should produce different signatures")
				
				// Different secret
				diffSigner := NewSigner("different-secret")
				diffSig4, err := diffSigner.Sign(tt.msgID, tt.timestamp, tt.payload)
				require.NoError(t, err)
				assert.NotEqual(t, signature, diffSig4, "Different secrets should produce different signatures")
			}
		})
	}
}

func TestNewSigner(t *testing.T) {
	tests := []struct {
		name   string
		secret string
	}{
		{
			name:   "creates signer with regular secret",
			secret: "aha-whsec-test1234567890",
		},
		{
			name:   "creates signer with empty secret",
			secret: "",
		},
		{
			name:   "creates signer with special characters",
			secret: "aha-whsec-!@#$%^&*()_+-=[]{}|;:,.<>?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signer := NewSigner(tt.secret)
			assert.NotNil(t, signer)
			assert.Equal(t, []byte(tt.secret), signer.secret)
		})
	}
}

func TestGenerateRandomString(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{
			name:   "generates string of correct length",
			length: 64,
		},
		{
			name:   "generates short string",
			length: 1,
		},
		{
			name:   "generates long string",
			length: 256,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateRandomString(tt.length)
			assert.Equal(t, tt.length, len(result))
			
			// Check all characters are valid
			for _, c := range result {
				assert.True(t, isValidRandomChar(c), "Invalid character in random string: %c", c)
			}
			
			// Check uniqueness (generate multiple and ensure they're different)
			if tt.length > 10 {
				results := make(map[string]bool)
				for i := 0; i < 10; i++ {
					r := generateRandomString(tt.length)
					assert.False(t, results[r], "Duplicate random string generated")
					results[r] = true
				}
			}
		})
	}
}

// Helper function to check if a character is valid in the random string
func isValidRandomChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}