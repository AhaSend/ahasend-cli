package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	random "math/rand/v2"

	"github.com/google/uuid"
)

type Signer struct {
	secret []byte
}

func NewSigner(secret string) *Signer {
	return &Signer{
		secret: []byte(secret),
	}
}

func GenerateWebhookSecret() (string, error) {
	randomString := generateRandomString(64)

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

func generateRandomString(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[random.IntN(len(letters))]
	}
	return string(b)
}
