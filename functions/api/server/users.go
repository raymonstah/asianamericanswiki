package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/raymonstah/asianamericanswiki/internal/userdao"
)

type User struct {
	ID             string           `json:"id"`
	Saved          []Saved          `json:"saved"`
	RecentlyViewed []RecentlyViewed `json:"recently_viewed"`
}

type Saved struct {
	HumanID string    `json:"human_id"`
	SavedAt time.Time `json:"saved_at"`
}

type RecentlyViewed struct {
	HumanID  string    `json:"human_id"`
	ViewedAt time.Time `json:"viewed_at"`
}

func (s *Server) User(w http.ResponseWriter, r *http.Request) (err error) {
	var (
		ctx      = r.Context()
		oplog    = httplog.LogEntry(r.Context())
		userAuth = Token(ctx)
	)

	defer func(start time.Time) {
		oplog.Err(err).
			Str("request", "User").
			Str("uid", userAuth.UID).
			Dur("duration", time.Since(start).Round(time.Millisecond)).
			Msg("completed request")
	}(time.Now())

	user, err := s.userDAO.User(ctx, userAuth.UID, userdao.WithRecentlyViewed(), userdao.WithSaved())
	if err != nil {
		return NewInternalServerError(err)
	}

	userResponse := toUserResponse(user)
	s.writeData(w, http.StatusOK, userResponse)
	return nil
}

func (s *Server) ViewHuman(w http.ResponseWriter, r *http.Request) (err error) {
	var (
		ctx     = r.Context()
		oplog   = httplog.LogEntry(r.Context())
		humanID = chi.URLParam(r, "humanID")
		user    = Token(ctx)
	)

	defer func(start time.Time) {
		uid := ""
		if user != nil {
			uid = user.UID
		}
		oplog.Err(err).
			Str("request", "ViewHuman").
			Str("uid", uid).
			Dur("duration", time.Since(start).Round(time.Millisecond)).
			Msg("completed request")
	}(time.Now())

	human, err := s.GetHumanFromCache(ctx, humanID)
	if err != nil {
		return err
	}

	if human.Draft {
		return nil
	}

	if err := s.humanDAO.View(ctx, humandao.ViewInput{
		HumanID: humanID,
	}); err != nil {
		return NewInternalServerError(err)
	}

	if user != nil {
		err = s.userDAO.ViewHuman(ctx, userdao.ViewHumanInput{
			HumanID: humanID,
			UserID:  user.UID,
		})
		if err != nil {
			return NewInternalServerError(err)
		}
	}

	s.writeData(w, http.StatusNoContent, nil)
	return nil
}

func (s *Server) SaveHuman(w http.ResponseWriter, r *http.Request) (err error) {
	var (
		ctx     = r.Context()
		oplog   = httplog.LogEntry(r.Context())
		humanID = chi.URLParam(r, "humanID")
		user    = Token(ctx)
	)

	defer func(start time.Time) {
		oplog.Err(err).
			Str("request", "SaveHuman").
			Str("uid", user.UID).
			Dur("duration", time.Since(start).Round(time.Millisecond)).
			Msg("completed request")
	}(time.Now())

	_, err = s.GetHumanFromCache(ctx, humanID)
	if err != nil {
		return err
	}

	err = s.userDAO.SaveHuman(ctx, userdao.SaveHumanInput{
		HumanID: humanID,
		UserID:  user.UID,
	})
	if err != nil {
		return NewInternalServerError(err)
	}

	s.writeData(w, http.StatusNoContent, nil)
	return nil
}

func (s *Server) UnsaveHuman(w http.ResponseWriter, r *http.Request) (err error) {
	var (
		ctx     = r.Context()
		oplog   = httplog.LogEntry(r.Context())
		humanID = chi.URLParam(r, "humanID")
		user    = Token(ctx)
	)

	defer func(start time.Time) {
		oplog.Err(err).
			Str("request", "UnsaveHuman").
			Str("uid", user.UID).
			Dur("duration", time.Since(start).Round(time.Millisecond)).
			Msg("completed request")
	}(time.Now())

	_, err = s.GetHumanFromCache(ctx, humanID)
	if err != nil {
		return err
	}

	err = s.userDAO.UnsaveHuman(ctx, userdao.UnsaveHumanInput{
		HumanID: humanID,
		UserID:  user.UID,
	})
	if err != nil {
		return NewInternalServerError(err)
	}

	s.writeData(w, http.StatusNoContent, nil)
	return nil
}

func toUserResponse(user userdao.User) User {
	saved := make([]Saved, 0, len(user.Saved))
	recentlyViewed := make([]RecentlyViewed, 0, len(user.RecentlyViewed))

	for _, s := range user.Saved {
		saved = append(saved, Saved(s))
	}

	for _, rv := range user.RecentlyViewed {
		recentlyViewed = append(recentlyViewed, RecentlyViewed(rv))
	}

	return User{
		ID:             user.ID,
		Saved:          saved,
		RecentlyViewed: recentlyViewed,
	}
}
