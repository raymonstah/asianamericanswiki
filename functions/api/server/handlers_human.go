package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/raymonstah/asianamericanswiki/internal/ethnicity"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/blevesearch/bleve/v2"
)

type HTMLResponseHumans struct {
	Base
	Humans      []humandao.Human
	Count       int
	Ethnicities []ethnicity.Ethnicity
	Tags        []string
}

func (s *ServerHTML) HandlerHumans(w http.ResponseWriter, r *http.Request) error {
	var (
		tags        = r.URL.Query()["tag"]
		ethnicitiesList = r.URL.Query()["ethnicity"]
		gender      = r.URL.Query().Get("gender")
		dobBefore   = r.URL.Query().Get("dobBefore")
		dobAfter    = r.URL.Query().Get("dobAfter")
		search      = r.URL.Query().Get("search")
	)
	
	s.lock.Lock()
	allHumans := make([]humandao.Human, len(s.humans))
	copy(allHumans, s.humans)
	s.lock.Unlock()

	allTags := getTags(allHumans)
	filters := []humandao.FilterOpt{}
	
	humans := make([]humandao.Human, 0, len(allHumans))
	for _, human := range allHumans {
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
		
		s.lock.Lock()
		result, err := s.index.Search(searchReq)
		s.lock.Unlock()
		
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
	if len(ethnicitiesList) > 0 {
		for _, ethn := range ethnicitiesList {
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

func (s *ServerHTML) HandlerSearchSuggest(w http.ResponseWriter, r *http.Request) error {
	search := r.URL.Query().Get("search")
	if search == "" {
		return nil
	}

	nameQuery := bleve.NewPrefixQuery(search)
	nameQuery.SetField("Name")
	nameQuery.SetBoost(5.0)
	query := bleve.NewMatchQuery(search)
	query.SetFuzziness(1)

	queryAll := bleve.NewDisjunctionQuery(query, nameQuery)
	searchReq := bleve.NewSearchRequest(queryAll)
	searchReq.Size = 5

	s.lock.Lock()
	result, err := s.index.Search(searchReq)
	if err != nil {
		s.lock.Unlock()
		return err
	}

	humans := make([]humandao.Human, 0, len(result.Hits))
	for _, hit := range result.Hits {
		for _, h := range s.humans {
			if h.ID == hit.ID {
				humans = append(humans, h)
				break
			}
		}
	}
	s.lock.Unlock()

	if err := s.template.ExecuteTemplate(w, "search-suggestions.html", humans); err != nil {
		s.logger.Error().Err(err).Msg("unable to execute search-suggestions template")
	}

	return nil
}

func (s *ServerHTML) HandlerRandom(w http.ResponseWriter, r *http.Request) error {
	s.lock.Lock()
	var published []humandao.Human
	for _, h := range s.humans {
		if !h.Draft {
			published = append(published, h)
		}
	}
	s.lock.Unlock()

	if len(published) == 0 {
		http.Redirect(w, r, "/humans", http.StatusSeeOther)
		return nil
	}

	randomHuman := published[rand.Intn(len(published))]
	http.Redirect(w, r, "/humans/"+randomHuman.Path, http.StatusSeeOther)
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

	s.lock.Lock()
	var human humandao.Human
	for _, h := range s.humans {
		if h.Path == path || h.ID == path {
			human = h
			break
		}
	}
	
	var similar []humandao.Human
	if human.ID != "" {
		for _, humanID := range human.Similar {
			for _, h := range s.humans {
				if h.ID == humanID {
					similar = append(similar, h)
				}
			}
		}
	}
	s.lock.Unlock()
	
	if human.ID == "" {
		return NewNotFoundError(fmt.Errorf("%w: %v", humandao.ErrHumanNotFound, path))
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
		ethnicityList   = r.Form["ethnicity"]
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
		Ethnicity:   ethnicityList,
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

	if err := s.updateIndex(human); err != nil {
		s.logger.Error().Err(err).Str("id", human.ID).Msg("unable to update index")
	}
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
		ethnicityList   = r.Form["ethnicity"]
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
	human.Ethnicity = ethnicityList
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

	if err := s.updateIndex(human); err != nil {
		s.logger.Error().Err(err).Str("id", human.ID).Msg("unable to update index")
	}

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

	if err := s.deleteFromIndex(human.ID); err != nil {
		s.logger.Error().Err(err).Str("id", human.ID).Msg("unable to delete from index")
	}

	s.logger.Info().Str("id", human.ID).Str("name", human.Name).Msg("successfully deleted human")

	// If it's an HTMX request, we might want to redirect or just return empty
	if r.Header.Get("HX-Request") != "" {
		w.Header().Add("HX-Redirect", "/admin")
		return nil
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
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
	
	s.lock.Lock()
	for _, h := range s.humans {
		if h.Path == path {
			human = h
			break
		}
	}
	s.lock.Unlock()
	
	if human.ID == "" {
		return NewNotFoundError(humandao.ErrHumanNotFound)
	}

	if err := s.humanDAO.Publish(ctx, humandao.PublishInput{HumanID: human.ID, UserID: token.UID}); err != nil {
		return err
	}

	if err := s.updateIndex(human); err != nil {
		s.logger.Error().Err(err).Str("id", human.ID).Msg("unable to update index")
	}

	url := fmt.Sprintf("/humans/%s", human.Path)
	w.Header().Add("HX-Redirect", url)

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
