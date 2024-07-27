package server

import (
	"context"
	"fmt"
	"testing"

	"firebase.google.com/go/v4/auth"
	"github.com/segmentio/ksuid"
	"github.com/tj/assert"
)

func createTestUser(t *testing.T, ctx context.Context, authClient *auth.Client) (userRecord *auth.UserRecord, email, password string) {
	email = fmt.Sprintf("%v@test.com", ksuid.New().String())
	password = ksuid.New().String()
	userRecord, err := authClient.CreateUser(ctx, (&auth.UserToCreate{}).Email(email).Password(password))
	assert.NoError(t, err)
	t.Cleanup(func() {
		err := authClient.DeleteUser(ctx, userRecord.UID)
		assert.NoError(t, err)
	})
	return userRecord, email, password
}

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
