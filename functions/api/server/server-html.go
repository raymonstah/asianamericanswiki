package server

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"firebase.google.com/go/v4/auth"
	"github.com/blevesearch/bleve/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/raymonstah/asianamericanswiki/internal/imageutil"
	"github.com/raymonstah/asianamericanswiki/internal/xai"
	"github.com/rs/zerolog"
)

//go:embed public/*
var publicFS embed.FS

const webpExt = ".webp"
var maxMemoryMB = int64(10 << 20) // 10MB

type ServerHTML struct {
	local         bool
	authClient    Authorizer
	humanDAO      *humandao.DAO
	logger        zerolog.Logger
	template      *template.Template
	storageClient *storage.Client
	storageURL    string
	xaiClient     *xai.Client
	uploader      *imageutil.Uploader

	index  bleve.Index
	humans []humandao.Human
	lock   sync.Mutex

	firebaseConfig FirebaseConfig
}

type ServerHTMLConfig struct {
	Local          bool
	HumanDAO       *humandao.DAO
	Logger         zerolog.Logger
	AuthClient     Authorizer
	StorageClient  *storage.Client
	XAIClient      *xai.Client
	FirebaseConfig FirebaseConfig
}

func NewServerHTML(conf ServerHTMLConfig) *ServerHTML {
	storageURL := "https://storage.googleapis.com"
	if conf.Local {
		storageURL = "http://127.0.0.1:9199"
	}

	uploader := imageutil.NewUploader(conf.StorageClient, conf.HumanDAO, storageURL)

	return &ServerHTML{
		local:          conf.Local,
		authClient:     conf.AuthClient,
		humanDAO:       conf.HumanDAO,
		logger:         conf.Logger,
		storageClient:  conf.StorageClient,
		storageURL:     storageURL,
		xaiClient:      conf.XAIClient,
		uploader:       uploader,
		firebaseConfig: conf.FirebaseConfig,
	}
}

func (s *ServerHTML) initializeIndex(ctx context.Context) error {
	defer func(now time.Time) {
		s.logger.Info().Dur("elapsed", time.Since(now)).Msg("index initialized")
	}(time.Now())
	mapping := bleve.NewIndexMapping()
	humansMapping := bleve.NewDocumentMapping()
	mapping.AddDocumentMapping("humans", humansMapping)
	index, err := bleve.NewMemOnly(mapping)
	if err != nil {
		return err
	}

	humans, err := s.humanDAO.ListHumans(ctx, humandao.ListHumansInput{
		Limit:         500,
		IncludeDrafts: true,
	})
	if err != nil {
		return err
	}

	for _, human := range humans {
		if err := index.Index(human.ID, human); err != nil {
			return err
		}
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	s.index = index
	s.humans = humans
	return nil
}

func (s *ServerHTML) updateIndex(human humandao.Human) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.index != nil {
		if err := s.index.Index(human.ID, human); err != nil {
			return err
		}
	}

	found := false
	for i, h := range s.humans {
		if h.ID == human.ID {
			s.humans[i] = human
			found = true
			break
		}
	}
	if !found {
		s.humans = append(s.humans, human)
	}

	return nil
}

func (s *ServerHTML) deleteFromIndex(id string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.index != nil {
		if err := s.index.Delete(id); err != nil {
			return err
		}
	}

	for i, h := range s.humans {
		if h.ID == id {
			s.humans = append(s.humans[:i], s.humans[i+1:]...)
			break
		}
	}

	return nil
}

