package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog"
	"github.com/rs/zerolog"

	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
)

type HumanCreateRequest struct {
	Name        string   `json:"name,omitempty"`
	Gender      string   `json:"gender,omitempty"`
	DOB         string   `json:"dob,omitempty"`
	DOD         string   `json:"dod,omitempty"`
	Ethnicity   []string `json:"ethnicity,omitempty"`
	Description string   `json:"description,omitempty"`
	Location    []string `json:"location,omitempty"`
	Website     string   `json:"website,omitempty"`
	Twitter     string   `json:"twitter,omitempty"`
	IMDB        string   `json:"imdb,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	ImagePath   string   `json:"image_path,omitempty"`
}

type HumanCreateResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	SignedURL string `json:"signedUrl,omitempty"`
}

func (s *Server) HumanCreate(w http.ResponseWriter, r *http.Request) (err error) {
	var (
		ctx   = r.Context()
		oplog = httplog.LogEntry(r.Context())
		token = Token(ctx)
	)

	defer func(start time.Time) {
		oplog.Err(err).
			Str("request", "HumanCreate").
			Str("token", token.UID).
			Dur("duration", time.Since(start).Round(time.Millisecond)).
			Msg("completed request")
	}(time.Now())

	var request HumanCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return NewBadRequestError(err)
	}

	isAdmin := IsAdmin(token)
	// if they are not admin, impose a limit on how many drafts they can create
	if !isAdmin {
		drafts, err := s.humanDAO.UserDrafts(ctx, humandao.UserDraftsInput{
			UserID: token.UID,
			Limit:  10,
			Offset: 0,
		})
		if err != nil {
			return NewInternalServerError(fmt.Errorf("unable to find user drafts: %w", err))
		}
		if len(drafts) > 5 {
			return NewBadRequestError(fmt.Errorf("too many contributions, please try again later"))
		}
	}

	extension := ""
	if request.ImagePath != "" {
		extension = strings.Split(request.ImagePath, ".")[1]
		if extension != "jpeg" && extension != "png" {
			return NewBadRequestError(fmt.Errorf("invalid image extension"))
		}
	}

	human, err := s.humanDAO.AddHuman(ctx, humandao.AddHumanInput{
		Name:        request.Name,
		Gender:      humandao.Gender(request.Gender),
		DOB:         request.DOB,
		DOD:         request.DOD,
		Ethnicity:   request.Ethnicity,
		Description: request.Description,
		Location:    request.Location,
		Website:     request.Website,
		Twitter:     request.Twitter,
		IMDB:        request.IMDB,
		Tags:        request.Tags,
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

	response := HumanCreateResponse{
		ID:   human.ID,
		Name: human.Name,
		Path: human.Path,
	}

	if extension != "" {
		// create a signed url for the user to upload an image
		// a cloud event will be triggered when the image is uploaded to set the image path on the human.
		contentType := "image/" + extension
		oplog.Info().Str("contentType", contentType).Msg("creating signed url")
		signedURL, err := s.storageClient.Bucket(api.ImagesStorageBucket).
			SignedURL(human.ID, &storage.SignedURLOptions{
				Method:      http.MethodPut,
				ContentType: contentType,
				Expires:     time.Now().Add(1 * time.Hour),
			})
		if err != nil {
			oplog.Err(err).Msg("unable to create signed url")
		}
		response.SignedURL = signedURL
	}

	s.writeData(w, http.StatusCreated, response)
	return nil
}

type Human struct {
	ID            string                 `json:"id,omitempty"`
	Name          string                 `json:"name,omitempty"`
	Gender        humandao.Gender        `json:"gender,omitempty"`
	Path          string                 `json:"path,omitempty"`
	ReactionCount humandao.ReactionCount `json:"reactionCount,omitempty"`
	DOB           string                 `json:"dob,omitempty"`
	DOD           string                 `json:"dod,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
	Ethnicity     []string               `json:"ethnicity,omitempty"`
	BirthLocation string                 `json:"birthLocation,omitempty"`
	Location      []string               `json:"location,omitempty"`
	InfluencedBy  []string               `json:"influencedBy,omitempty"`
	FeaturedImage string                 `json:"featuredImage,omitempty"`
	Draft         bool                   `json:"draft,omitempty"`
	AIGenerated   bool                   `json:"ai_generated,omitempty"`
	Description   string                 `json:"description,omitempty"`
	CreatedAt     time.Time              `json:"createdAt"`
	UpdatedAt     time.Time              `json:"updatedAt"`
	Affiliates    []Affiliate            `json:"affiliates"`
	Socials       Socials                `json:"socials"`
}

type Socials struct {
	IMDB    string `json:"imdb,omitempty"`
	X       string `json:"x,omitempty"`
	Website string `json:"website,omitempty"`
}

type Affiliate struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	Image string `json:"image"`
}

