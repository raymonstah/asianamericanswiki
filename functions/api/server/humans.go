package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog"
	"github.com/segmentio/ksuid"

	"github.com/raymonstah/asianamericanswiki/internal/humandao"
)

type HumanCreateRequest struct {
	Name string
}

type HumanCreateResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s Server) HumanCreate(w http.ResponseWriter, r *http.Request) (err error) {
	var (
		ctx   = r.Context()
		oplog = httplog.LogEntry(r.Context())
	)
	defer func(start time.Time) {
		oplog.Err(err).
			Str("request", "HumanCreate").
			Dur("duration", time.Since(start).Round(time.Millisecond)).
			Msg("completed request")
	}(time.Now())

	var request HumanCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return NewBadRequestError(err)
	}

	human, err := s.humanDAO.AddHuman(ctx, humandao.AddHumanInput{
		Name: request.Name,
	})
	if err != nil {
		return NewInternalServerError(err)
	}

	response := HumanCreateResponse{
		ID:   human.ID,
		Name: human.Name,
	}

	s.writeData(w, http.StatusCreated, response)
	return nil
}

type HumansListResponse struct {
	Humans []Human `json:"humans"`
}

type Human struct {
	ID            string                 `json:"id,omitempty"`
	Name          string                 `json:"name,omitempty"`
	Path          string                 `json:"path,omitempty"`
	ReactionCount humandao.ReactionCount `json:"reactionCount,omitempty"`
	DOB           string                 `json:"dob,omitempty"`
	DOD           string                 `json:"dod,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
	Website       string                 `json:"website,omitempty"`
	Ethnicity     []string               `json:"ethnicity,omitempty"`
	BirthLocation string                 `json:"birthLocation,omitempty"`
	Location      []string               `json:"location,omitempty"`
	InfluencedBy  []string               `json:"influencedBy,omitempty"`
	Twitter       string                 `json:"twitter,omitempty"`
	FeaturedImage string                 `json:"featuredImage,omitempty"`
	Draft         bool                   `json:"draft,omitempty"`
	AIGenerated   bool                   `json:"aIGenerated,omitempty"`
	Description   string                 `json:"description,omitempty"`
	CreatedAt     time.Time              `json:"createdAt"`
	UpdatedAt     time.Time              `json:"updatedAt"`
}

func (s Server) HumansList(w http.ResponseWriter, r *http.Request) (err error) {
	var (
		ctx       = r.Context()
		oplog     = httplog.LogEntry(r.Context())
		limitStr  = r.URL.Query().Get("limit")
		limit     = numOrFallback(limitStr, 10)
		offsetStr = r.URL.Query().Get("offset")
		offset    = numOrFallback(offsetStr, 0)
	)

	defer func(start time.Time) {
		oplog.Err(err).
			Str("request", "HumansList").
			Dur("duration", time.Since(start).Round(time.Millisecond)).
			Msg("completed request")
	}(time.Now())

	humans, err := s.humanDAO.ListHumans(ctx, humandao.ListHumansInput{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return NewInternalServerError(err)
	}

	humansResponse := convertHumans(humans)
	s.writeData(w, http.StatusOK, humansResponse)
	return nil
}

func (s Server) HumanGet(w http.ResponseWriter, r *http.Request) (err error) {
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

	id := ""
	if _, err := ksuid.Parse(path); err == nil {
		id = path
	}

	human, err := s.humanDAO.Human(ctx, humandao.HumanInput{
		HumanID: id,
		Path:    path,
	})
	if err != nil {
		if errors.Is(err, humandao.ErrHumanNotFound) {
			return NewNotFoundError(err)
		}
		return NewInternalServerError(err)
	}

	s.writeData(w, http.StatusOK, convertHuman(human))
	return nil
}

func convertHumans(humans []humandao.Human) (response []Human) {
	for _, human := range humans {
		response = append(response, convertHuman(human))
	}
	return response
}

func convertHuman(human humandao.Human) Human {
	return Human{
		ID:            human.ID,
		Name:          human.Name,
		Path:          human.Path,
		ReactionCount: human.ReactionCount,
		DOB:           human.DOB,
		DOD:           human.DOD,
		Tags:          human.Tags,
		Website:       human.Website,
		Ethnicity:     human.Ethnicity,
		BirthLocation: human.BirthLocation,
		Location:      human.Location,
		InfluencedBy:  human.InfluencedBy,
		Twitter:       human.Twitter,
		FeaturedImage: human.FeaturedImage,
		Draft:         human.Draft,
		AIGenerated:   human.AIGenerated,
		Description:   human.Description,
		CreatedAt:     human.CreatedAt,
		UpdatedAt:     human.UpdatedAt,
	}
}

func numOrFallback(num string, fallback int) int {
	result, err := strconv.Atoi(num)
	if err != nil {
		return fallback
	}
	return result
}
