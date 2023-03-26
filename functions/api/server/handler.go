package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"firebase.google.com/go/v4/auth"
	"github.com/go-chi/httplog"
)

type contextKey string

var (
	tokenKey contextKey = "tokenKey"
)

type Handler func(w http.ResponseWriter, r *http.Request) error

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := h(w, r); err != nil {
		var errResponse ErrorResponse
		ok := errors.As(err, &errResponse)
		if !ok {
			errResponse.Status = http.StatusInternalServerError
			errResponse.Err = err
		}
		w.WriteHeader(errResponse.Status)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{"error": errResponse.Error()}); err != nil {
			oplog := httplog.LogEntry(r.Context())
			oplog.Err(err).Msg("unable to encode error response")
			return
		}

	}
}

func (s Server) AuthMiddleware(next http.Handler) http.Handler {
	return Handler(func(w http.ResponseWriter, r *http.Request) error {
		ctx := r.Context()

		tokenString, err := parseBearerToken(r)
		if err != nil {
			return NewUnauthorizedError(err)
		}

		token, err := s.authClient.VerifyIDToken(ctx, tokenString)
		if err != nil {
			return NewUnauthorizedError(fmt.Errorf("unable to verify id token: %w", err))
		}

		ctx = WithToken(ctx, token)
		next.ServeHTTP(w, r.WithContext(ctx))
		return nil
	})

}

func (s Server) AdminMiddleware(next http.Handler) http.Handler {
	return Handler(func(w http.ResponseWriter, r *http.Request) error {
		ctx := r.Context()

		tokenString, err := parseBearerToken(r)
		if err != nil {
			return NewUnauthorizedError(err)
		}

		token, err := s.authClient.VerifyIDToken(ctx, tokenString)
		if err != nil {
			return NewUnauthorizedError(fmt.Errorf("unable to verify id token: %w", err))
		}

		admin, ok := token.Claims["admin"]
		if !ok || !admin.(bool) {
			return NewForbiddenError(fmt.Errorf("user is not an admin"))
		}

		ctx = WithToken(ctx, token)
		next.ServeHTTP(w, r.WithContext(ctx))
		return nil
	})

}

func parseBearerToken(r *http.Request) (token string, err error) {
	tok := r.Header.Get("Authorization")
	if len(tok) > 6 && strings.ToUpper(tok[0:7]) == "BEARER " {
		return tok[7:], nil
	}
	return "", fmt.Errorf("invalid authorization token")
}

// Token pulls the auth.Token out of the context.
func Token(ctx context.Context) *auth.Token {
	token, ok := ctx.Value(tokenKey).(*auth.Token)
	if !ok {
		return nil
	}
	return token
}

// WithToken takes a token and sticks it in the context.
func WithToken(ctx context.Context, token *auth.Token) context.Context {
	return context.WithValue(ctx, tokenKey, token)
}
