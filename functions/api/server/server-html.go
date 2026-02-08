package server

import (
	"context"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"firebase.google.com/go/v4/auth"
	"github.com/blevesearch/bleve/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/ethnicity"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/raymonstah/asianamericanswiki/internal/imageutil"
	"github.com/raymonstah/asianamericanswiki/internal/xai"
	"github.com/rs/zerolog"
)

//go:embed public/*
var publicFS embed.FS

const webpExt = ".webp"

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
}

type ServerHTMLConfig struct {
	Local         bool
	HumanDAO      *humandao.DAO
	Logger        zerolog.Logger
	AuthClient    Authorizer
	StorageClient *storage.Client
	XAIClient     *xai.Client
}

func NewServerHTML(conf ServerHTMLConfig) *ServerHTML {
	storageURL := "https://storage.googleapis.com"
	if conf.Local {
		storageURL = "http://127.0.0.1:9199"
	}

	uploader := imageutil.NewUploader(conf.StorageClient, conf.HumanDAO, storageURL)

	return &ServerHTML{
		local:         conf.Local,
		authClient:    conf.AuthClient,
		humanDAO:      conf.HumanDAO,
		logger:        conf.Logger,
		storageClient: conf.StorageClient,
		storageURL:    storageURL,
		xaiClient:     conf.XAIClient,
		uploader:      uploader,
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

	// fields, err := index.Fields()
	// if err != nil {
	// return err
	// }
	// for _, field := range fields {
	// fmt.Printf("field: %v\n", field)
	// }

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
			"slicesContains": slicesContain,
			"year":           time.Now().Year,
			"imagePrompt":    xai.DefaultImagePrompt,
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
	router.Handle("/*", s.WrapFileServer(publicFS))

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

type HTMLResponseHumans struct {
	Base
	Humans      []humandao.Human
	Count       int
	Ethnicities []ethnicity.Ethnicity
	Tags        []string
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

	humans := make([]humandao.Human, 0, len(s.humans))
	for _, human := range s.humans {
		if !human.Draft {
			humans = append(humans, human)
		}
	}

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
	humans := make([]humandao.Human, 0, len(s.humans))
	for _, human := range s.humans {
		if !human.Draft {
			humans = append(humans, human)
		}
	}
	if search != "" {
		nameQuery := bleve.NewPrefixQuery(search)
		nameQuery.SetField("Name")
		nameQuery.SetBoost(5.0)
		query := bleve.NewMatchQuery(search)
		query.SetFuzziness(1)

		queryAll := bleve.NewDisjunctionQuery(query, nameQuery)
		searchReq := bleve.NewSearchRequest(queryAll)
		result, err := s.index.Search(searchReq)
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
		Base:        getBase(s, false),
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
	Human           humandao.Human
	HumanFormFields HumanFormFields
	Similar         []humandao.Human
}

func (s *ServerHTML) HandlerAbout(w http.ResponseWriter, r *http.Request) error {
	if err := s.template.ExecuteTemplate(w, "about.html", nil); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute about.html template")
	}

	return nil
}

func (s *ServerHTML) HandlerHuman(w http.ResponseWriter, r *http.Request) error {
	var (
		token = s.parseOptionalToken(r)
		admin = IsAdmin(token)
		path  = chi.URLParamFromCtx(r.Context(), "id")
	)

	var err error
	path, err = url.PathUnescape(path)
	if err != nil {
		return err
	}

	var human humandao.Human
	for _, h := range s.humans {
		if h.Path == path || h.ID == path {
			human = h
			break
		}
	}
	if human.ID == "" {
		return NewNotFoundError(fmt.Errorf("%w: %v", humandao.ErrHumanNotFound, path))
	}

	var similar []humandao.Human
	for _, humanID := range human.Similar {
		for _, h := range s.humans {
			if h.ID == humanID {
				similar = append(similar, h)
			}
		}
	}

	response := HTMLResponseHuman{Human: human, Similar: similar, Base: getBase(s, admin)}
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

func (s *ServerHTML) HandlerHumanAdd(w http.ResponseWriter, r *http.Request) error {
	var (
		token = s.parseOptionalToken(r)
		admin = IsAdmin(token)
		ctx   = r.Context()
	)

	if !admin {
		return NewForbiddenError(fmt.Errorf("you are not an admin"))
	}

	if err := r.ParseMultipartForm(maxMemoryMB); err != nil {
		return err
	}

	var (
		name        = strings.TrimSpace(r.Form.Get("name"))
		gender      = strings.TrimSpace(r.Form.Get("gender"))
		description = strings.TrimSpace(r.Form.Get("description"))
		dob         = strings.TrimSpace(r.Form.Get("dob"))
		dod         = strings.TrimSpace(r.Form.Get("dod"))
		ethnicity   = r.Form["ethnicity"]
		tags        = r.Form["tags"]
		imdb        = strings.TrimSpace(r.Form.Get("imdb"))
		x           = strings.TrimSpace(r.Form.Get("x"))
		website     = strings.TrimSpace(r.Form.Get("website"))
		instagram   = strings.TrimSpace(r.Form.Get("instagram"))
	)
	var rawFeaturedImage []byte
	var rawThumbnail []byte

	featuredImageFile, header, err := r.FormFile("featured_image")
	if err != http.ErrMissingFile {
		if err != nil {
			return NewBadRequestError(fmt.Errorf("invalid image: %w", err))
		}

		raw, err := io.ReadAll(featuredImageFile)
		if err != nil {
			return NewBadRequestError(fmt.Errorf("invalid image: %w", err))
		}
		rawFeaturedImage = raw
		imageExtension := filepath.Ext(header.Filename)
		if imageExtension != webpExt {
			return NewBadRequestError(fmt.Errorf("featured image should be in webp format"))
		}
	}

	thumbnailFile, header, err := r.FormFile("thumbnail")
	if err != http.ErrMissingFile {
		if err != nil {
			return NewBadRequestError(fmt.Errorf("invalid image: %w", err))
		}

		raw, err := io.ReadAll(thumbnailFile)
		if err != nil {
			return NewBadRequestError(fmt.Errorf("invalid image: %w", err))
		}
		rawThumbnail = raw
		imageExtension := filepath.Ext(header.Filename)
		if imageExtension != webpExt {
			return NewBadRequestError(fmt.Errorf("thumbnail image should be in webp format"))
		}
	}

	human, err := s.humanDAO.AddHuman(ctx, humandao.AddHumanInput{
		Name:        name,
		Gender:      humandao.Gender(gender),
		DOB:         dob,
		DOD:         dod,
		Ethnicity:   ethnicity,
		Description: description,
		Website:     website,
		Twitter:     x,
		IMDB:        imdb,
		Instagram:   instagram,
		Tags:        tags,
		CreatedBy:   token.UID,
		Draft:       true,
	})
	if err != nil {
		if errors.Is(err, humandao.ErrInvalidGender) {
			return NewBadRequestError(err)
		}
		if errors.Is(err, humandao.ErrHumanAlreadyExists) {
			return NewBadRequestError(err)
		}
		return NewInternalServerError(err)
	}

	if len(rawThumbnail) > 0 || len(rawFeaturedImage) > 0 {
		if len(rawFeaturedImage) > 0 {
			if _, err := s.uploader.UploadHumanImages(ctx, human, rawFeaturedImage); err != nil {
				return err
			}
		} else if len(rawThumbnail) > 0 {
			// This case is unlikely given the form, but let's handle it by using thumbnail as featured
			if _, err := s.uploader.UploadHumanImages(ctx, human, rawThumbnail); err != nil {
				return err
			}
		}
	}

	_ = s.initializeIndex(ctx)
	http.Redirect(w, r, fmt.Sprintf("/humans/%s", human.Path), http.StatusSeeOther)

	return nil
}

func (s *ServerHTML) HandlerHumanEdit(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	path, err := url.PathUnescape(chi.URLParamFromCtx(r.Context(), "id"))
	if err != nil {
		return err
	}
	token, err := s.parseToken(r)
	if err != nil {
		w.Header().Add("HX-Redirect", "/login")
		return nil
	}

	admin := IsAdmin(token)
	if !admin {
		return NewForbiddenError(fmt.Errorf("you are not an admin"))
	}

	human, err := s.humanDAO.Human(ctx, humandao.HumanInput{Path: path})
	if err != nil {
		if errors.Is(err, humandao.ErrHumanNotFound) {
			return NewNotFoundError(err)
		}
		return err
	}

	humans, err := s.humanDAO.ListHumans(ctx, humandao.ListHumansInput{
		Limit:         1000,
		IncludeDrafts: true,
	})
	if err != nil {
		return NewInternalServerError(fmt.Errorf("unable to list humans: %w", err))
	}

	response := HTMLResponseHuman{
		Human: human,
		Base:  getBase(s, admin),
		HumanFormFields: HumanFormFields{
			Ethnicities: ethnicity.All,
			Tags:        getTags(humans),
		},
	}
	if err := s.template.ExecuteTemplate(w, "humans-id-edit.html", response); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute humans-id-edit.html template")
	}

	return nil
}

var maxMemoryMB = int64(10 << 20) // 10MB

func (s *ServerHTML) HandlerHumanUpdate(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	// todo: make this return just a partial template
	token, err := s.parseToken(r)
	if err != nil {
		return NewUnauthorizedError(err)
	}

	admin := IsAdmin(token)
	if !admin {
		return NewForbiddenError(fmt.Errorf("user is not an admin"))
	}

	if err := r.ParseMultipartForm(maxMemoryMB); err != nil {
		return err
	}

	var (
		description = strings.TrimSpace(r.Form.Get("description"))
		x           = strings.TrimSpace(r.Form.Get("x"))
		instagram   = strings.TrimSpace(r.Form.Get("instagram"))
		website     = strings.TrimSpace(r.Form.Get("website"))
		imdb        = strings.TrimSpace(r.Form.Get("imdb"))
		tags        = r.Form["tags"]
		tagsOther   = r.Form.Get("tags-other")
		dob         = strings.TrimSpace(r.Form.Get("dob"))
		name        = strings.TrimSpace(r.Form.Get("name"))
		ethnicity   = r.Form["ethnicity"]
		gender      = strings.TrimSpace(r.Form.Get("gender"))
	)
	if tagsOther != "" {
		tags = append(tags, strings.Split(tagsOther, ",")...)
	}

	var rawFeaturedImage []byte
	var rawThumbnail []byte

	featuredImageFile, header, err := r.FormFile("featured_image")
	if err != http.ErrMissingFile {
		if err != nil {
			return NewBadRequestError(fmt.Errorf("invalid image: %w", err))
		}

		raw, err := io.ReadAll(featuredImageFile)
		if err != nil {
			return NewBadRequestError(fmt.Errorf("invalid image: %w", err))
		}
		rawFeaturedImage = raw
		imageExtension := filepath.Ext(header.Filename)
		if imageExtension != webpExt {
			return NewBadRequestError(fmt.Errorf("featured image should be in webp format"))
		}
	}

	thumbnailFile, header, err := r.FormFile("thumbnail")
	if err != http.ErrMissingFile {
		if err != nil {
			return NewBadRequestError(fmt.Errorf("invalid image: %w", err))
		}

		raw, err := io.ReadAll(thumbnailFile)
		if err != nil {
			return NewBadRequestError(fmt.Errorf("invalid image: %w", err))
		}
		rawThumbnail = raw
		imageExtension := filepath.Ext(header.Filename)
		if imageExtension != webpExt {
			return NewBadRequestError(fmt.Errorf("thumbnail image should be in webp format"))
		}
	}

	humanPathOrID := chi.URLParamFromCtx(r.Context(), "id")
	humanPathOrID, err = url.PathUnescape(humanPathOrID)
	if err != nil {
		return err
	}

	var human humandao.Human
	// Try looking up by ID first, then Path
	human, err = s.humanDAO.Human(ctx, humandao.HumanInput{HumanID: humanPathOrID})
	if err != nil {
		human, err = s.humanDAO.Human(ctx, humandao.HumanInput{Path: humanPathOrID})
		if err != nil {
			if errors.Is(err, humandao.ErrHumanNotFound) {
				return NewNotFoundError(err)
			}
			return err
		}
	}

	human.Description = description
	human.Socials.X = x
	human.Socials.Instagram = instagram
	human.Socials.Website = website
	human.Socials.IMDB = imdb
	human.Tags = tags
	human.DOB = dob
	human.Ethnicity = ethnicity
	if gender != "" {
		human.Gender = humandao.Gender(gender)
	}
	if name != "" {
		human.Name = name
	}

	if len(rawFeaturedImage) > 0 {
		if _, err := s.uploader.UploadHumanImages(ctx, human, rawFeaturedImage); err != nil {
			return err
		}
	} else if len(rawThumbnail) > 0 {
		if _, err := s.uploader.UploadHumanImages(ctx, human, rawThumbnail); err != nil {
			return err
		}
	} else {
		if err := s.humanDAO.UpdateHuman(ctx, human); err != nil {
			return err
		}
	}

	_ = s.initializeIndex(ctx)

	s.logger.Info().Str("id", human.ID).Str("name", human.Name).Msg("successfully updated human")
	http.Redirect(w, r, fmt.Sprintf("/humans/%s", human.Path), http.StatusSeeOther)
	return nil
}

func (s *ServerHTML) HandlerHumanDelete(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	token, err := s.parseToken(r)
	if err != nil {
		return NewUnauthorizedError(err)
	}

	admin := IsAdmin(token)
	if !admin {
		return NewForbiddenError(fmt.Errorf("user is not an admin"))
	}

	humanPathOrID := chi.URLParamFromCtx(r.Context(), "id")
	humanPathOrID, err = url.PathUnescape(humanPathOrID)
	if err != nil {
		return err
	}

	var human humandao.Human
	// Try looking up by ID first, then Path
	human, err = s.humanDAO.Human(ctx, humandao.HumanInput{HumanID: humanPathOrID})
	if err != nil {
		human, err = s.humanDAO.Human(ctx, humandao.HumanInput{Path: humanPathOrID})
		if err != nil {
			if errors.Is(err, humandao.ErrHumanNotFound) {
				return NewNotFoundError(err)
			}
			return err
		}
	}

	if err := s.humanDAO.Delete(ctx, humandao.DeleteInput{HumanID: human.ID}); err != nil {
		return err
	}

	_ = s.initializeIndex(ctx)

	s.logger.Info().Str("id", human.ID).Str("name", human.Name).Msg("successfully deleted human")

	// If it's an HTMX request, we might want to redirect or just return empty
	if r.Header.Get("HX-Request") != "" {
		w.Header().Add("HX-Redirect", "/admin")
		return nil
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
	return nil
}

type HTMLResponseLogin struct {
	Base
}

func (s *ServerHTML) HandlerLogin(w http.ResponseWriter, r *http.Request) error {
	if r.Method == http.MethodPost {
		idToken, err := parseBearerToken(r)
		if err != nil {
			return err
		}
		// Set session expiration to 5 days.
		expiresIn := time.Hour * 24 * 5

		// Create the session cookie. This will also verify the ID token in the process.
		// The session cookie will have the same claims as the ID token.
		// To only allow session cookie setting on recent sign-in, auth_time in ID token
		// can be checked to ensure user was recently signed in before creating a session cookie.
		cookie, err := s.authClient.SessionCookie(r.Context(), idToken, expiresIn)
		if err != nil {
			return NewUnauthorizedError(fmt.Errorf("unable to create session token: %w", err))
		}

		// Set cookie policy for session cookie.
		http.SetCookie(w, &http.Cookie{
			Name:     "session",
			Value:    cookie,
			MaxAge:   int(expiresIn.Seconds()),
			HttpOnly: true,
			Secure:   true,
		})

		// Get the original path the user was trying to access.
		referer := r.Header.Get("Referer")
		fmt.Println("referer", referer)
		if referer == "" {
			referer = "/admin"
		}

		w.Header().Add("HX-Redirect", referer)
		http.Redirect(w, r, referer, http.StatusFound)
		return nil
	}

	response := HTMLResponseLogin{Base: getBase(s, false)}
	if err := s.template.ExecuteTemplate(w, "login.html", response); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute login template")
	}

	return nil
}

type HTMLResponseAdmin struct {
	Base
	AdminName       string
	Drafts          []humandao.Human
	HumanFormFields HumanFormFields
	Human           humandao.Human
}

// HumanFormFields holds helper data to populate the form to add a new human.
type HumanFormFields struct {
	Source      string
	Ethnicities []ethnicity.Ethnicity
	Tags        []string
}

func (s *ServerHTML) HandlerAdmin(w http.ResponseWriter, r *http.Request) error {
	token, err := s.parseToken(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return nil
	}

	admin := IsAdmin(token)
	if !admin {
		return NewForbiddenError(fmt.Errorf("user is not an admin"))
	}

	humans, err := s.humanDAO.ListHumans(r.Context(), humandao.ListHumansInput{
		Limit:         1000,
		IncludeDrafts: true,
	})
	if err != nil {
		return NewInternalServerError(fmt.Errorf("unable to list humans: %w", err))
	}

	var drafts []humandao.Human
	for _, human := range humans {
		if human.Draft {
			drafts = append(drafts, human)
		}
	}

	response := HTMLResponseAdmin{
		Base:      getBase(s, false),
		AdminName: token.Claims["name"].(string),
		HumanFormFields: HumanFormFields{
			Ethnicities: ethnicity.All,
			Tags:        getTags(humans),
		},
		Drafts: drafts,
	}
	if err := s.template.ExecuteTemplate(w, "admin.html", response); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute admin template")
	}

	return nil
}

// HandlerGenerate takes in the form, and populates it based on the data in the 'source' field.
func (s *ServerHTML) HandlerGenerate(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := s.logger
	token, err := s.parseToken(r)
	if err != nil {
		return NewUnauthorizedError(err)
	}

	admin := IsAdmin(token)
	if !admin {
		return NewForbiddenError(fmt.Errorf("user is not an admin"))
	}

	if err := r.ParseForm(); err != nil {
		return NewBadRequestError(fmt.Errorf("invalid form received: %w", err))
	}

	source := r.FormValue("source")

	addHumanRequest, err := s.xaiClient.FromText(ctx, xai.FromTextInput{Data: source})
	if err != nil {
		return NewInternalServerError(err)
	}
	logger.Info().Str("source", source).Any("addHumanRequest", addHumanRequest).Msg("generated response from xAI")

	human := humandao.Human{
		Name:        addHumanRequest.Name,
		Gender:      humandao.Gender(addHumanRequest.Gender),
		Ethnicity:   addHumanRequest.Ethnicity,
		DOB:         addHumanRequest.DOB,
		DOD:         addHumanRequest.DOD,
		Description: addHumanRequest.Description,
	}

	humans, err := s.humanDAO.ListHumans(ctx, humandao.ListHumansInput{
		Limit:         1000,
		IncludeDrafts: true,
	})
	if err != nil {
		return NewInternalServerError(fmt.Errorf("unable to list humans: %w", err))
	}

	response := HTMLResponseAdmin{
		HumanFormFields: HumanFormFields{
			Source:      source,
			Ethnicities: ethnicity.All,
			Tags:        getTags(humans),
		},
		Human: human,
	}
	if err := s.template.ExecuteTemplate(w, "new-human-form.html", response); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute admin template")
	}

	return nil
}

