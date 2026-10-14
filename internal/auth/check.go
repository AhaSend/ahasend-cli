package auth

import (
	stderrors "errors"
	"fmt"
	"net/http"

	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/responses"
)

// KeyCheck is the result of a check of an API key that passed.
type KeyCheck struct {
	// Account is the key's account, or nil when the key may not read it.
	Account *responses.Account
	// Unreadable is the API's refusal (HTTP 403) to show the account to a
	// key that belongs to it but lacks the scope to read it, or nil.
	Unreadable error
}

// invalidKeyMessage is the API's message for a key it does not know or that
// belongs to another account than the one in the path.
const invalidKeyMessage = "invalid API key"

// CheckKey checks that the API key of c belongs to the account ID of c by
// reading the account. The API answers an account route with 401 when the
// key belongs to another account; GET /v2/ping does not check the account,
// so it cannot tell.
//
// A 403 from the account route is either the key's IP allow-list, which the
// API checks before the account, or the key's scope, which it checks after.
// Ping comes after the IP check and before the scope check, so CheckKey
// pings to tell them apart: a 403 from ping means the key may not be used
// from this IP, and fails the check with the API's 403; a ping that
// succeeds means the key belongs to the account but may not read it, which
// passes the check.
//
// A 401 is an auth error that names the account ID; any other failure is
// returned as it is.
func CheckKey(c client.AhaSendClient) (KeyCheck, error) {
	account, err := c.GetAccount()
	if err == nil {
		return KeyCheck{Account: account}, nil
	}
	var apiErr *api.APIError
	if !stderrors.As(err, &apiErr) {
		return KeyCheck{}, err
	}
	switch apiErr.StatusCode {
	case http.StatusForbidden:
		_, pingErr := c.Ping()
		if pingErr == nil {
			return KeyCheck{Unreadable: err}, nil
		}
		var pingAPIErr *api.APIError
		if stderrors.As(pingErr, &pingAPIErr) && pingAPIErr.StatusCode == http.StatusForbidden {
			return KeyCheck{}, errors.NewAPIError("the API refuses the API key from this IP address", pingErr)
		}
		return KeyCheck{}, pingErr
	case http.StatusUnauthorized:
		if apiErr.Message == invalidKeyMessage {
			return KeyCheck{}, errors.NewAuthError(
				fmt.Sprintf("the API key is not valid for account %s; check both", c.GetAccountID()), err)
		}
		return KeyCheck{}, errors.NewAuthError(
			fmt.Sprintf("the API refused the API key for account %s", c.GetAccountID()), err)
	}
	return KeyCheck{}, err
}

// IsKeyRefused reports whether err, from CheckKey, is the API refusing the
// key: a 401, or a 403 for the key's IP address.
func IsKeyRefused(err error) bool {
	var apiErr *api.APIError
	return stderrors.As(err, &apiErr) &&
		(apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden)
}
