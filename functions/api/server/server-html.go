package server

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog"
	"github.com/raymonstah/asianamericanswiki/internal/ethnicity"
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

	index  bleve.Index
	humans []humandao.Human
	lock   sync.Mutex
}

func NewServerHTML(local bool, humanDAO *humandao.DAO, logger zerolog.Logger) *ServerHTML {
	return &ServerHTML{
		local:    local,
		humanDAO: humanDAO,
		logger:   logger,
	}
}

func (s *ServerHTML) initializeIndex(ctx context.Context) error {
	defer func(now time.Time) {
		s.logger.Info().Dur("elapsed", time.Since(now)).Msg("index initialized")
		fmt.Println("index initialized")
	}(time.Now())
	mapping := bleve.NewIndexMapping()
	index, err := bleve.NewMemOnly(mapping)
	if err != nil {
		return err
	}

	humans, err := s.humanDAO.ListHumans(ctx, humandao.ListHumansInput{
		Limit: 500,
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

func (s *ServerHTML) Register(router chi.Router) error {
	ctx := context.Background()
	if err := s.initializeIndex(ctx); err != nil {
		return err
	}

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
	// redirect the old search route to the new one
	router.Get("/search", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/humans", http.StatusMovedPermanently)
	})

	return nil
}

type HTMLResponseHumans struct {
	Humans      []humandao.Human
	EnableAds   bool
	Count       int
	Ethnicities []ethnicity.Ethnicity
	Tags        []string
}

type Ethnicity struct {
	ID    string
	Emoji string
}

func (s *ServerHTML) HandlerIndex(w http.ResponseWriter, r *http.Request) error {
	var indexParams struct {
		EnableAds bool
		Humans    []humandao.Human
	}

	indexParams.EnableAds = !s.local
	// deep copy humans
	humans := append([]humandao.Human(nil), s.humans...)
	for i, human := range humans {
		humans[i].Path = "/humans/" + human.Path
	}
	indexParams.Humans = humans

	if err := s.template.ExecuteTemplate(w, "index.html", indexParams); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute index.html template")
	}

	return nil
}

func (s *ServerHTML) HandlerHumans(w http.ResponseWriter, r *http.Request) error {
	var (
		tags        = r.URL.Query()["tag"]
		ethnicities = r.URL.Query()["ethnicity"]
		gender      = r.URL.Query().Get("gender")
		dobBefore   = r.URL.Query().Get("dobBefore")
		dobAfter    = r.URL.Query().Get("dobAfter")
		search      = r.URL.Query().Get("search")
	)
	allTags := getTags(s.humans)
	filters := []humandao.FilterOpt{}
	// deep copy humans
	humans := append([]humandao.Human(nil), s.humans...)

	if search != "" {
		query := bleve.NewMatchQuery(search)
		query.SetFuzziness(1)
		result, err := s.index.Search(bleve.NewSearchRequest(query))
		if err != nil {
			return err
		}
		hitIDs := make([]string, 0, len(result.Hits))
		for _, hit := range result.Hits {
			hitIDs = append(hitIDs, hit.ID)
		}

		filters = append(filters, humandao.ByIDs(hitIDs...))
	}

	if len(tags) > 0 {
		filters = append(filters, humandao.ByTags(tags...))
	}
	if len(ethnicities) > 0 {
		for _, ethn := range ethnicities {
			filters = append(filters, humandao.ByEthnicity(ethn))
		}
	}
	if gender != "" {
		filters = append(filters, humandao.ByGender(humandao.Gender(gender)))
	}
	if dobBefore != "" {
		age, err := time.Parse("2006-01-02", dobBefore)
		if err != nil {
			return NewBadRequestError(err)
		}
		filters = append(filters, humandao.ByAgeOlderThan(age))
	}
	if dobAfter != "" {
		age, err := time.Parse("2006-01-02", dobAfter)
		if err != nil {
			return NewBadRequestError(err)
		}
		filters = append(filters, humandao.ByAgeYoungerThan(age))
	}

	humans = humandao.ApplyFilters(humans, filters...)
	for i, human := range humans {
		humans[i].Path = "/humans/" + human.Path
	}

	response := HTMLResponseHumans{
		Count:       len(humans),
		EnableAds:   !s.local,
		Humans:      humans,
		Ethnicities: ethnicity.All,
		Tags:        allTags,
	}
	if err := s.template.ExecuteTemplate(w, "humans.html", response); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute humans.html template")
	}

	return nil
}

func getTags(humans []humandao.Human) []string {
	uniqueTags := make(map[string]struct{}, 64)
	for _, human := range humans {
		for _, tag := range human.Tags {
			uniqueTags[tag] = struct{}{}
		}
	}
	tags := make([]string, 0, len(uniqueTags))
	for tag := range uniqueTags {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
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