func (s *ServerHTML) HandlerPublish(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	path := chi.URLParamFromCtx(r.Context(), "id")
	token, err := s.parseToken(r)
	if err != nil {
		return NewUnauthorizedError(err)
	}

	admin := IsAdmin(token)
	if !admin {
		return NewForbiddenError(fmt.Errorf("user is not an admin"))
	}

	var human humandao.Human
	for _, h := range s.humans {
		if h.Path == path {
			human = h
			break
		}
	}
	if human.ID == "" {
		return NewNotFoundError(humandao.ErrHumanNotFound)
	}

	if err := s.humanDAO.Publish(ctx, humandao.PublishInput{HumanID: human.ID, UserID: token.UID}); err != nil {
		return err
	}

	_ = s.initializeIndex(ctx)

	url := fmt.Sprintf("/humans/%s", human.Path)
	w.Header().Add("HX-Redirect", url)

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
	return slices.Contains(haystack, needle)
}

type HTMLResponseXAIAdmin struct {
	Base
	Humans []humandao.Human
}

func (s *ServerHTML) HandlerXAIAdmin(w http.ResponseWriter, r *http.Request) error {
	token, err := s.parseToken(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return nil
	}

	admin := IsAdmin(token)
	if !admin {
		return NewForbiddenError(fmt.Errorf("user is not an admin"))
	}

	humans, err := s.humanDAO.ListHumans(r.Context(), humandao.ListHumansInput{
		Limit:         1000,
		IncludeDrafts: true,
	})
	if err != nil {
		return NewInternalServerError(fmt.Errorf("unable to list humans: %w", err))
	}

	response := HTMLResponseXAIAdmin{
		Base:   getBase(s, admin),
		Humans: humans,
	}
	if err := s.template.ExecuteTemplate(w, "xai-admin.html", response); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute xai-admin template")
	}

	return nil
}