func (s *ServerHTML) Register(router chi.Router) error {
	ctx := context.Background()
	if err := s.initializeIndex(ctx); err != nil {
		return err
	}

	templatesFS, err := fs.Sub(publicFS, "public/templates")
	if err != nil {
		return err
	}
	publicStaticFS, err := fs.Sub(publicFS, "public/static")
	if err != nil {
		return err
	}

	htmlTemplates, err := template.New("").
		Funcs(template.FuncMap{
			"slicesContains": slicesContain,
			"year":           time.Now().Year,
			"imagePrompt":    xai.DefaultImagePrompt,
			"nl2br": func(text string) template.HTML {
				return template.HTML(strings.ReplaceAll(template.HTMLEscapeString(text), "\n", "<br>"))
			},
		}).
		ParseFS(templatesFS, "*.html")
	if err != nil {
		return err
	}

	s.template = htmlTemplates

	router.Get("/", HttpHandler(s.HandlerIndex).Serve(s.HandlerError))
	router.Get("/about", HttpHandler(s.HandlerAbout).Serve(s.HandlerError))
	router.Get("/random", HttpHandler(s.HandlerRandom).Serve(s.HandlerError))
	router.Get("/sitemap.xml", HttpHandler(s.HandlerSitemap).Serve(s.HandlerError))
	router.Get("/robots.txt", HttpHandler(s.HandlerRobots).Serve(s.HandlerError))
	router.Get("/humans", HttpHandler(s.HandlerHumans).Serve(s.HandlerError))
	router.Get("/search/suggest", HttpHandler(s.HandlerSearchSuggest).Serve(s.HandlerError))
	router.Get("/humans/{id}", HttpHandler(s.HandlerHuman).Serve(s.HandlerError))
	router.Post("/humans", HttpHandler(s.HandlerHumanAdd).Serve(s.HandlerError))
	router.Post("/humans/{id}", HttpHandler(s.HandlerHumanUpdate).Serve(s.HandlerError))
	router.Get("/humans/{id}/edit", HttpHandler(s.HandlerHumanEdit).Serve(s.HandlerError))
	router.Post("/humans/{id}/publish", HttpHandler(s.HandlerPublish).Serve(s.HandlerError))
	router.Delete("/humans/{id}", HttpHandler(s.HandlerHumanDelete).Serve(s.HandlerError))
	router.Get("/login", HttpHandler(s.HandlerLogin).Serve(s.HandlerError))
	router.Post("/login", HttpHandler(s.HandlerLogin).Serve(s.HandlerError))
	router.Get("/admin", HttpHandler(s.HandlerAdmin).Serve(s.HandlerError))
	router.Post("/generate", HttpHandler(s.HandlerGenerate).Serve(s.HandlerError))
	router.Get("/admin/xai", HttpHandler(s.HandlerXAIAdmin).Serve(s.HandlerError))
	router.Get("/admin/xai/human/{id}", HttpHandler(s.HandlerXAIHuman).Serve(s.HandlerError))
	router.Post("/admin/xai/generate", HttpHandler(s.HandlerXAIGenerate).Serve(s.HandlerError))
	router.Post("/admin/xai/upload", HttpHandler(s.HandlerXAIUpload).Serve(s.HandlerError))
	router.Handle("/xai-generations/*", http.StripPrefix("/xai-generations/", http.FileServer(http.Dir("tmp/xai_generations"))))
	// redirect the old search route to the new one
	router.Get("/search", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/humans", http.StatusMovedPermanently)
	})
	router.Handle("/*", s.WrapFileServer(publicStaticFS))

	return nil
}

func (s *ServerHTML) parseOptionalToken(r *http.Request) *auth.Token {
	ctx := r.Context()
	cookie, err := r.Cookie("session")
	if err != nil {
		return nil
	}

	tokenString := cookie.Value
	token, err := s.authClient.VerifySessionCookieAndCheckRevoked(ctx, tokenString)
	if err != nil {
		return nil
	}

	return token
}

func (s *ServerHTML) parseToken(r *http.Request) (*auth.Token, error) {
	ctx := r.Context()
	cookie, err := r.Cookie("session")
	if err != nil {
		return nil, err
	}

	tokenString := cookie.Value

	token, err := s.authClient.VerifySessionCookieAndCheckRevoked(ctx, tokenString)
	if err != nil {
		return nil, NewUnauthorizedError(fmt.Errorf("unable to verify id token: %w", err))
	}

	return token, nil
}