func (s *Server) HumansList(w http.ResponseWriter, r *http.Request) (err error) {
	var (
		ctx         = r.Context()
		oplog       = httplog.LogEntry(r.Context())
		limitStr    = r.URL.Query().Get("limit")
		limit       = numOrFallback(limitStr, 10)
		offsetStr   = r.URL.Query().Get("offset")
		offset      = numOrFallback(offsetStr, 0)
		orderBy     = r.URL.Query().Get("orderBy")
		dir         = r.URL.Query().Get("direction")
		ethnicity   = r.URL.Query().Get("ethnicity")
		gender      = r.URL.Query().Get("gender")
		olderThan   = r.URL.Query().Get("olderThan")
		youngerThan = r.URL.Query().Get("youngerThan")
		tags        = r.URL.Query()["tags"]
	)
	direction := firestore.Desc
	if dir == "asc" {
		direction = firestore.Asc
	}
	if orderBy == "" {
		orderBy = "created_at"
	}

	defer func(start time.Time) {
		oplog.Err(err).
			Str("request", "HumansList").
			Dur("duration", time.Since(start).Round(time.Millisecond)).
			Msg("completed request")
	}(time.Now())

	key := fmt.Sprintf("%v-%v-%d-%d-%v-%v-%v-%v-%v", orderBy, direction, limit, offset,
		ethnicity, gender, olderThan, youngerThan, strings.Join(tags, ","))
	raw, ok := s.humanCache.Get(key)
	var humans []humandao.Human
	if ok {
		humans = raw.([]humandao.Human)
	} else {
		zerolog.Ctx(ctx).Debug().Str("key", key).Msg("HumansList cache miss")
		humans, err = s.humanDAO.ListHumans(ctx, humandao.ListHumansInput{
			Limit:     limit,
			Offset:    offset,
			OrderBy:   orderBy,
			Direction: direction,
		})
		if err != nil {
			if errors.Is(err, humandao.ErrInvalidOrderBy) {
				return NewBadRequestError(err)
			}
			return NewInternalServerError(err)
		}

		// Apply Filters
		filters := []humandao.FilterOpt{}
		if ethnicity != "" {
			filters = append(filters, humandao.ByEthnicity(ethnicity))
		}
		if gender != "" {
			filters = append(filters, humandao.ByGender(humandao.Gender(gender)))
		}
		if olderThan != "" {
			age, err := time.Parse("2006-01-02", olderThan)
			if err != nil {
				return NewBadRequestError(err)
			}
			filters = append(filters, humandao.ByAgeOlderThan(age))
		}
		if youngerThan != "" {
			age, err := time.Parse("2006-01-02", youngerThan)
			if err != nil {
				return NewBadRequestError(err)
			}
			filters = append(filters, humandao.ByAgeYoungerThan(age))
		}
		if len(tags) > 0 {
			filters = append(filters, humandao.ByTags(tags...))
		}
		humans = humandao.ApplyFilters(humans, filters...)

		s.humanCache.SetDefault(key, humans)
	}

	humansResponse := convertHumans(humans)
	s.writeData(w, http.StatusOK, humansResponse)
	return nil
}
func (s *Server) HumanGet(w http.ResponseWriter, r *http.Request) (err error) {
	var (
		ctx   = r.Context()
		oplog = httplog.LogEntry(r.Context())
		path  = chi.URLParamFromCtx(ctx, "path")
	)
	defer func(start time.Time) {
		oplog.Err(err).
			Str("request", "HumanGet").
			Dur("duration", time.Since(start).Round(time.Millisecond)).
			Msg("completed request")
	}(time.Now())

	human, err := s.GetHumanFromCache(ctx, path)
	if err != nil {
		return err
	}

	s.writeData(w, http.StatusOK, convertHuman(human))
	return nil
}

// HumansByID finds all humans given a list of IDs, preserves order.
// All IDs must be valid, or a HTTP 404 will be returned.
func (s *Server) HumansByID(w http.ResponseWriter, r *http.Request) (err error) {
	var (
		ctx   = r.Context()
		oplog = httplog.LogEntry(r.Context())
	)

	defer func(start time.Time) {
		oplog.Err(err).
			Str("request", "HumansByID").
			Dur("duration", time.Since(start).Round(time.Millisecond)).
			Msg("completed request")
	}(time.Now())

	var humanIDs []string
	if err := json.NewDecoder(r.Body).Decode(&humanIDs); err != nil {
		return NewBadRequestError(err)
	}

	if len(humanIDs) > 100 {
		return NewBadRequestError(fmt.Errorf("too many humanIDs, max is 100"))
	}

	humans, err := s.GetHumansFromCache(ctx, humanIDs...)
	if err != nil {
		return err
	}

	s.writeData(w, http.StatusOK, convertHumans(humans))
	return nil
}

func convertHumans(humans []humandao.Human) (response []Human) {
	for _, human := range humans {
		response = append(response, convertHuman(human))
	}
	if response == nil {
		response = []Human{}
	}
	return response
}

func convertHuman(human humandao.Human) Human {
	var affiliates []Affiliate = []Affiliate{}
	for _, affiliate := range human.Affiliates {
		affiliates = append(affiliates, Affiliate{
			ID:    affiliate.ID,
			Name:  affiliate.Name,
			URL:   affiliate.URL,
			Image: affiliate.Image,
		})
	}

	return Human{
		ID:            human.ID,
		Name:          human.Name,
		Gender:        human.Gender,
		Path:          human.Path,
		ReactionCount: human.ReactionCount,
		DOB:           human.DOB,
		DOD:           human.DOD,
		Tags:          human.Tags,
		Ethnicity:     human.Ethnicity,
		BirthLocation: human.BirthLocation,
		Location:      human.Location,
		InfluencedBy:  human.InfluencedBy,
		FeaturedImage: human.FeaturedImage,
		Draft:         human.Draft,
		AIGenerated:   human.AIGenerated,
		Description:   human.Description,
		CreatedAt:     human.CreatedAt,
		UpdatedAt:     human.UpdatedAt,
		Affiliates:    affiliates,
		Socials: Socials{
			IMDB:    human.Socials.IMDB,
			X:       human.Socials.X,
			Website: human.Socials.Website,
		},
	}
}

func numOrFallback(num string, fallback int) int {
	result, err := strconv.Atoi(num)
	if err != nil {
		return fallback
	}
	return result
}
