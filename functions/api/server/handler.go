package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"firebase.google.com/go/v4/auth"
	"github.com/go-chi/httplog"
	"github.com/raymonstah/asianamericanswiki/internal/ratelimiter"
)

type contextKey string

var (
	tokenKey contextKey = "tokenKey"
)

type Handler func(w http.ResponseWriter, r *http.Request) error

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
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return err
		}

		if err := s.rateLimiter.Check(ip); err != nil {
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
			return NewForbiddenError(fmt.Errorf("user is not an admin"))
		}

		ctx := WithToken(r.Context(), token)
		next.ServeHTTP(w, r.WithContext(ctx))
		return nil
	})

}

func (s *Server) parseToken(r *http.Request, optional bool) (*auth.Token, error) {
	ctx := r.Context()

	tokenString, err := parseBearerToken(r, optional)
	if err != nil {
		return nil, NewUnauthorizedError(err)
	}

	if optional {
		return nil, nil
	}

	token, err := s.authClient.VerifyIDToken(ctx, tokenString)
	if err != nil {
		return nil, NewUnauthorizedError(fmt.Errorf("unable to verify id token: %w", err))
	}

	return token, nil
}

func parseBearerToken(r *http.Request, optional bool) (token string, err error) {
	tok := r.Header.Get("Authorization")
	if len(tok) > 6 && strings.ToUpper(tok[0:7]) == "BEARER " {
		return tok[7:], nil
	}
	if optional {
		return "", nil
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
