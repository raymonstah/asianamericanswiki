package server

import (
	"testing"

	"firebase.google.com/go/v4/auth"
	"github.com/tj/assert"
)

func TestIsAdmin(t *testing.T) {
	t.Parallel()
	tcs := map[string]struct {
		token   *auth.Token
		isAdmin bool
	}{
		"admin": {
			token: &auth.Token{
				Claims: map[string]interface{}{"admin": true},
			},
			isAdmin: true,
		},
		"not-admin": {
			token: &auth.Token{
				Claims: map[string]interface{}{"admin": false},
			},
			isAdmin: false,
		},
		"no-claims": {
			token: &auth.Token{
				Claims: map[string]interface{}{},
			},
			isAdmin: false,
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			got := IsAdmin(tc.token)
			assert.Equal(t, tc.isAdmin, got)
		})
	}
}