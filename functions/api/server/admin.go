package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
)

func (s *Server) HumansDraft(w http.ResponseWriter, r *http.Request) (err error) {
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
			Str("request", "HumansDraft").
			Dur("duration", time.Since(start).Round(time.Millisecond)).
			Msg("completed request")
	}(time.Now())

	humans, err := s.humanDAO.Drafts(ctx, humandao.DraftsInput{
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

func (s *Server) HumansReview(w http.ResponseWriter, r *http.Request) (err error) {
	var (
		ctx   = r.Context()
		oplog = httplog.LogEntry(r.Context())
		id    = chi.URLParamFromCtx(ctx, "id")
		token = Token(ctx)
	)
	defer func(start time.Time) {
		oplog.Err(err).
			Str("request", "HumansReview").
			Dur("duration", time.Since(start).Round(time.Millisecond)).
			Msg("completed request")
	}(time.Now())

	human, err := s.humanDAO.Human(ctx, humandao.HumanInput{HumanID: id})
	if err != nil {
		if errors.Is(err, humandao.ErrHumanNotFound) {
			return NewNotFoundError(err)
		}

		return NewInternalServerError(err)
	}

	if !human.Draft {
		return NewBadRequestError(errors.New("not a draft"))
	}

	var body struct {
		Review string `json:"review"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return NewBadRequestError(err)
	}

	if body.Review == "approve" {
		err = s.humanDAO.Publish(ctx, humandao.PublishInput{
			HumanID: id,
			UserID:  token.UID,
		})
		// todo: refresh cache
	} else {
		err = s.humanDAO.Delete(ctx, humandao.DeleteInput{
			HumanID: id,
		})
	}
	if err != nil {
		return NewInternalServerError(err)
	}

	s.writeData(w, http.StatusNoContent, nil)
	return nil
}
