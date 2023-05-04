package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog"

	"github.com/raymonstah/asianamericanswiki/internal/humandao"
)

type ReactionResponse struct {
	ID           string    `json:"id,omitempty"`
	UserID       string    `json:"user_id,omitempty"`
	HumanID      string    `json:"human_id,omitempty"`
	ReactionKind string    `json:"reaction_kind,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
}

func (s *Server) ReactionsForHuman(w http.ResponseWriter, r *http.Request) (err error) {
	var (
		ctx     = r.Context()
		oplog   = httplog.LogEntry(r.Context())
		humanID = chi.URLParam(r, "humanID")
	)
	defer func(start time.Time) {
		oplog.Err(err).
			Str("request", "ReactionsForHuman").
			Dur("duration", time.Since(start).Round(time.Millisecond)).
			Msg("completed request")
	}(time.Now())

	human, err := s.humanDAO.Human(ctx, humandao.HumanInput{
		HumanID: humanID,
	})
	if err != nil {
		if errors.Is(err, humandao.ErrHumanNotFound) {
			return NewNotFoundError(err)
		}
		return NewInternalServerError(err)
	}

	type reactionCountResponse struct {
		ReactionKind humandao.ReactionKind `json:"reaction_kind"`
		Count        int                   `json:"count"`
	}

	var response []reactionCountResponse

	for _, reactionKind := range humandao.AllReactionKinds {
		response = append(response, reactionCountResponse{
			ReactionKind: reactionKind,
			Count:        human.ReactionCount[string(reactionKind)],
		})
	}

	sort.Slice(response, func(i, j int) bool {
		return response[i].ReactionKind < response[j].ReactionKind
	})

	s.writeData(w, http.StatusOK, response)
	return nil
}

func (s *Server) GetReactions(w http.ResponseWriter, r *http.Request) (err error) {
	var (
		ctx   = r.Context()
		oplog = httplog.LogEntry(r.Context())
		token = Token(ctx)
	)
	defer func(start time.Time) {
		oplog.Err(err).
			Str("request", "GetReactions").
			Dur("duration", time.Since(start).Round(time.Millisecond)).
			Msg("completed request")
	}(time.Now())

	reactions, err := s.humanDAO.GetReactions(ctx, humandao.GetReactionsInput{UserID: token.UID})
	if err != nil {
		return NewInternalServerError(err)
	}

	reactionsResponse := toReactionsResponse(reactions)

	s.writeData(w, http.StatusOK, reactionsResponse)
	return nil
}

func (s *Server) PostReaction(w http.ResponseWriter, r *http.Request) (err error) {
	var (
		ctx   = r.Context()
		oplog = httplog.LogEntry(r.Context())
		token = Token(ctx)
	)
	defer func(start time.Time) {
		oplog.Err(err).
			Str("request", "PostReaction").
			Dur("duration", time.Since(start).Round(time.Millisecond)).
			Msg("completed request")
	}(time.Now())

	var input struct {
		HumanID      string `json:"human_id"`
		ReactionKind string `json:"reaction_kind"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		return NewBadRequestError(err)
	}

	reactionKind, err := humandao.ToReactionKind(input.ReactionKind)
	if err != nil {
		return NewBadRequestError(err)
	}

	reaction, err := s.humanDAO.React(ctx, humandao.ReactInput{
		UserID:       token.UID,
		HumanID:      input.HumanID,
		ReactionKind: reactionKind,
	})
	if err != nil {
		return NewInternalServerError(err)
	}

	reactionResponse := toReactionResponse(reaction)
	s.writeData(w, http.StatusCreated, reactionResponse)
	return nil
}

func (s *Server) DeleteReaction(w http.ResponseWriter, r *http.Request) (err error) {
	var (
		ctx        = r.Context()
		oplog      = httplog.LogEntry(r.Context())
		token      = Token(ctx)
		reactionID = chi.URLParam(r, "id")
	)
	defer func(start time.Time) {
		oplog.Err(err).
			Str("request", "DeleteReaction").
			Dur("duration", time.Since(start).Round(time.Millisecond)).
			Msg("completed request")
	}(time.Now())

	err = s.humanDAO.ReactUndo(ctx, humandao.ReactUndoInput{UserID: token.UID, ReactionID: reactionID})
	if err != nil {
		if errors.Is(err, humandao.ErrUnauthorized) {
			return NewForbiddenError(err)
		}
		return NewInternalServerError(err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func toReactionsResponse(reactions []humandao.Reaction) []ReactionResponse {
	reactionsResponse := make([]ReactionResponse, 0, len(reactions))
	for _, reaction := range reactions {
		reactionsResponse = append(reactionsResponse, toReactionResponse(reaction))
	}
	return reactionsResponse
}

func toReactionResponse(reaction humandao.Reaction) ReactionResponse {
	return ReactionResponse{
		ID:           reaction.ID,
		UserID:       reaction.UserID,
		HumanID:      reaction.HumanID,
		ReactionKind: string(reaction.ReactionKind),
		CreatedAt:    reaction.CreatedAt,
	}
}
