package server

import (
	"encoding/json"
	"net/http"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog"
	"github.com/rs/zerolog"

	"github.com/raymonstah/asianamericanswiki/internal/contributor"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
)

type Config struct {
	AuthClient  *auth.Client
	HumansDAO   *humandao.DAO
	Logger      zerolog.Logger
	Version     string
	Contributor contributor.Client
}

type Server struct {
	authClient  *auth.Client
	router      chi.Router
	logger      zerolog.Logger
	humanDAO    *humandao.DAO
	version     string
	contributor contributor.Client
}

func (s Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.router.ServeHTTP(writer, request)
}

func NewServer(config Config) Server {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(httplog.RequestLogger(config.Logger))
	r.Use(middleware.StripSlashes)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
	}))
	s := Server{
		authClient:  config.AuthClient,
		router:      r,
		logger:      config.Logger,
		humanDAO:    config.HumansDAO,
		version:     config.Version,
		contributor: config.Contributor,
	}

	s.setupRoutes()
	return s
}

func (s Server) setupRoutes() {
	s.router.Method(http.MethodGet, "/version", Handler(s.Version))
	s.router.Method(http.MethodPost, "/contribute", Handler(s.Contribute))

	s.router.Method(http.MethodGet, "/humans/{humanID}/reactions", Handler(s.ReactionsForHuman))
	s.router.Route("/reactions", func(r chi.Router) {
		r.Use(s.AuthMiddleware())
		r.Method(http.MethodGet, "/", Handler(s.GetReactions))
		r.Method(http.MethodPost, "/", Handler(s.PostReaction))
		r.Method(http.MethodDelete, "/{id}", Handler(s.DeleteReaction))
	})
}

func (s Server) Version(w http.ResponseWriter, r *http.Request) error {
	data := map[string]string{
		"version": s.version,
		"now":     time.Now().String(),
	}
	s.WriteData(w, http.StatusOK, data)
	return nil
}

func (s Server) WriteData(w http.ResponseWriter, status int, data any) {
	w.WriteHeader(status)
	dataResponse := struct {
		Data any `json:"data"`
	}{
		Data: data,
	}
	if err := json.NewEncoder(w).Encode(dataResponse); err != nil {
		s.logger.Err(err).Msg("error encoding json data response")
	}
}