type HTMLResponseXAIHuman struct {
	Base
	Human          humandao.Human
	ExistingImages []string
}

func (s *ServerHTML) HandlerXAIHuman(w http.ResponseWriter, r *http.Request) error {
	token, err := s.parseToken(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return nil
	}

	admin := IsAdmin(token)
	if !admin {
		return NewForbiddenError(fmt.Errorf("user is not an admin"))
	}

	id := chi.URLParam(r, "id")
	var human humandao.Human
	for _, h := range s.humans {
		if h.ID == id || h.Path == id {
			human = h
			break
		}
	}
	if human.ID == "" {
		return NewNotFoundError(fmt.Errorf("human not found"))
	}

	// Scan for existing local images
	var existingImages []string
	localDir := filepath.Join("tmp", "xai_generations", human.ID)
	files, err := os.ReadDir(localDir)
	if err == nil {
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".webp") {
				existingImages = append(existingImages, "/xai-generations/"+human.ID+"/"+file.Name())
			}
		}
	}
	// Sort to show newest first if they have timestamps in names
	sort.Slice(existingImages, func(i, j int) bool {
		return existingImages[i] > existingImages[j]
	})

	response := HTMLResponseXAIHuman{
		Base:           getBase(s, admin),
		Human:          human,
		ExistingImages: existingImages,
	}
	if err := s.template.ExecuteTemplate(w, "xai-human.html", response); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute xai-human template")
	}

	return nil
}

