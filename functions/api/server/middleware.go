package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"firebase.google.com/go/v4/auth"
)

var ErrNoAuthorization = fmt.Errorf("no authorization token")

type Authorizer interface {
	VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error)
	SessionCookie(ctx context.Context, idToken string, expiresIn time.Duration) (string, error)
	VerifySessionCookieAndCheckRevoked(ctx context.Context, idToken string) (*auth.Token, error)
}

type NoOpAuthorizer struct{}

func (n NoOpAuthorizer) VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	return &auth.Token{
		UID:    "test-user",
		Claims: map[string]interface{}{"admin": true},
	}, nil
}

func (n NoOpAuthorizer) SessionCookie(ctx context.Context, idToken string, expiresIn time.Duration) (string, error) {
	return "", nil
}

func (n NoOpAuthorizer) VerifySessionCookieAndCheckRevoked(ctx context.Context, idToken string) (*auth.Token, error) {
	return &auth.Token{
		UID:    "test-user",
		Claims: map[string]interface{}{"admin": true},
	}, nil
}

func IsAdmin(token *auth.Token) bool {
	if token == nil {
		return false
	}
	admin, ok := token.Claims["admin"]
	if !ok {
		return false
	}
	return admin.(bool)
}

func parseBearerToken(r *http.Request) (token string, err error) {
	const bearerPrefix = "Bearer "

	tok := r.Header.Get("Authorization")
	if strings.HasPrefix(tok, bearerPrefix) {
		return tok[len(bearerPrefix):], nil
	}

	return "", ErrNoAuthorization
}
