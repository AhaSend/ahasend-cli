package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateScope(t *testing.T) {
	tests := []struct {
		name    string
		scope   string
		wantErr bool
	}{
		// Legacy static scopes that were valid before extraction
		{name: "messages send all", scope: "messages:send:all", wantErr: false},
		{name: "messages cancel all", scope: "messages:cancel:all", wantErr: false},
		{name: "messages read all", scope: "messages:read:all", wantErr: false},
		{name: "domains read", scope: "domains:read", wantErr: false},
		{name: "domains write", scope: "domains:write", wantErr: false},
		{name: "domains delete all", scope: "domains:delete:all", wantErr: false},
		{name: "accounts read", scope: "accounts:read", wantErr: false},
		{name: "accounts write", scope: "accounts:write", wantErr: false},
		{name: "accounts billing", scope: "accounts:billing", wantErr: false},
		{name: "accounts members read", scope: "accounts:members:read", wantErr: false},
		{name: "accounts members add", scope: "accounts:members:add", wantErr: false},
		{name: "accounts members update", scope: "accounts:members:update", wantErr: false},
		{name: "accounts members remove", scope: "accounts:members:remove", wantErr: false},
		{name: "webhooks read all", scope: "webhooks:read:all", wantErr: false},
		{name: "webhooks write all", scope: "webhooks:write:all", wantErr: false},
		{name: "webhooks delete all", scope: "webhooks:delete:all", wantErr: false},
		{name: "routes read all", scope: "routes:read:all", wantErr: false},
		{name: "routes write all", scope: "routes:write:all", wantErr: false},
		{name: "routes delete all", scope: "routes:delete:all", wantErr: false},
		{name: "suppressions read", scope: "suppressions:read", wantErr: false},
		{name: "suppressions write", scope: "suppressions:write", wantErr: false},
		{name: "suppressions delete", scope: "suppressions:delete", wantErr: false},
		{name: "suppressions wipe", scope: "suppressions:wipe", wantErr: false},
		{name: "smtp-credentials read all", scope: "smtp-credentials:read:all", wantErr: false},
		{name: "smtp-credentials write all", scope: "smtp-credentials:write:all", wantErr: false},
		{name: "smtp-credentials delete all", scope: "smtp-credentials:delete:all", wantErr: false},
		{name: "statistics read all", scope: "statistics-transactional:read:all", wantErr: false},
		{name: "api-keys read", scope: "api-keys:read", wantErr: false},
		{name: "api-keys write", scope: "api-keys:write", wantErr: false},
		{name: "api-keys delete", scope: "api-keys:delete", wantErr: false},

		// New sub-account scopes
		{name: "sub-accounts read", scope: "sub-accounts:read", wantErr: false},
		{name: "sub-accounts write", scope: "sub-accounts:write", wantErr: false},
		{name: "sub-accounts delete", scope: "sub-accounts:delete", wantErr: false},
		{name: "sub-accounts suspend", scope: "sub-accounts:suspend", wantErr: false},
		{name: "sub-accounts usage", scope: "sub-accounts:usage", wantErr: false},

		// New sub-account API key scopes
		{name: "sub-account-api-keys read", scope: "sub-account-api-keys:read", wantErr: false},
		{name: "sub-account-api-keys write", scope: "sub-account-api-keys:write", wantErr: false},
		{name: "sub-account-api-keys delete", scope: "sub-account-api-keys:delete", wantErr: false},

		// Dynamic domain scopes
		{name: "messages send domain", scope: "messages:send:{example.com}", wantErr: false},
		{name: "messages cancel domain", scope: "messages:cancel:{example.com}", wantErr: false},
		{name: "messages read domain", scope: "messages:read:{example.com}", wantErr: false},
		{name: "domains delete domain", scope: "domains:delete:{example.com}", wantErr: false},
		{name: "webhooks read domain", scope: "webhooks:read:{example.com}", wantErr: false},
		{name: "webhooks write domain", scope: "webhooks:write:{example.com}", wantErr: false},
		{name: "webhooks delete domain", scope: "webhooks:delete:{example.com}", wantErr: false},
		{name: "routes read domain", scope: "routes:read:{example.com}", wantErr: false},
		{name: "smtp-credentials read domain", scope: "smtp-credentials:read:{example.com}", wantErr: false},
		{name: "statistics read domain", scope: "statistics-transactional:read:{example.com}", wantErr: false},

		// Invalid scopes still fail after extraction
		{name: "empty scope", scope: "", wantErr: true},
		{name: "unknown scope", scope: "invalid", wantErr: true},
		{name: "unknown nested scope", scope: "messages:invalid", wantErr: true},
		{name: "too generic", scope: "write", wantErr: true},
		{name: "unknown operation", scope: "domains:delete", wantErr: true},
		{name: "dynamic empty domain", scope: "messages:send:{}", wantErr: true},
		{name: "dynamic domain without dot", scope: "messages:send:{localhost}", wantErr: true},
		{name: "dynamic prefix not allowed", scope: "accounts:read:{example.com}", wantErr: true},
		{name: "sub-account unknown operation", scope: "sub-accounts:invalid", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateScope(tt.scope)
			if tt.wantErr {
				assert.Error(t, err, "scope %q should be invalid", tt.scope)
			} else {
				assert.NoError(t, err, "scope %q should be valid", tt.scope)
			}
		})
	}
}
