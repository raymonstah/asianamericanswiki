package server

import (
	context "context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/patrickmn/go-cache"
	"github.com/rs/zerolog"

	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/raymonstah/asianamericanswiki/internal/ratelimiter"
	"github.com/raymonstah/asianamericanswiki/internal/userdao"
	"github.com/raymonstah/asianamericanswiki/internal/xai"
)

type FirebaseConfig struct {
	APIKey            string
	AuthDomain        string
	ProjectID         string
	StorageBucket     string
	MessagingSenderId string
	AppID             string
	MeasurementID     string
}

type Config struct {
	AuthClient     Authorizer
	HumanDAO       *humandao.DAO
	UserDAO        *userdao.DAO
	Logger         zerolog.Logger
	Version        string
	XAIClient      *xai.Client
	StorageClient  *storage.Client
	Local          bool
	FirebaseConfig FirebaseConfig
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
	xaiClient     *xai.Client
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
		xaiClient:     config.XAIClient,
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
		Local:          config.Local,
		HumanDAO:       config.HumanDAO,
		Logger:         config.Logger,
		StorageClient:  config.StorageClient,
		AuthClient:     config.AuthClient,
		XAIClient:      config.XAIClient,
		FirebaseConfig: config.FirebaseConfig,
	})
	if err := htmlServer.Register(r); err != nil {
		panic(err)
	}

	return s
}

func (s *Server) setupRoutes() {
	humanServer := &HumanServer{
		logger:     s.logger,
		humanCache: s.humanCache,
		humanDAO:   s.humanDAO,
		version:    s.version,
	}

	gwmux := runtime.NewServeMux()
	ctx := context.Background()
	if err := RegisterHumanServiceHandlerServer(ctx, gwmux, humanServer); err != nil {
		panic(err)
	}

	s.router.Group(func(r chi.Router) {
		r.Use(httplog.RequestLogger(s.logger))
		r.HandleFunc("/api/*", func(w http.ResponseWriter, r *http.Request) {
			r.URL.Path = strings.ReplaceAll(r.URL.Path, "/api", "")
			gwmux.ServeHTTP(w, r)
		})
	})
}

type HumanServer struct {
	UnimplementedHumanServiceServer // embed by value

	version    string
	logger     zerolog.Logger
	humanCache *cache.Cache
	humanDAO   *humandao.DAO
}

func (s *HumanServer) Version(ctx context.Context, req *VersionRequest) (*VersionResponse, error) {
	return &VersionResponse{
		Version:       s.version,
		Now:           time.Now().String(),
		unknownFields: []byte{},
		sizeCache:     0,
	}, nil
}

func (s *HumanServer) Humans(ctx context.Context, req *HumansRequest) (*HumansResponse, error) {
	humansRaw, ok := s.humanCache.Get("humans")
	if ok {
		s.logger.Debug().Msg("cache hit")
		humans := convert(humansRaw.([]humandao.Human))
		return &HumansResponse{
			Humans: humans,
		}, nil
	}
	humans, err := s.humanDAO.ListHumans(ctx, humandao.ListHumansInput{
		Limit:  500,
		Offset: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to list humans: %w", err)
	}

	s.humanCache.Set("humans", humans, cache.DefaultExpiration)
	protoHumans := convert(humans)

	return &HumansResponse{
		Humans: protoHumans,
	}, nil
}

func convert(humans []humandao.Human) []*Human {
	var humansOut []*Human
	for _, human := range humans {
		humanOut := &Human{
			Id:          human.ID,
			Name:        human.Name,
			Path:        human.Path,
			Dob:         human.DOB,
			Dod:         human.DOD,
			Tags:        human.Tags,
			Ethnicity:   human.Ethnicity,
			Image:       human.FeaturedImage,
			Description: human.Description,
			Socials: &HumanSocial{
				Instagram: human.Socials.Instagram,
				X:         human.Socials.X,
				Website:   human.Socials.Website,
				Imdb:      human.Socials.IMDB,
			},
			Gender: Gender(Gender_value[strings.ToUpper(string(human.Gender))]),
		}
		humansOut = append(humansOut, humanOut)
	}

	return humansOut
}
