package printer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthProfilesNameOneProfileInTheSingular(t *testing.T) {
	output := renderWith(t, "plain", func(h ResponseHandler) error { return h.HandleAuthProfiles([]string{"default"}) })

	assert.Equal(t, "Found 1 profile: default\n", output)
}
