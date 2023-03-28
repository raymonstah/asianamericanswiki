// Package twitter is used and deployed as a Google cloud function.
// It is triggered by any changes to the /humans collection in Firestore.
// It then calls the Twitter API to follow/unfollow humans.
package twitter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/raymonstah/fsevent"
	"github.com/rs/zerolog"
)

type Human struct {
	Twitter string `firestore:"twitter,omitempty"`
	Draft   bool   `firestore:"draft,omitempty"`
}

// TwitterFollow is triggered by a change to a Firestore document.
func TwitterFollow(ctx context.Context, e fsevent.FirestoreEvent) error {
	consumerKey := os.Getenv("TWITTER_CONSUMER_KEY")
	consumerSecret := os.Getenv("TWITTER_CONSUMER_SECRET")
	accessToken := os.Getenv("TWITTER_ACCESS_TOKEN")
	accessSecret := os.Getenv("TWITTER_ACCESS_SECRET")
	if consumerKey == "" || consumerSecret == "" || accessToken == "" || accessSecret == "" {
		log.Fatal("Consumer key/secret and Access token/secret required")
	}
	logger := zerolog.New(os.Stdout)
	handler := NewHandler(logger, consumerKey, consumerSecret, accessToken, accessSecret)

	return handler.do(ctx, e)
}

func NewHandler(logger zerolog.Logger, consumerKey, consumerSecret, accessToken, accessSecret string) *Handler {
	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessSecret)
	httpClient := config.Client(oauth1.NoContext, token)

	client := twitter.NewClient(httpClient)
	return &Handler{
		client: client,
		logger: logger,
	}
}

type Handler struct {
	logger zerolog.Logger
	client *twitter.Client
}

func (h *Handler) do(ctx context.Context, e fsevent.FirestoreEvent) error {
	raw, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to marshal firestore event: %w", err)
	}
	h.logger.Info().Str("rawEvent", base64.StdEncoding.EncodeToString(raw)).Msg("")

	humanID := getHumanID(e)
	h.logger = h.logger.With().
		Str("type", e.Type()).
		Str("humanID", humanID).Logger()

	var human Human
	if err := e.Value.DataTo(&human); err != nil {
		return fmt.Errorf("unable to convert to human: %w", err)
	}

	if humanID == "" {
		h.logger.Info().Msg("human ID is empty")
		return nil
	}

	if e.Type() == fsevent.TypeCreate && human.Draft {
		// nothing to do if marked as Draft
		h.logger.Info().Msg("found human created as draft, ignoring..")
		return nil
	}

	handles := []string{parseHandle(human.Twitter)}

	if e.Type() == fsevent.TypeDelete || (e.Type() == fsevent.TypeUpdate && human.Draft) {
		if err := h.unfollowHandles(handles); err != nil {
			return err
		}
	} else {
		if err := h.followHandles(handles); err != nil {
			return err
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

func (h *Handler) unfollowHandles(toUnfollow []string) error {
	for _, toUnfollow := range toUnfollow {
		h.logger.Info().Str("user", toUnfollow).Msg("attempting to unfollow")
		_, _, err := h.client.Friendships.Destroy(&twitter.FriendshipDestroyParams{
			ScreenName: toUnfollow,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *Handler) followHandles(toFollows []string) error {
	for _, toFollow := range toFollows {
		h.logger.Info().Str("user", toFollow).Msg("attempting to follow")
		_, _, err := h.client.Friendships.Create(&twitter.FriendshipCreateParams{
			ScreenName: toFollow,
		})
		if err != nil {
			if !strings.Contains(err.Error(), "twitter: 160 You've already requested to follow") &&
				!strings.Contains(err.Error(), "twitter: 108 Cannot find specified user.") {
				return err
			}
		}
	}

	return nil
}

func parseHandle(raw string) string {
	handle := strings.ReplaceAll(raw, `"`, "")
	handle = strings.ReplaceAll(handle, `'`, "")
	handle = strings.TrimPrefix(handle, "https://twitter.com/")
	handle = strings.TrimPrefix(handle, "@")
	return handle
}
