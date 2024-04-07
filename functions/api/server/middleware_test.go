package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/segmentio/ksuid"
	"github.com/tj/assert"
)

func TestServer_AuthMiddleware_Unauthorized(t *testing.T) {
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: api.ProjectID})
	assert.NoError(t, err)
	authClient, err := app.Auth(ctx)
	assert.NoError(t, err)

	fsClient, err := app.Firestore(ctx)
	assert.NoError(t, err)

	s := NewServer(Config{
		AuthClient: authClient,
		HumanDAO:   humandao.NewDAO(fsClient),
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer fakeToken")
	h := s.AuthMiddleware(http.HandlerFunc(nil))

	h.ServeHTTP(w, r)
	body, err := io.ReadAll(io.Reader(w.Result().Body))
	assert.NoError(t, err)

	var m map[string]interface{}
	err = json.Unmarshal(body, &m)
	assert.NoError(t, err)
	assert.Contains(t, m["error"], "unable to verify id token")
	assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
}

func TestServer_AuthMiddleware(t *testing.T) {
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: api.ProjectID})
	assert.NoError(t, err)

	fsClient, err := app.Firestore(ctx)
	assert.NoError(t, err)

	authClient, err := app.Auth(ctx)
	assert.NoError(t, err)
	s := NewServer(Config{
		AuthClient: authClient,
		HumanDAO:   humandao.NewDAO(fsClient),
	})

	email := fmt.Sprintf("%v@test.com", ksuid.New().String())
	password := ksuid.New().String()
	userRecord, err := authClient.CreateUser(ctx, (&auth.UserToCreate{}).Email(email).Password(password))
	assert.NoError(t, err)
	defer func() {
		err := authClient.DeleteUser(ctx, userRecord.UID)
		assert.NoError(t, err)
	}()

	idToken, err := signIn(email, password)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+idToken)
	h := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := Token(r.Context())
		assert.NotNil(t, token)
		assert.Equal(t, userRecord.UID, token.UID)
	}))

	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
}

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

func TestServer_AdminMiddleware(t *testing.T) {
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: api.ProjectID})
	assert.NoError(t, err)
	authClient, err := app.Auth(ctx)
	assert.NoError(t, err)

	fsClient, err := app.Firestore(ctx)
	assert.NoError(t, err)

	s := NewServer(Config{
		AuthClient: authClient,
		HumanDAO:   humandao.NewDAO(fsClient),
	})

	userRecord, email, password := createTestUser(t, ctx, authClient)
	// Give user the admin claim
	claims := map[string]any{"admin": true}
	err = authClient.SetCustomUserClaims(ctx, userRecord.UID, claims)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	h := s.AdminMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		token := Token(r.Context())
		assert.NotNil(t, token)
		assert.Equal(t, userRecord.UID, token.UID)
	}))

	idToken, err := signIn(email, password)
	assert.NoError(t, err)
	r.Header.Set("Authorization", "Bearer "+idToken)
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)

	t.Run("without-admin-claim", func(t *testing.T) {
		// Remove the admin claim
		err := authClient.SetCustomUserClaims(ctx, userRecord.UID, nil)
		assert.NoError(t, err)

		idToken, err = signIn(email, password)
		assert.NoError(t, err)
		r.Header.Set("Authorization", "Bearer "+idToken)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		assert.Equal(t, http.StatusForbidden, w.Result().StatusCode)
		bodyRaw, err := io.ReadAll(io.Reader(w.Result().Body))
		assert.NoError(t, err)

		assert.JSONEq(t, `{"error":"user is not an admin"}`, string(bodyRaw))
	})
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

func Test_ParseToken(t *testing.T) {
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: api.ProjectID})
	assert.NoError(t, err)
	authClient, err := app.Auth(ctx)
	assert.NoError(t, err)
	fsClient, err := app.Firestore(ctx)
	assert.NoError(t, err)

	s := NewServer(Config{
		AuthClient: authClient,
		HumanDAO:   humandao.NewDAO(fsClient),
	})

	tcs := map[string]struct {
		r         func() *http.Request
		optional  bool
		hasToken  bool
		expectErr error
	}{
		"valid-token": {
			r: func() *http.Request {
				_, email, password := createTestUser(t, ctx, authClient)
				idToken, err := signIn(email, password)
				assert.NoError(t, err)
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				r.Header.Set("Authorization", "Bearer "+idToken)
				return r
			},
			optional:  false,
			hasToken:  true,
			expectErr: nil,
		},
		"no-token": {
			r: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			optional:  false,
			hasToken:  false,
			expectErr: NewUnauthorizedError(ErrNoAuthorization),
		},
		"optional-no-token": {
			r: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			optional:  true,
			hasToken:  false,
			expectErr: nil,
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			req := tc.r()
			gotToken, err := s.parseToken(req, tc.optional)
			assert.Equal(t, tc.expectErr, err)
			assert.Equal(t, tc.hasToken, gotToken != nil)
		})
	}
}
