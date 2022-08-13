package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog"
	"github.com/rs/zerolog"

	"github.com/raymonstah/asianamericanswiki/internal/humandao"
)

type Config struct {
	AuthClient *auth.Client
	HumansDAO  *humandao.DAO
	Logger     zerolog.Logger
}

type Server struct {
	authClient *auth.Client
	router     chi.Router
	logger     zerolog.Logger
	humansDAO  *humandao.DAO
}

func (s Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.router.ServeHTTP(writer, request)
}

func NewServer(config Config) Server {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(httplog.RequestLogger(config.Logger))
	r.Use(middleware.StripSlashes)
	r.Use(middleware.Recoverer)

	s := Server{
		authClient: config.AuthClient,
		router:     r,
		logger:     config.Logger,
		humansDAO:  config.HumansDAO,
	}

	s.setupRoutes()
	return s
}

func (s Server) setupRoutes() {
	s.router.Method(http.MethodGet, "/humans/{humanID}/reactions", Handler(s.ReactionsForHuman))

	s.router.Route("/reactions", func(r chi.Router) {
		r.Use(s.AuthMiddleware())
		r.Method(http.MethodGet, "/", Handler(s.GetReactions))
		r.Method(http.MethodPost, "/", Handler(s.PostReaction))
		r.Method(http.MethodDelete, "/{id}", Handler(s.DeleteReaction))
	})
}

type ReactionResponse struct {
	ID           string    `json:"id,omitempty"`
	UserID       string    `json:"user_id,omitempty"`
	HumanID      string    `json:"human_id,omitempty"`
	ReactionKind string    `json:"reaction_kind,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
}

func (s Server) ReactionsForHuman(w http.ResponseWriter, r *http.Request) (err error) {
	var (
		ctx     = r.Context()
		oplog   = httplog.LogEntry(r.Context())
		humanID = chi.URLParam(r, "humanID")
	)
	defer func(start time.Time) {
		oplog.Err(err).
			Str("request", "ReactionForHuman").
			Dur("duration", time.Since(start).Round(time.Millisecond)).
			Msg("completed request")
	}(time.Now())

	human, err := s.humansDAO.Human(ctx, humandao.HumanInput{
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
			Count:        human.ReactionCount[reactionKind],
		})
	}

	sort.Slice(response, func(i, j int) bool {
		return response[i].ReactionKind < response[j].ReactionKind
	})

	s.WriteData(w, http.StatusOK, response)
	return nil
}

func (s Server) GetReactions(w http.ResponseWriter, r *http.Request) (err error) {
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

	reactions, err := s.humansDAO.GetReactions(ctx, humandao.GetReactionsInput{UserID: token.UID})
	if err != nil {
		return NewInternalServerError(err)
	}

	reactionsResponse := toReactionsResponse(reactions)

	s.WriteData(w, http.StatusOK, reactionsResponse)
	return nil
}

func (s Server) PostReaction(w http.ResponseWriter, r *http.Request) (err error) {
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

	reaction, err := s.humansDAO.React(ctx, humandao.ReactInput{
		UserID:       token.UID,
		HumanID:      input.HumanID,
		ReactionKind: reactionKind,
	})
	if err != nil {
		return NewInternalServerError(err)
	}

	reactionResponse := toReactionResponse(reaction)
	s.WriteData(w, http.StatusCreated, reactionResponse)
	return nil
}

func (s Server) DeleteReaction(w http.ResponseWriter, r *http.Request) (err error) {
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

	err = s.humansDAO.ReactUndo(ctx, humandao.ReactUndoInput{UserID: token.UID, ReactionID: reactionID})
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

func (s Server) WriteData(w http.ResponseWriter, status int, data any) {
	w.WriteHeader(status)
	dataResponse := struct {
		Data any `json:"data"`
	}{
		Data: data,
	}
	if err := json.NewEncoder(w).Encode(dataResponse); err != nil {
		s.logger.Err(err).Msg("error encoding json data response")
	}
}
