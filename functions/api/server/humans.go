package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/httplog"

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
