package server

import (
	"context"
	"errors"

	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/segmentio/ksuid"
)

func (s *Server) GetHumansFromCache(ctx context.Context, humanIDs ...string) ([]humandao.Human, error) {
	cacheMiss := make([]string, 0, len(humanIDs))
	humans := make([]humandao.Human, 0, len(humanIDs))
	for _, humanID := range humanIDs {
		humanRaw, found := s.humanCache.Get(humanID)
		if !found {
			s.logger.Debug().Str("humanID", humanID).Msg("cache miss")
			cacheMiss = append(cacheMiss, humanID)
		} else {
			human := humanRaw.(humandao.Human)
			humans = append(humans, human)
		}
	}

	remainingHumans, err := s.humanDAO.HumansByID(ctx, humandao.HumansByIDInput{
		HumanIDs: cacheMiss,
	})
	if err != nil {
		if errors.Is(err, humandao.ErrHumanNotFound) {
			return nil, NewNotFoundError(err)
		}
		return nil, NewInternalServerError(err)
	}
	for _, human := range remainingHumans {
		s.humanCache.SetDefault(human.ID, human)
		s.humanCache.SetDefault(human.Path, human)
	}

	humans = append(humans, remainingHumans...)

	// Preserve ordering
	idToHuman := make(map[string]humandao.Human)
	for _, human := range humans {
		idToHuman[human.ID] = human
	}

	orderedHumans := make([]humandao.Human, 0, len(humanIDs))
	for _, id := range humanIDs {
		orderedHumans = append(orderedHumans, idToHuman[id])
	}

	return orderedHumans, nil
}

func (s *Server) GetHumanFromCache(ctx context.Context, humanID string) (human humandao.Human, err error) {
	humanRaw, found := s.humanCache.Get(humanID)
	if !found {
		id := ""
		if _, err := ksuid.Parse(humanID); err == nil {
			id = humanID
		}

		human, err := s.humanDAO.Human(ctx, humandao.HumanInput{
			HumanID: id,
			Path:    humanID,
		})

		if err != nil {
			if errors.Is(err, humandao.ErrHumanNotFound) {
				return humandao.Human{}, NewNotFoundError(err)
			}
			return humandao.Human{}, NewInternalServerError(err)
		}
		// Make the human findable by both its ID and its path.
		s.humanCache.SetDefault(humanID, human)
		s.humanCache.SetDefault(human.Path, human)
		return human, nil
	}

	return humanRaw.(humandao.Human), nil
}
