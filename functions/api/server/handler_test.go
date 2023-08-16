package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/segmentio/ksuid"
	"github.com/tj/assert"

	"github.com/raymonstah/asianamericanswiki/functions/api"
)

func TestMain(m *testing.M) {
	if err := os.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "localhost:8081"); err != nil {
		log.Fatal("failed to set FIREBASE_AUTH_EMULATOR_HOST environment variable", err)
	}

	if err := os.Setenv("FIRESTORE_EMULATOR_HOST", "localhost:8080"); err != nil {
		log.Fatal("failed to set FIRESTORE_EMULATOR_HOST environment variable", err)
	}
	m.Run()
}

func TestServer_AuthMiddleware_Unauthorized(t *testing.T) {
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: api.ProjectID})
	assert.NoError(t, err)
	authClient, err := app.Auth(ctx)
	assert.NoError(t, err)

	s := NewServer(Config{
		AuthClient: authClient,
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer fakeToken")
	h := s.AuthMiddleware(http.HandlerFunc(nil))

	h.ServeHTTP(w, r)
	body, err := ioutil.ReadAll(w.Result().Body)
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
	authClient, err := app.Auth(ctx)
	assert.NoError(t, err)
	s := NewServer(Config{
		AuthClient: authClient,
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

func TestServer_AdminMiddleware(t *testing.T) {
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: api.ProjectID})
	assert.NoError(t, err)
	authClient, err := app.Auth(ctx)
	assert.NoError(t, err)
	s := NewServer(Config{
		AuthClient: authClient,
	})

	email := fmt.Sprintf("%v@test.com", ksuid.New().String())
	password := ksuid.New().String()
	userRecord, err := authClient.CreateUser(ctx, (&auth.UserToCreate{}).Email(email).Password(password))
	assert.NoError(t, err)

	// Give user the admin claim
	claims := map[string]any{"admin": true}
	err = authClient.SetCustomUserClaims(ctx, userRecord.UID, claims)
	assert.NoError(t, err)

	defer func() {
		err := authClient.DeleteUser(ctx, userRecord.UID)
		assert.NoError(t, err)
	}()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	h := s.AdminMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		bodyRaw, err := ioutil.ReadAll(w.Result().Body)
		assert.NoError(t, err)

		assert.JSONEq(t, `{"error":"user is not an admin"}`, string(bodyRaw))
	})
}

func signIn(email, password string) (string, error) {
	signInURL := "http://localhost:8081/identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=fake-api-key"
	body := fmt.Sprintf(`{"email": %q, "password": %q}`, email, password)
	r, err := http.NewRequest(http.MethodPost, signInURL, bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")

	if err != nil {
		return "", fmt.Errorf("unable to create sign in request: %w", err)
	}

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return "", fmt.Errorf("unable to make request: %w", err)
	}

	var results struct {
		IDToken string `json:"idToken"`
		Email   string `json:"email"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return "", fmt.Errorf("unable to decode response: %w", err)
	}

	return results.IDToken, nil
}
