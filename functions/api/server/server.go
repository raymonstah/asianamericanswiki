package server

import (
	"encoding/json"
	"net/http"
	"time"

	"cloud.google.com/go/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog"
	"github.com/patrickmn/go-cache"
	"github.com/rs/zerolog"

	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/raymonstah/asianamericanswiki/internal/openai"
	"github.com/raymonstah/asianamericanswiki/internal/ratelimiter"
	"github.com/raymonstah/asianamericanswiki/internal/userdao"
)

type Config struct {
	AuthClient    Authorizer
	HumansDAO     *humandao.DAO
	UsersDAO      *userdao.DAO
	Logger        zerolog.Logger
	Version       string
	OpenAIClient  *openai.Client
	StorageClient *storage.Client
	Local         bool
}

type Server struct {
	authClient    Authorizer
	router        chi.Router
	logger        zerolog.Logger
	humanCache    *cache.Cache
	rateLimiter   *ratelimiter.RateLimiter
	humanDAO      *humandao.DAO
	userDAO       *userdao.DAO
	version       string
	openAIClient  *openai.Client
	storageClient *storage.Client
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.router.ServeHTTP(writer, request)
}

func NewServer(config Config) *Server {
	r := chi.NewRouter()
	humanCache := cache.New(5*time.Minute, 10*time.Minute)
	s := &Server{
		authClient:    config.AuthClient,
		router:        r,
		logger:        config.Logger,
		humanCache:    humanCache,
		rateLimiter:   ratelimiter.New(3, time.Second),
		humanDAO:      config.HumansDAO,
		userDAO:       config.UsersDAO,
		version:       config.Version,
		openAIClient:  config.OpenAIClient,
		storageClient: config.StorageClient,
	}

	r.Use(middleware.RealIP)
	r.Use(middleware.StripSlashes)
	r.Use(cors.Handler(cors.Options{
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
	}))
	s.setupRoutes()
	htmlServer := NewServerHTML(config.Local, config.HumansDAO, config.Logger)
	r.Group(func(r chi.Router) {
		r.Use(httplog.RequestLogger(s.logger))
	})
		if err := htmlServer.Register(r); err != nil {
			panic(err)
		}

	return s
}

func (s *Server) setupRoutes() {
	s.router.Group(func(r chi.Router) {
		r.Use(httplog.RequestLogger(s.logger))
		r.Route("/api/v1/", func(r chi.Router) {
			r.Method(http.MethodGet, "/version", Handler(s.Version))

			r.Method(http.MethodGet, "/humans/{humanID}/reactions", Handler(s.ReactionsForHuman))

			r.Route("/humans", func(r chi.Router) {
				r.Method(http.MethodGet, "/", Handler(s.HumansList))
				r.With(s.AuthMiddleware).Method(http.MethodPost, "/search", Handler(s.HumansByID))
				r.Method(http.MethodGet, "/{path}", Handler(s.HumanGet))
				r.With(s.AuthMiddleware).Method(http.MethodPost, "/", Handler(s.HumanCreate))
				r.With(s.AdminMiddleware).Method(http.MethodGet, "/drafts", Handler(s.HumansDraft))
				r.With(s.AdminMiddleware).Method(http.MethodPost, "/{id}/review", Handler(s.HumansReview))
			})

			r.Route("/reactions", func(r chi.Router) {
				r.Method(http.MethodGet, "/", Handler(s.GetReactions))
				r.With(s.AuthMiddleware).Method(http.MethodPost, "/", Handler(s.PostReaction))
				r.With(s.AuthMiddleware).Method(http.MethodDelete, "/{id}", Handler(s.DeleteReaction))
			})

			r.With(s.AuthMiddleware).Method(http.MethodGet, "/user", Handler(s.User))
			r.With(s.AuthMiddleware).Method(http.MethodPost, "/humans/{humanID}/save", Handler(s.SaveHuman))
			r.With(s.AuthMiddleware).Method(http.MethodDelete, "/humans/{humanID}/save", Handler(s.UnsaveHuman))
			r.
				With(s.RateLimitMiddleware).
				With(s.OptionalAuthMiddleware).
				Method(http.MethodPost, "/humans/{humanID}/view", Handler(s.ViewHuman))
		})
	})
}

func (s *Server) Version(w http.ResponseWriter, r *http.Request) error {
	data := map[string]string{
		"version": s.version,
		"now":     time.Now().String(),
	}
	s.writeData(w, http.StatusOK, data)
	return nil
}

func (s *Server) writeData(w http.ResponseWriter, status int, data any) {
	w.WriteHeader(status)
	if status != http.StatusNoContent {
		dataResponse := struct {
			Data any `json:"data"`
		}{
			Data: data,
		}
		if err := json.NewEncoder(w).Encode(dataResponse); err != nil {
			s.logger.Err(err).Msg("error encoding json data response")
		}
	}
}