func (s *ServerHTML) WrapFileServer(fileSystem fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fileSystem))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fs.Stat(fileSystem, r.URL.Path[1:])
		if err != nil {
			if os.IsNotExist(err) {
				_ = s.HandlerError(w, r, NewNotFoundError(fmt.Errorf("page does not exist")))
				return
			}
			// fallthrough
		}

		w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%v", 24*time.Hour.Seconds()))
		fileServer.ServeHTTP(w, r)
	})
}

func (s *ServerHTML) HandlerError(w http.ResponseWriter, r *http.Request, e ErrorResponse) error {
	if e.Status == http.StatusInternalServerError {
		s.logger.Error().Err(e.Err).Msg("handler internal error")
	}
	var errorParam struct {
		Base
		Error  string
		Status int
	}
	base := getBase(s, false)
	errorParam.Base = base

	errorParam.Status = e.Status
	errorParam.Error = e.Err.Error()

	w.WriteHeader(errorParam.Status)
	if err := s.template.ExecuteTemplate(w, "error.html", errorParam); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute error.html template")
		http.Error(w, "Something went terribly wrong!", http.StatusInternalServerError)
		return err
	}

	return nil
}

type Base struct {
	Local bool
	Admin bool
}

func (s *ServerHTML) HandlerIndex(w http.ResponseWriter, r *http.Request) error {
	var indexParams struct {
		Base
		Musicians     []humandao.Human
		Comedians     []humandao.Human
		Actors        []humandao.Human
		Legends       []humandao.Human
		RecentlyAdded []humandao.Human
		Count         int
	}
	base := getBase(s, false)
	indexParams.Base = base

	s.lock.Lock()
	humans := make([]humandao.Human, 0, len(s.humans))
	for _, human := range s.humans {
		if !human.Draft {
			humans = append(humans, human)
		}
	}
	s.lock.Unlock()

	for i, human := range humans {
		humans[i].Path = "/humans/" + human.Path
	}

	if len(humans) >= 10 {
		indexParams.RecentlyAdded = humans[:10]
	}
	musicians := byName(humans, "Russell Llantino", "Thuy Tran", "Jonathan Park")
	actors := byName(humans, "Michelle Yeoh", "Sung Kang", "Constance Wu")
	comedians := byName(humans, "Bobby Lee", "Sheng Wang", "Ali Wong")
	legends := byName(humans, "Bruce Lee", "Anna May Wong", "Yuri Kochiyama")

	indexParams.Musicians = musicians
	indexParams.Actors = actors
	indexParams.Comedians = comedians
	indexParams.Legends = legends
	indexParams.Count = len(humans)

	if err := s.template.ExecuteTemplate(w, "index.html", indexParams); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute index.html template")
	}

	return nil
}

func (s *ServerHTML) HandlerAbout(w http.ResponseWriter, r *http.Request) error {
	if err := s.template.ExecuteTemplate(w, "about.html", nil); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute about.html template")
	}

	return nil
}

type HttpHandler func(http.ResponseWriter, *http.Request) error

func (h HttpHandler) Serve(errorHandler func(w http.ResponseWriter, r *http.Request, e ErrorResponse) error) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		oplog := httplog.LogEntry(r.Context())
		if err := h(w, r); err != nil {
			var errResponse ErrorResponse
			ok := errors.As(err, &errResponse)
			if !ok {
				errResponse.Status = http.StatusInternalServerError
				errResponse.Err = err
			}
			oplog.Err(err).Int("status", errResponse.Status).Msg("error serving request")
			_ = errorHandler(w, r, errResponse)
			return
		}
	}
}

func getBase(s *ServerHTML, admin bool) Base {
	base := Base{
		Admin: admin,
		Local: s.local,
	}
	return base
}

func slicesContain(haystack []string, needle string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}
