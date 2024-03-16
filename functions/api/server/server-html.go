package server

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
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
	local        bool
	rollbarToken string
	humanDAO     *humandao.DAO
	logger       zerolog.Logger
	template     *template.Template

	index  bleve.Index
	humans []humandao.Human
	lock   sync.Mutex
}

type ServerHTMLConfig struct {
	RollbarToken string
}

func NewServerHTML(local bool, humanDAO *humandao.DAO, logger zerolog.Logger, conf ServerHTMLConfig) *ServerHTML {
	return &ServerHTML{
		local:        local,
		humanDAO:     humanDAO,
		logger:       logger,
		rollbarToken: conf.RollbarToken,
	}
}

func (s *ServerHTML) initializeIndex(ctx context.Context) error {
	defer func(now time.Time) {
		s.logger.Info().Dur("elapsed", time.Since(now)).Msg("index initialized")
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

	router.Get("/", HttpHandler(s.HandlerIndex).Serve(s.HandlerError))
	router.Get("/about", HttpHandler(s.HandlerAbout).Serve(s.HandlerError))
	router.Get("/humans", HttpHandler(s.HandlerHumans).Serve(s.HandlerError))
	router.Get("/humans/{id}", HttpHandler(s.HandlerHuman).Serve(s.HandlerError))
	router.Post("/humans/{id}", HttpHandler(s.HandlerHumanUpdate).Serve(s.HandlerError))
	router.Get("/humans/{id}/edit", HttpHandler(s.HandlerHumanEdit).Serve(s.HandlerError))
	// redirect the old search route to the new one
	router.Get("/search", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/humans", http.StatusMovedPermanently)
	})
	router.Handle("/*", s.WrapFileServer(publicFS))

	return nil
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
	var errorParam struct {
		EnableAds bool
		Error     string
		Status    int
	}
	errorParam.EnableAds = !s.local
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
	Local        bool
	EnableAds    bool
	RollbarToken string
	Admin        bool
}

type HTMLResponseHumans struct {
	Base
	Humans      []humandao.Human
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
		Base
		Musicians     []humandao.Human
		Comedians     []humandao.Human
		Actors        []humandao.Human
		Legends       []humandao.Human
		RecentlyAdded []humandao.Human
	}

	indexParams.EnableAds = !s.local
	indexParams.RollbarToken = s.rollbarToken
	// deep copy humans
	humans := append([]humandao.Human(nil), s.humans...)
	for i, human := range humans {
		humans[i].Path = "/humans/" + human.Path
	}

	if len(humans) >= 10 {
		indexParams.RecentlyAdded = humans[:10]
	}
	musicians := byName(humans, "Samica Jhangiani", "Thuy Tran", "Jonathan Park")
	actors := byName(humans, "Michelle Yeoh", "Sung Kang", "Constance Wu")
	comedians := byName(humans, "Bobby Lee", "Sheng Wang", "Ali Wong")
	legends := byName(humans, "Bruce Lee", "Anna May Wong", "Yuri Kochiyama")

	indexParams.Musicians = musicians
	indexParams.Actors = actors
	indexParams.Comedians = comedians
	indexParams.Legends = legends

	if err := s.template.ExecuteTemplate(w, "index.html", indexParams); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute index.html template")
	}

	return nil
}

func byName(humans []humandao.Human, names ...string) []humandao.Human {
	m := make(map[string]humandao.Human)
	for _, human := range humans {
		m[human.Name] = human
	}
	results := make([]humandao.Human, 0, len(names))
	for _, name := range names {
		human, ok := m[name]
		if !ok {
			continue
		}
		results = append(results, human)
	}

	return results
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
		Base: Base{
			Local:        s.local,
			EnableAds:    !s.local,
			RollbarToken: s.rollbarToken,
		},
		Count:       len(humans),
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
	Base
	Human humandao.Human
}

func (s *ServerHTML) HandlerAbout(w http.ResponseWriter, r *http.Request) error {
	if err := s.template.ExecuteTemplate(w, "about.html", nil); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute about.html template")
	}

	return nil
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
		if errors.Is(err, humandao.ErrHumanNotFound) {
			return NewNotFoundError(err)
		}
		return err
	}

	base := Base{EnableAds: !s.local, Admin: s.local, RollbarToken: s.rollbarToken, Local: s.local}
	response := HTMLResponseHuman{Human: human, Base: base}
	if err := s.template.ExecuteTemplate(w, "humans-id.html", response); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute humans-id template")
	}

	go func() {
		ctx := context.Background()
		// best attempt to update the view count
		if err := s.humanDAO.View(ctx, humandao.ViewInput{HumanID: human.ID}); err != nil {
			s.logger.Error().Err(err).Str("humanName", human.Name).Msg("unable to update human view count")
		}
	}()

	return nil
}

func (s *ServerHTML) HandlerHumanEdit(w http.ResponseWriter, r *http.Request) error {
	path := chi.URLParamFromCtx(r.Context(), "id")
	ctx := r.Context()
	path, err := url.PathUnescape(path)
	if err != nil {
		return err
	}

	human, err := s.humanDAO.Human(ctx, humandao.HumanInput{Path: path})
	if err != nil {
		if errors.Is(err, humandao.ErrHumanNotFound) {
			return NewNotFoundError(err)
		}
		return err
	}

	base := Base{EnableAds: !s.local, Admin: s.local, RollbarToken: s.rollbarToken, Local: s.local}
	response := HTMLResponseHuman{Human: human, Base: base}
	if err := s.template.ExecuteTemplate(w, "humans-id-edit.html", response); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute humans-id-edit.html template")
	}

	return nil
}

func (s *ServerHTML) HandlerHumanUpdate(w http.ResponseWriter, r *http.Request) error {
	// todo: protect this handler
	// todo: make this return just a partial template
	if err := r.ParseForm(); err != nil {
		return err
	}
	fmt.Println("form received", r.Form)
	description := strings.TrimSpace(r.Form.Get("description"))

	path := chi.URLParamFromCtx(r.Context(), "id")
	ctx := r.Context()
	path, err := url.PathUnescape(path)
	if err != nil {
		return err
	}

	human, err := s.humanDAO.Human(ctx, humandao.HumanInput{Path: path})
	if err != nil {
		if errors.Is(err, humandao.ErrHumanNotFound) {
			return NewNotFoundError(err)
		}
		return err
	}

	human.Description = description
	if err := s.humanDAO.UpdateHuman(ctx, human); err != nil {
		return err
	}

	base := Base{EnableAds: !s.local, Admin: s.local, RollbarToken: s.rollbarToken, Local: s.local}
	response := HTMLResponseHuman{Human: human, Base: base}
	if err := s.template.ExecuteTemplate(w, "humans-id.html", response); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute humans-id template")
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
