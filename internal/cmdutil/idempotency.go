package cmdutil

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/api"
	"github.com/spf13/cobra"
)

// maxIdempotencyKeyLength is the most characters the API accepts in the
// Idempotency-Key header.
const maxIdempotencyKeyLength = 255

// AddIdempotencyKeyFlag adds the --idempotency-key flag.
func AddIdempotencyKeyFlag(cmd *cobra.Command) {
	cmd.Flags().String("idempotency-key", "", "Idempotency key for safe retries (generated when omitted)")
}

// ReadIdempotencyKey returns --idempotency-key, or "" when it was not given.
// An empty or blank key, and a key that starts or ends with white space, are
// validation errors: Go's HTTP client drops the white space around a header
// value, so the API would get another key than the one given, or none.
func ReadIdempotencyKey(cmd *cobra.Command) (string, error) {
	key, err := ReadNonEmpty(cmd, "idempotency-key", "")
	if err != nil || key == nil {
		return "", err
	}
	if strings.TrimSpace(*key) != *key {
		return "", errors.NewValidationError(
			"--idempotency-key cannot start or end with a space, which the HTTP header would drop", nil)
	}
	return *key, nil
}

// IdempotencyOptions returns the request option that sends --idempotency-key,
// or no option when the flag is not given, so the SDK generates the key. A
// key ReadIdempotencyKey or CheckIdempotencyKey refuses is a validation
// error; commands call it before authenticating.
func IdempotencyOptions(cmd *cobra.Command) ([]api.RequestOption, error) {
	key, err := ReadIdempotencyKey(cmd)
	if err != nil || key == "" {
		return nil, err
	}
	if err := CheckIdempotencyKey(key, 1); err != nil {
		return nil, err
	}
	return []api.RequestOption{api.WithIdempotencyKey(key)}, nil
}

// CheckIdempotencyKey checks --idempotency-key for a command that sends
// requests requests with the keys of IdempotencyKeyFor: a key with a
// character that is not printable ASCII, which an HTTP header cannot carry
// as it is, and a key whose longest form is longer than the API accepts are
// validation errors.
func CheckIdempotencyKey(key string, requests int) error {
	if !validation.IsPrintableASCII(key) {
		return errors.NewValidationError(
			"--idempotency-key must be printable ASCII: letters, digits, punctuation and spaces, without control or non-ASCII characters", nil)
	}
	excess := utf8.RuneCountInString(IdempotencyKeyFor(key, requests-1, requests)) - maxIdempotencyKeyLength
	if excess <= 0 {
		return nil
	}
	limit := utf8.RuneCountInString(key) - excess
	if requests == 1 {
		return errors.NewValidationError(fmt.Sprintf("--idempotency-key must be at most %d characters", limit), nil)
	}
	return errors.NewValidationError(fmt.Sprintf(
		"--idempotency-key must be at most %d characters for a send of %d requests, since each request adds -N to it",
		limit, requests), nil)
}

// IdempotencyKeyFor returns the idempotency key of the request at index, from
// 0, of a command that sends requests requests: key itself for a single
// request, otherwise key followed by "-" and index.
func IdempotencyKeyFor(key string, index, requests int) string {
	if requests == 1 {
		return key
	}
	return key + "-" + strconv.Itoa(index)
}
