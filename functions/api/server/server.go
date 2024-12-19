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
	HumanDAO      *humandao.DAO
	UserDAO       *userdao.DAO
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
		humanDAO:      config.HumanDAO,
		userDAO:       config.UserDAO,
		version:       config.Version,
		openAIClient:  config.OpenAIClient,
		storageClient: config.StorageClient,
	}

	r.Use(middleware.RealIP)
	r.Use(middleware.CleanPath)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(cors.Handler(cors.Options{
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
	}))
	s.setupRoutes()
	htmlServer := NewServerHTML(ServerHTMLConfig{
		Local:         config.Local,
		HumanDAO:      config.HumanDAO,
		Logger:        config.Logger,
		StorageClient: config.StorageClient,
		AuthClient:    config.AuthClient,
		RollbarToken:  "e1082079233c44628d29032fc1847ca7",
		OpenaiClient:  config.OpenAIClient,
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
			r.Method(http.MethodGet, "/humans", Handler(s.Humans))
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

type Human struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Path        string          `json:"path"`
	DOB         string          `json:"dob"`
	DOD         string          `json:"dod"`
	Tags        []string        `json:"tags"`
	Ethnicity   []string        `json:"ethnicity"`
	Image       string          `json:"image"`
	Description string          `json:"description"`
	Socials     HumanSocial     `json:"socials"`
	Views       int64           `json:"views"`
	Gender      humandao.Gender `json:"gender"`
}

type HumanSocial struct {
	Instagram string `json:"instagram"`
	X         string `json:"x"`
	Website   string `json:"website"`
	IMDB      string `json:"imdb"`
}

func (s *Server) Humans(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	humansRaw, ok := s.humanCache.Get("humans")
	if ok {
		s.logger.Debug().Msg("cache hit")
		s.writeData(w, http.StatusOK, humansRaw)
		return nil
	}
	humans, err := s.humanDAO.ListHumans(ctx, humandao.ListHumansInput{
		Limit:  500,
		Offset: 0,
	})
	if err != nil {
		s.writeData(w, http.StatusInternalServerError, err)
	}
	humansResult := convert(humans)
	s.humanCache.Set("humans", humansResult, cache.DefaultExpiration)
	s.writeData(w, http.StatusOK, humans)
	return nil
}

func convert(humans []humandao.Human) []Human {
	var humansOut []Human
	for _, human := range humans {
		humanOut := Human{
			ID:          human.ID,
			Name:        human.Name,
			Path:        human.Path,
			DOB:         human.DOB,
			DOD:         human.DOD,
			Tags:        human.Tags,
			Ethnicity:   human.Ethnicity,
			Image:       human.FeaturedImage,
			Description: human.Description,
			Socials: HumanSocial{
				Instagram: human.Socials.Instagram,
				X:         human.Socials.X,
				Website:   human.Socials.Website,
				IMDB:      human.Socials.IMDB,
			},
			Views:  human.Views,
			Gender: human.Gender,
		}
		humansOut = append(humansOut, humanOut)
	}

	return humansOut
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
