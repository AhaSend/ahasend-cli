package webhooks

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"math/big"
	"time"

	"github.com/google/uuid"
)

// randomSource is where secrets get their randomness: the operating system's
// cryptographic random number generator. Tests replace it.
var randomSource io.Reader = rand.Reader

type Signer struct {
	secret []byte
}

func NewSigner(secret string) *Signer {
	return &Signer{
		secret: []byte(secret),
	}
}

// GenerateWebhookSecret returns a new secret for signing webhooks:
// "aha-whsec-" and 64 random letters and digits.
func GenerateWebhookSecret() (string, error) {
	randomString, err := generateRandomString(64)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("aha-whsec-%s", randomString), nil
}

func (s *Signer) Sign(msgID string, timestamp time.Time, payload []byte) (string, error) {
	timestampStr := fmt.Sprintf("%d", timestamp.Unix())
	toSign := fmt.Sprintf("%s.%s.%s", msgID, timestampStr, string(payload))

	h := hmac.New(sha256.New, s.secret)
	h.Write([]byte(toSign))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return fmt.Sprintf("v1,%s", signature), nil
}

func GenerateMsgID() string {
	// Generate UUIDv7 (time-ordered UUID)
	id := uuid.Must(uuid.NewV7())
	return id.String()
}

// generateRandomString returns n letters and digits drawn uniformly from
// randomSource.
func generateRandomString(n int) (string, error) {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	count := big.NewInt(int64(len(letters)))
	b := make([]byte, n)
	for i := range b {
		index, err := rand.Int(randomSource, count)
		if err != nil {
			return "", fmt.Errorf("failed to read random data: %w", err)
		}
		b[i] = letters[index.Int64()]
	}
	return string(b), nil
}
