package validation

import (
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/errors"
)

// validStaticScopes lists the API-key scopes that are accepted verbatim,
// based on the backend implementation. The top-level API-key commands and
// the nested sub-account API-key commands share this single set to avoid
// drift.
var validStaticScopes = map[string]bool{
	// Messages scopes
	"messages:send:all":   true,
	"messages:cancel:all": true,
	"messages:read:all":   true,

	// Domain scopes
	"domains:read":       true,
	"domains:write":      true,
	"domains:delete:all": true,

	// Account scopes
	"accounts:read":           true,
	"accounts:write":          true,
	"accounts:billing":        true,
	"accounts:members:read":   true,
	"accounts:members:add":    true,
	"accounts:members:update": true,
	"accounts:members:remove": true,

	// Webhook scopes
	"webhooks:read:all":   true,
	"webhooks:write:all":  true,
	"webhooks:delete:all": true,

	// Route scopes
	"routes:read:all":   true,
	"routes:write:all":  true,
	"routes:delete:all": true,

	// Suppression scopes
	"suppressions:read":   true,
	"suppressions:write":  true,
	"suppressions:delete": true,
	"suppressions:wipe":   true,

	// SMTP Credentials scopes
	"smtp-credentials:read:all":   true,
	"smtp-credentials:write:all":  true,
	"smtp-credentials:delete:all": true,

	// Statistics scopes
	"statistics-transactional:read:all": true,

	// API Keys scopes
	"api-keys:read":   true,
	"api-keys:write":  true,
	"api-keys:delete": true,

	// Sub-account scopes
	"sub-accounts:read":    true,
	"sub-accounts:write":   true,
	"sub-accounts:delete":  true,
	"sub-accounts:suspend": true,
	"sub-accounts:usage":   true,

	// Sub-account API key scopes
	"sub-account-api-keys:read":   true,
	"sub-account-api-keys:write":  true,
	"sub-account-api-keys:delete": true,
}

// validDynamicPrefixes lists scope prefixes that can carry a {domain}
// restriction suffix.
var validDynamicPrefixes = []string{
	// Messages with domain restriction
	"messages:send:{",
	"messages:cancel:{",
	"messages:read:{",

	// Domain deletion with domain restriction
	"domains:delete:{",

	// Webhooks with domain restriction
	"webhooks:read:{",
	"webhooks:write:{",
	"webhooks:delete:{",

	// Routes with domain restriction
	"routes:read:{",
	"routes:write:{",
	"routes:delete:{",

	// SMTP Credentials with domain restriction
	"smtp-credentials:read:{",
	"smtp-credentials:write:{",
	"smtp-credentials:delete:{",

	// Statistics with domain restriction
	"statistics-transactional:read:{",
}

// ValidateScope checks if an API-key scope is valid, accepting either a known
// static scope or a dynamic scope with a {domain} restriction.
func ValidateScope(scope string) error {
	// Check if it's a valid static scope
	if validStaticScopes[scope] {
		return nil
	}

	// Check if it's a valid dynamic scope with domain restriction
	for _, prefix := range validDynamicPrefixes {
		if strings.HasPrefix(scope, prefix) && strings.HasSuffix(scope, "}") {
			// Extract domain and validate format
			domain := strings.TrimSuffix(strings.TrimPrefix(scope, prefix), "}")
			if domain == "" {
				return errors.NewValidationError("empty domain in dynamic scope: "+scope, nil)
			}
			// Basic domain validation (could be enhanced)
			if !strings.Contains(domain, ".") {
				return errors.NewValidationError("invalid domain format in scope: "+scope, nil)
			}
			// Note: The domain must exist in your account for the API to accept it
			return nil
		}
	}

	return errors.NewValidationError("invalid scope: "+scope, nil)
}