func (s *ServerHTML) HandlerXAIGenerate(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	token, err := s.parseToken(r)
	if err != nil {
		return NewUnauthorizedError(err)
	}

	admin := IsAdmin(token)
	if !admin {
		return NewForbiddenError(fmt.Errorf("user is not an admin"))
	}

	if err := r.ParseForm(); err != nil {
		return NewBadRequestError(err)
	}

	humanID := r.FormValue("human_id")
	prompt := r.FormValue("prompt")
	numImagesStr := r.FormValue("num_images")
	numImages := 1
	if _, err := fmt.Sscanf(numImagesStr, "%d", &numImages); err != nil {
		s.logger.Warn().Err(err).Str("num_images", numImagesStr).Msg("invalid num_images value, defaulting to 1")
	}

	// Fetch human to get source images
	var human humandao.Human
	for _, h := range s.humans {
		if h.ID == humanID {
			human = h
			break
		}
	}

	baseImage := human.Images.Featured
	if baseImage != "" && (strings.Contains(baseImage, "127.0.0.1") || strings.HasPrefix(baseImage, "http://")) {
		s.logger.Info().Str("url", baseImage).Msg("local source image detected, converting to base64")
		// Parse the emulator URL to get the object path
		// URL format: http://127.0.0.1:9199/asianamericanswiki-images/<humanID>/original.webp
		prefix := fmt.Sprintf("%s/%s/", s.storageURL, api.ImagesStorageBucket)
		objectPath := strings.TrimPrefix(baseImage, prefix)

		obj := s.storageClient.Bucket(api.ImagesStorageBucket).Object(objectPath)
		reader, err := obj.NewReader(ctx)
		if err != nil {
			s.logger.Error().Err(err).Str("path", objectPath).Msg("failed to create reader for local storage object")
		} else {
			defer func() {
				_ = reader.Close()
			}()
			data, err := io.ReadAll(reader)
			if err != nil {
				s.logger.Error().Err(err).Msg("failed to read local storage object")
			} else {
				base64Data := base64.StdEncoding.EncodeToString(data)
				mimeType := http.DetectContentType(data)
				baseImage = fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)
				s.logger.Info().Msg("successfully converted local image to base64 for xAI")
			}
		}
	}

	imageURLs, err := s.xaiClient.GenerateImage(ctx, xai.GenerateImageInput{
		Prompt: prompt,
		N:      numImages,
		Image:  baseImage,
	})
	if err != nil {
		if strings.Contains(err.Error(), "(status 429)") {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`<div class="col-span-full p-4 bg-amber-50 border border-amber-200 rounded-lg text-amber-800">
                <p class="font-bold">xAI is currently overloaded</p>
                <p class="text-sm">The model is experiencing high demand. Please wait a few minutes and try again.</p>
            </div>`))
			return nil
		}
		return NewInternalServerError(err)
	}

	// Save images locally
	localDir := filepath.Join("tmp", "xai_generations", humanID)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return NewInternalServerError(err)
	}

	var localPaths []string
	for i, url := range imageURLs {
		resp, err := http.Get(url)
		if err != nil {
			continue
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		filename := fmt.Sprintf("%d_%d.webp", time.Now().Unix(), i)
		localPath := filepath.Join(localDir, filename)
		out, err := os.Create(localPath)
		if err != nil {
			continue
		}
		defer func() {
			_ = out.Close()
		}()
		_, _ = io.Copy(out, resp.Body)
		localPaths = append(localPaths, "/xai-generations/"+humanID+"/"+filename)
	}

	var data = struct {
		Images  []string
		HumanID string
	}{
		Images:  localPaths,
		HumanID: humanID,
	}

	if err := s.template.ExecuteTemplate(w, "xai-images-partial.html", data); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute xai-images-partial template")
	}

	return nil
}

