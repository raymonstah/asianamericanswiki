// Package main is used and deployed as a Google cloud function.
// It is triggered by any changes to the /humans collection in Firestore.
// It then calls the Twitter API to follow/unfollow humans.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/dghubble/oauth1"
	twitter "github.com/g8rswimmer/go-twitter/v2"
	"github.com/raymonstah/fsevent"
)

type Human struct {
	Twitter string `firestore:"twitter,omitempty"`
	Draft   bool   `firestore:"draft,omitempty"`
}

// TwitterFollow is triggered by a change to a Firestore document.
func TwitterFollow(ctx context.Context, e fsevent.FirestoreEvent) error {
	apiKey := os.Getenv("TWITTER_API_KEY")
	apiKeySecret := os.Getenv("TWITTER_API_KEY_SECRET")
	accessToken := os.Getenv("TWITTER_ACCESS_TOKEN")
	accessSecret := os.Getenv("TWITTER_ACCESS_SECRET")
	if apiKey == "" || apiKeySecret == "" || accessToken == "" || accessSecret == "" {
		log.Fatal("API key/secret and Access token/secret required")
	}
	handler := NewHandler(apiKey, apiKeySecret, accessToken, accessSecret)

	return handler.do(ctx, e)
}

type NoopAuthorizer struct{}

func (n NoopAuthorizer) Add(req *http.Request) {}

func NewHandler(apiKey, apiKeySecret, accessToken, accessSecret string) *Handler {
	config := oauth1.NewConfig(apiKey, apiKeySecret)
	httpClient := config.Client(oauth1.NoContext, &oauth1.Token{
		Token:       accessToken,
		TokenSecret: accessSecret,
	})

	var noopAuthorizer NoopAuthorizer
	client := &twitter.Client{
		Authorizer: noopAuthorizer,
		Client:     httpClient,
		Host:       "https://api.twitter.com",
	}

	return &Handler{
		client: client,
	}
}

type Handler struct {
	client *twitter.Client
}

func (h *Handler) do(ctx context.Context, e fsevent.FirestoreEvent) error {
	// lookup the current user
	resp, err := h.client.AuthUserLookup(ctx, twitter.UserLookupOpts{})
	if err != nil {
		return fmt.Errorf("unable to lookup user: %w", err)
	}
	userID := resp.Raw.Users[0].ID
	slog.Info("user id", slog.String("userID", userID))

	raw, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to marshal firestore event: %w", err)
	}
	slog.Info("processing event", slog.String("rawEvent", base64.StdEncoding.EncodeToString(raw)))

	humanID := getHumanID(e)
	logger := slog.With(slog.String("humanID", humanID)).With(slog.String("type", e.Type()))
	var human Human
	if err := e.Value.DataTo(&human); err != nil {
		return fmt.Errorf("unable to convert to human: %w", err)
	}

	if humanID == "" {
		logger.Error("human ID is empty")
		return nil
	}

	if e.Type() == fsevent.TypeCreate && human.Draft {
		// nothing to do if marked as Draft
		logger.Info("found human created as draft, ignoring..")
		return nil
	}

	handles := []string{parseHandle(human.Twitter)}

	if e.Type() == fsevent.TypeDelete || (e.Type() == fsevent.TypeUpdate && human.Draft) {
		if err := h.unfollowHandles(ctx, userID, handles); err != nil {
			return err
		}
	} else {
		if err := h.followHandles(ctx, userID, handles); err != nil {
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

func (h *Handler) unfollowHandles(ctx context.Context, userID string, toUnfollow []string) error {
	for _, toUnfollow := range toUnfollow {
		slog.Info("attempting to unfollow", slog.Any("toUnfollow", toUnfollow))
		_, err := h.client.DeleteUserFollows(ctx, userID, toUnfollow)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *Handler) followHandles(ctx context.Context, userID string, toFollows []string) error {
	for _, toFollow := range toFollows {
		slog.Info("attempting to follow", slog.Any("toFollow", toFollow))
		_, err := h.client.UserFollows(ctx, userID, toFollow)
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
