package server

import (
	"context"
	"embed"
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
	"github.com/raymonstah/asianamericanswiki/internal/openai"
	"github.com/rs/zerolog"
)

//go:embed public/*
var publicFS embed.FS

type ServerHTML struct {
	local         bool
	rollbarToken  string
	authClient    Authorizer
	humanDAO      *humandao.DAO
	logger        zerolog.Logger
	template      *template.Template
	storageClient *storage.Client
	storageURL    string
	openaiClient  *openai.Client

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
	OpenaiClient  *openai.Client
	RollbarToken  string
}

func NewServerHTML(conf ServerHTMLConfig) *ServerHTML {
	storageURL := "https://storage.googleapis.com"
	if conf.Local {
		storageURL = "http://localhost:9199"
	}

	return &ServerHTML{
		local:         conf.Local,
		authClient:    conf.AuthClient,
		humanDAO:      conf.HumanDAO,
		logger:        conf.Logger,
		rollbarToken:  conf.RollbarToken,
		storageClient: conf.StorageClient,
		storageURL:    storageURL,
		openaiClient:  conf.OpenaiClient,
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
	router.Get("/login", HttpHandler(s.HandlerLogin).Serve(s.HandlerError))
	router.Post("/login", HttpHandler(s.HandlerLogin).Serve(s.HandlerError))
	router.Get("/admin", HttpHandler(s.HandlerAdmin).Serve(s.HandlerError))
	router.Post("/generate", HttpHandler(s.HandlerGenerate).Serve(s.HandlerError))
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
	Local           bool
	EnableAds       bool
	EnableAnalytics bool
	RollbarToken    string
	Admin           bool
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
	humans := make([]humandao.Human, 0, len(s.humans))
	for _, human := range s.humans {
		if !human.Draft {
			humans = append(humans, human)
		}
	}
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

	path, err := url.PathUnescape(path)
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

	response := HTMLResponseHuman{Human: human, Base: getBase(s, admin)}
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
		path  = chi.URLParamFromCtx(r.Context(), "id")
		ctx   = r.Context()
	)

	if !admin {
		return NewForbiddenError(fmt.Errorf("you are not an admin"))
	}

	path, err := url.PathUnescape(path)
	if err != nil {
		return err
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
	var rawImage []byte
	var imageExtension string
	file, header, err := r.FormFile("featured_image")
	if err != http.ErrMissingFile {
		if err != nil {
			return NewBadRequestError(fmt.Errorf("invalid image: %w", err))
		}

		raw, err := io.ReadAll(file)
		if err != nil {
			return NewBadRequestError(fmt.Errorf("invalid image: %w", err))
		}
		rawImage = raw
		imageExtension = filepath.Ext(header.Filename)
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

	objectID := fmt.Sprintf("%v%v", human.ID, imageExtension)
	if len(rawImage) > 0 {
		obj := s.storageClient.Bucket(api.ImagesStorageBucket).Object(objectID)
		writer := obj.NewWriter(ctx)
		if _, err := writer.Write(rawImage); err != nil {
			return err
		}

		if err := writer.Close(); err != nil {
			return err
		}
		human.FeaturedImage = fmt.Sprintf("%v/%v/%v", s.storageURL, api.ImagesStorageBucket, objectID)
		if err := s.humanDAO.UpdateHuman(ctx, human); err != nil {
			return err
		}
	}

	_ = s.initializeIndex(ctx)
	http.Redirect(w, r, fmt.Sprintf("/humans/%s", human.Path), http.StatusSeeOther)

	return nil
}

func (s *ServerHTML) HandlerHumanEdit(w http.ResponseWriter, r *http.Request) error {
	var (
		path = chi.URLParamFromCtx(r.Context(), "id")
		ctx  = r.Context()
	)
	path, err := url.PathUnescape(path)
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

	response := HTMLResponseHuman{
		Human: human,
		Base:  getBase(s, admin),
		HumanFormFields: HumanFormFields{
			Ethnicities: ethnicity.All,
			Tags:        getTags(s.humans),
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
		description    = strings.TrimSpace(r.Form.Get("description"))
		x              = strings.TrimSpace(r.Form.Get("x"))
		instagram      = strings.TrimSpace(r.Form.Get("instagram"))
		website        = strings.TrimSpace(r.Form.Get("website"))
		imdb           = strings.TrimSpace(r.Form.Get("imdb"))
		rawImage       []byte
		imageExtension string
		tags           = r.Form["tags"]
		tagsOther      = r.Form.Get("tags-other")
	)
	tags = append(tags, strings.Split(tagsOther, ",")...)

	file, header, err := r.FormFile("featured_image")
	if err != http.ErrMissingFile {
		if err != nil {
			return NewBadRequestError(fmt.Errorf("invalid image: %w", err))
		}

		raw, err := io.ReadAll(file)
		if err != nil {
			return NewBadRequestError(fmt.Errorf("invalid image: %w", err))
		}
		rawImage = raw
		imageExtension = filepath.Ext(header.Filename)
	}

	humanPath := chi.URLParamFromCtx(r.Context(), "id")
	humanPath, err = url.PathUnescape(humanPath)
	if err != nil {
		return err
	}

	human, err := s.humanDAO.Human(ctx, humandao.HumanInput{Path: humanPath})
	if err != nil {
		if errors.Is(err, humandao.ErrHumanNotFound) {
			return NewNotFoundError(err)
		}
		return err
	}

	human.Description = description
	human.Socials.X = x
	human.Socials.Instagram = instagram
	human.Socials.Website = website
	human.Socials.IMDB = imdb
	human.Tags = tags

	objectID := fmt.Sprintf("%v%v", human.ID, imageExtension)
	if len(rawImage) > 0 {
		obj := s.storageClient.Bucket(api.ImagesStorageBucket).Object(objectID)
		writer := obj.NewWriter(ctx)
		if _, err := writer.Write(rawImage); err != nil {
			return err
		}

		if err := writer.Close(); err != nil {
			return err
		}
		human.FeaturedImage = fmt.Sprintf("%v/%v/%v", s.storageURL, api.ImagesStorageBucket, objectID)
	}
	if err := s.humanDAO.UpdateHuman(ctx, human); err != nil {
		return err
	}

	_ = s.initializeIndex(ctx)

	response := HTMLResponseHuman{Human: human, Base: getBase(s, admin)}
	if err := s.template.ExecuteTemplate(w, "humans-id.html", response); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute humans-id template")
	}

	s.logger.Info().Str("id", human.ID).Str("name", human.Name).Msg("successfully updated human")
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
		return NewUnauthorizedError(err)
	}

	admin := IsAdmin(token)
	if !admin {
		return NewForbiddenError(fmt.Errorf("user is not an admin"))
	}

	var drafts []humandao.Human
	for _, human := range s.humans {
		if human.Draft {
			drafts = append(drafts, human)
		}
	}

	response := HTMLResponseAdmin{
		Base:      getBase(s, false),
		AdminName: token.Claims["name"].(string),
		HumanFormFields: HumanFormFields{
			Ethnicities: ethnicity.All,
			Tags:        getTags(s.humans),
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

	addHumanRequest, err := s.openaiClient.FromText(ctx, openai.FromTextInput{Data: source})
	if err != nil {
		return NewInternalServerError(err)
	}
	logger.Info().Str("source", source).Any("addHumanRequest", addHumanRequest).Msg("generated response from openai")

	human := humandao.Human{
		Name:        addHumanRequest.Name,
		Gender:      humandao.Gender(addHumanRequest.Gender),
		Ethnicity:   addHumanRequest.Ethnicity,
		DOB:         addHumanRequest.DOB,
		DOD:         addHumanRequest.DOD,
		Description: addHumanRequest.Description,
	}

	response := HTMLResponseAdmin{
		HumanFormFields: HumanFormFields{
			Source:      source,
			Ethnicities: ethnicity.All,
			Tags:        getTags(s.humans),
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
		EnableAds:       false,
		EnableAnalytics: !s.local,
		Admin:           admin,
		RollbarToken:    s.rollbarToken,
		Local:           s.local,
	}
	return base
}

func slicesContain(haystack []string, needle string) bool {
	return slices.Contains(haystack, needle)
}
