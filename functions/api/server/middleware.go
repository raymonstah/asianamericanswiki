package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"firebase.google.com/go/v4/auth"
	"github.com/raymonstah/asianamericanswiki/internal/ratelimiter"
)

var ErrNoAuthorization = fmt.Errorf("no authorization token")

type Authorizer interface {
	VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error)
}

type NoOpAuthorizer struct{}

func (n NoOpAuthorizer) VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	return &auth.Token{
		UID:    "test-user",
		Claims: map[string]interface{}{"admin": true},
	}, nil
}

func (s *Server) OptionalAuthMiddleware(next http.Handler) http.Handler {
	return Handler(func(w http.ResponseWriter, r *http.Request) error {
		token, err := s.parseToken(r, true)
		if err != nil {
			return err
		}

		ctx := WithToken(r.Context(), token)
		next.ServeHTTP(w, r.WithContext(ctx))
		return nil
	})

}

func (s *Server) AuthMiddleware(next http.Handler) http.Handler {
	return Handler(func(w http.ResponseWriter, r *http.Request) error {
		token, err := s.parseToken(r, false)
		if err != nil {
			return err
		}

		ctx := WithToken(r.Context(), token)
		next.ServeHTTP(w, r.WithContext(ctx))
		return nil
	})

}

func (s *Server) RateLimitMiddleware(next http.Handler) http.Handler {
	return Handler(func(w http.ResponseWriter, r *http.Request) error {
		if err := s.rateLimiter.Check(r.RemoteAddr); err != nil {
			if errors.Is(err, ratelimiter.ErrRateLimitExceeded) {
				return NewTooManyRequestsError(err)
			}
			return NewInternalServerError(err)
		}

		next.ServeHTTP(w, r)
		return nil
	})
}

func (s *Server) AdminMiddleware(next http.Handler) http.Handler {
	return Handler(func(w http.ResponseWriter, r *http.Request) error {
		token, err := s.parseToken(r, false)
		if err != nil {
			return err
		}

		admin, ok := token.Claims["admin"]
		if !ok || !admin.(bool) {
			s.logger.Warn().Str("uid", token.UID).Msg("user is not an admin")
			return NewForbiddenError(fmt.Errorf("user is not an admin"))
		}

		ctx := WithToken(r.Context(), token)
		next.ServeHTTP(w, r.WithContext(ctx))
		return nil
	})
}

func IsAdmin(token *auth.Token) bool {
	admin, ok := token.Claims["admin"]
	if !ok {
		return false
	}
	return admin.(bool)
}

func (s *Server) parseToken(r *http.Request, optional bool) (*auth.Token, error) {
	ctx := r.Context()

	tokenString, err := parseBearerToken(r)
	if err != nil {
		if optional && err == ErrNoAuthorization {
			return nil, nil
		}
		return nil, NewUnauthorizedError(err)
	}

	token, err := s.authClient.VerifyIDToken(ctx, tokenString)
	if err != nil {
		return nil, NewUnauthorizedError(fmt.Errorf("unable to verify id token: %w", err))
	}

	return token, nil
}

func parseBearerToken(r *http.Request) (token string, err error) {
	const bearerPrefix = "Bearer "

	tok := r.Header.Get("Authorization")
	if strings.HasPrefix(tok, bearerPrefix) {
		return tok[len(bearerPrefix):], nil
	}

	return "", ErrNoAuthorization
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