func (s *ServerHTML) HandlerXAIUpload(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	token, err := s.parseToken(r)
	if err != nil {
		return NewUnauthorizedError(err)
	}

	admin := IsAdmin(token)
	if !admin {
		return NewForbiddenError(fmt.Errorf("user is not an admin"))
	}

	if err := r.ParseForm(); err != nil {
		return NewBadRequestError(err)
	}

	humanID := r.FormValue("human_id")
	imagePath := r.FormValue("image_path") // e.g. /xai-generations/human_id/filename.webp

	// Convert local path back to filesystem path
	fsPath := filepath.Join("tmp", "xai_generations", strings.TrimPrefix(imagePath, "/xai-generations/"))

	raw, err := os.ReadFile(fsPath)
	if err != nil {
		return NewInternalServerError(err)
	}

	// Update human record
	human, err := s.humanDAO.Human(ctx, humandao.HumanInput{HumanID: humanID})
	if err != nil {
		return NewInternalServerError(err)
	}

	human.AIGenerated = true
	if _, err := s.uploader.UploadHumanImages(ctx, human, raw); err != nil {
		return NewInternalServerError(err)
	}

	_ = s.initializeIndex(ctx)

	s.logger.Info().Str("id", human.ID).Str("name", human.Name).Msg("successfully updated human with AI image")
	w.Header().Add("HX-Redirect", fmt.Sprintf("/humans/%s", human.Path))
	return nil
}
