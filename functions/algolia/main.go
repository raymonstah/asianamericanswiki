// Package algolia is used and deployed as a Google cloud function.
// It is triggered by any changes to the /humans collection in Firestore.
// It then updates the Algolia search index.
package algolia

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/algolia/algoliasearch-client-go/v3/algolia/search"
	"github.com/raymonstah/fsevent"
	"github.com/rs/zerolog"
)

var (
	algoliaApiKey  = os.Getenv("ALGOLIA_API_KEY")
	algoliaAppID   = os.Getenv("ALGOLIA_APP_ID")
	collectionPath = "humans"
)

type Human struct {
	ID            string   `json:"objectID"`
	Name          string   `firestore:"name" json:"name"`
	Path          string   `firestore:"urn_path" json:"urn_path"`
	DOB           string   `firestore:"dob,omitempty" json:"dob,omitempty"`
	DOD           string   `firestore:"dod,omitempty" json:"dod,omitempty"`
	Tags          []string `firestore:"tags,omitempty" json:"tags,omitempty"`
	Website       string   `firestore:"website,omitempty" json:"website,omitempty"`
	Ethnicity     []string `firestore:"ethnicity,omitempty" json:"ethnicity,omitempty"`
	BirthLocation string   `firestore:"birth_location,omitempty" json:"birth_location,omitempty"`
	Location      []string `firestore:"location,omitempty" json:"location,omitempty"`
	InfluencedBy  []string `firestore:"influenced_by,omitempty" json:"influenced_by,omitempty"`
	Twitter       string   `firestore:"twitter,omitempty" json:"twitter,omitempty"`
	Draft         bool     `firestore:"draft,omitempty" json:"draft,omitempty"`
	AIGenerated   bool     `firestore:"ai_generated,omitempty" json:"ai_generated,omitempty"`
	Description   string   `firestore:"description,omitempty" json:"description,omitempty"`
}

// HelloFirestore is triggered by a change to a Firestore document.
func HelloFirestore(ctx context.Context, e fsevent.FirestoreEvent) error {
	algoliaClient := search.NewClient(algoliaAppID, algoliaApiKey)
	logger := zerolog.New(os.Stdout)
	handler := NewHandler(algoliaClient, logger)
	return handler.algoliaSync(ctx, e)
}

func NewHandler(algolia search.ClientInterface, logger zerolog.Logger) *Handler {
	return &Handler{
		algolia: algolia,
		logger:  logger,
	}
}

type Handler struct {
	algolia search.ClientInterface
	logger  zerolog.Logger
}

func (h *Handler) algoliaSync(ctx context.Context, e fsevent.FirestoreEvent) error {
	//meta, err := metadata.FromContext(ctx)
	//if err != nil {
	//	return fmt.Errorf("metadata.FromContext: %v", err)
	//}

	raw, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to marshal firestore event: %w", err)
	}
	h.logger.Info().Str("rawEvent", base64.StdEncoding.EncodeToString(raw)).Msg("")

	humanID := getHumanID(e)
	h.logger = h.logger.With().
		Str("type", e.Type()).
		Str("humanID", humanID).Logger()

	index := h.algolia.InitIndex(collectionPath)
	var human Human
	if err := e.Value.DataTo(&human); err != nil {
		return fmt.Errorf("unable to convert to human: %w", err)
	}
	human.ID = humanID

	if human.ID == "" {
		h.logger.Info().Msg("human ID is empty")
		return nil
	}

	if e.Type() == fsevent.TypeCreate && human.Draft {
		// nothing to do if marked as Draft
		h.logger.Info().Msg("found human created as draft, ignoring..")
		return nil
	}

	if e.Type() == fsevent.TypeDelete || (e.Type() == fsevent.TypeUpdate && human.Draft) {
		h.logger.Info().Msg("deleting")
		_, err := index.DeleteObject(humanID)
		if err != nil {
			return fmt.Errorf("unable to delete object: %w", err)
		}
	} else {
		if human.Name == "" {
			h.logger.Info().Msg("human name is empty")
			return nil
		}
		h.logger.Info().Msg("saving")
		_, err = index.SaveObject(human)
		if err != nil {
			return fmt.Errorf("unable to save object to algolia: %w", err)
		}
	}
	return nil
}

func getHumanID(e fsevent.FirestoreEvent) string {
	var path string
	if e.OldValue != nil && e.OldValue.Name != "" {
		path = e.OldValue.Name
	}
	if e.Value != nil && e.Value.Name != "" {
		path = e.Value.Name
	}

	if path == "" {
		return ""
	}
	lastSlashIndex := strings.LastIndex(path, "/")
	if lastSlashIndex == -1 {
		return ""
	}

	humanID := path[lastSlashIndex+1:]
	return humanID
}
