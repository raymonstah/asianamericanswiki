package server

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/rs/zerolog"
)

//go:embed public/*
var publicFS embed.FS

type ServerHTML struct {
	local    bool
	humanDAO *humandao.DAO
	logger   zerolog.Logger
	template *template.Template
}

func NewServerHTML(local bool, humanDAO *humandao.DAO, logger zerolog.Logger) *ServerHTML {
	return &ServerHTML{
		local:    local,
		humanDAO: humanDAO,
		logger:   logger,
	}
}

func (s *ServerHTML) Register(router chi.Router) error {
	templatesFS, err := fs.Sub(publicFS, "public/templates")
	if err != nil {
		return err
	}
	publicFS, err := fs.Sub(publicFS, "public/static")
	if err != nil {
		return err
	}

	htmlTemplates, err := template.New("").
		Funcs(template.FuncMap{
			"year": time.Now().Year,
		}).
		ParseFS(templatesFS, "*.html")
	if err != nil {
		return err
	}

	s.template = htmlTemplates

	router.Handle("/*", http.FileServer(http.FS(publicFS)))
	router.Get("/", HttpHandler(s.HandlerIndex).Serve())
	router.Get("/about", func(w http.ResponseWriter, r *http.Request) {
		if err := htmlTemplates.ExecuteTemplate(w, "about.html", nil); err != nil {
			s.logger.Error().Err(err).Msg("unable to execute about.html template")
		}
	})

	router.Get("/humans", HttpHandler(s.HandlerHumans).Serve())
	router.Get("/humans/{id}", HttpHandler(s.HandlerHuman).Serve())

	return nil
}

type HTMLResponseHumans struct {
	Humans    []humandao.Human
	EnableAds bool
	Count     int
}

func (s *ServerHTML) HandlerIndex(w http.ResponseWriter, r *http.Request) error {
	var (
		ctx = r.Context()
	)
	humans, err := s.humanDAO.ListHumans(ctx, humandao.ListHumansInput{
		Limit:     500,
		OrderBy:   humandao.OrderByCreatedAt,
		Direction: firestore.Desc,
	})
	if err != nil {
		return err
	}

	var indexParams struct {
		EnableAds bool
		Humans    []humandao.Human
	}

	indexParams.EnableAds = !s.local
	indexParams.Humans = humans
	for i, human := range indexParams.Humans {
		indexParams.Humans[i].Path = "/humans/" + human.Path
	}

	if err := s.template.ExecuteTemplate(w, "index.html", indexParams); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute index.html template")
	}

	return nil
}

func (s *ServerHTML) HandlerHumans(w http.ResponseWriter, r *http.Request) error {
	var (
		ctx         = r.Context()
		tags        = r.URL.Query()["tag"]
		ethnicities = r.URL.Query()["ethnicity"]
	)
	humans, err := s.humanDAO.ListHumans(ctx, humandao.ListHumansInput{
		Limit: 500,
	})
	if err != nil {
		return err
	}
	filters := []humandao.FilterOpt{}
	if len(tags) > 0 {
		filters = append(filters, humandao.ByTags(tags...))
	}
	if len(ethnicities) > 0 {
		for _, ethn := range ethnicities {
			filters = append(filters, humandao.ByEthnicity(ethn))
		}
	}
	humans = humandao.ApplyFilters(humans, filters...)
	for i, human := range humans {
		humans[i].Path = "/humans/" + human.Path
	}

	response := HTMLResponseHumans{
		Count:     len(humans),
		EnableAds: !s.local,
		Humans:    humans,
	}
	if err := s.template.ExecuteTemplate(w, "humans.html", response); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute humans.html template")
	}

	return nil
}

type HTMLResponseHuman struct {
	Human     humandao.Human
	EnableAds bool
}

func (s *ServerHTML) HandlerHuman(w http.ResponseWriter, r *http.Request) error {
	path := chi.URLParamFromCtx(r.Context(), "id")
	ctx := r.Context()
	path, err := url.PathUnescape(path)
	if err != nil {
		return err
	}

	human, err := s.humanDAO.Human(ctx, humandao.HumanInput{Path: path})
	if err != nil {
		return err
	}

	response := HTMLResponseHuman{Human: human, EnableAds: !s.local}
	if err := s.template.ExecuteTemplate(w, "humans-id.html", response); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute humans-id template")
	}

	return nil
}

type HttpHandler func(http.ResponseWriter, *http.Request) error

func (h HttpHandler) Serve() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		oplog := httplog.LogEntry(r.Context())
		if err := h(w, r); err != nil {
			oplog.Err(err).Msg("error serving request")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
