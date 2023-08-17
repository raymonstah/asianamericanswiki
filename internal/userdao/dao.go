package userdao

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DAO struct {
	client             *firestore.Client
	userCollectionName string
}

type Option func(d *DAO)

func WithUserCollectionName(name string) Option {
	return func(d *DAO) {
		d.userCollectionName = name
	}
}

func NewDAO(client *firestore.Client, options ...Option) *DAO {
	dao := &DAO{
		client:             client,
		userCollectionName: "users",
	}

	for _, opt := range options {
		opt(dao)
	}

	return dao
}

type RecentlyViewed struct {
	HumanID  string    `firestore:"human_id,omitempty"`
	ViewedAt time.Time `firestore:"viewed_at,omitempty"`
}

type Saved struct {
	HumanID string    `firestore:"human_id,omitempty"`
	SavedAt time.Time `firestore:"saved_at,omitempty"`
}

type User struct {
	ID             string           `firestore:"id,omitempty"`
	RecentlyViewed []RecentlyViewed `firestore:"recently_viewed,omitempty"`
	Saved          []Saved          `firestore:"saved,omitempty"`
}

type SaveHumanInput struct {
	UserID  string
	HumanID string
}

type userOptions struct {
	getRecentlyViewed bool
	getSaved          bool
}

type UserOption func(o *userOptions)

func WithRecentlyViewed() UserOption {
	return func(o *userOptions) {
		o.getRecentlyViewed = true
	}
}

func WithSaved() UserOption {
	return func(o *userOptions) {
		o.getSaved = true
	}
}

func (d *DAO) User(ctx context.Context, id string, options ...UserOption) (User, error) {
	var userOptions userOptions
	for _, opt := range options {
		opt(&userOptions)
	}

	doc, err := d.client.Collection(d.userCollectionName).Doc(id).Get(ctx)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			return User{}, fmt.Errorf("unable to get user: %v: %w", id, err)
		}
	}

	var user User
	if err := doc.DataTo(&user); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("unable to convert user")
	}

	user.ID = id

	if userOptions.getRecentlyViewed {
		var recentlyViewed []RecentlyViewed
		recentlyViewedRefs, err := doc.Ref.Collection("recently_viewed").OrderBy("viewed_at", firestore.Desc).Limit(50).Documents(ctx).GetAll()
		if err != nil {
			return User{}, fmt.Errorf("unable to get recently viewed humans for user %v: %w", id, err)
		}

		for _, r := range recentlyViewedRefs {
			var rv RecentlyViewed
			if err := r.DataTo(&rv); err != nil {
				return User{}, fmt.Errorf("unable to convert recently viewed human for user %v: %w", id, err)
			}
			recentlyViewed = append(recentlyViewed, rv)
		}

		user.RecentlyViewed = recentlyViewed
	}

	if userOptions.getSaved {
		var saved []Saved
		savedRefs, err := doc.Ref.Collection("saved").OrderBy("saved_at", firestore.Desc).Limit(50).Documents(ctx).GetAll()
		if err != nil {
			return User{}, fmt.Errorf("unable to get saved humans for user %v: %w", id, err)
		}

		for _, r := range savedRefs {
			var s Saved
			if err := r.DataTo(&s); err != nil {
				return User{}, fmt.Errorf("unable to convert saved human for user %v: %w", id, err)
			}
			saved = append(saved, s)
		}

		user.Saved = saved
	}

	return user, nil
}

// SaveHuman indicates that the user has saved a given human.
func (d *DAO) SaveHuman(ctx context.Context, input SaveHumanInput) error {
	// return early if the input.HumanID has already been saved by the user
	docs, err := d.client.Collection(d.userCollectionName).Doc(input.UserID).
		Collection("saved").Where("human_id", "==", input.HumanID).Documents(ctx).GetAll()
	if err != nil {
		return fmt.Errorf("unable to check if human %v has already been saved by user %v: %w", input.HumanID, input.UserID, err)
	}
	if len(docs) > 0 {
		return nil
	}

	now := time.Now().UTC()
	_, err = d.client.Collection(d.userCollectionName).Doc(input.UserID).
		Collection("saved").NewDoc().
		Create(ctx, Saved{
			HumanID: input.HumanID,
			SavedAt: now,
		})
	if err != nil {
		return fmt.Errorf("unable to mark human %v as saved by %v: %w", input.HumanID, input.UserID, err)
	}

	return nil
}

type ViewHumanInput struct {
	UserID  string
	HumanID string
}

// ViewHuman indicates that the user has viewed a given human.
func (d *DAO) ViewHuman(ctx context.Context, input ViewHumanInput) error {
	// Get the last human viewed.
	docs, err := d.client.Collection(d.userCollectionName).Doc(input.UserID).
		Collection("recently_viewed").OrderBy("viewed_at", firestore.Desc).Limit(1).Documents(ctx).GetAll()
	if err != nil {
		return fmt.Errorf("unable to check if human %v has already been saved by user %v: %w", input.HumanID, input.UserID, err)
	}

	if len(docs) > 0 {
		var recentlyViewed RecentlyViewed
		if err := docs[0].DataTo(&recentlyViewed); err != nil {
			return fmt.Errorf("unable to convert recently viewed human for user %v: %w", input.UserID, err)
		}
		// Same as the last one viewed, so ignore
		if recentlyViewed.HumanID == input.HumanID {
			return nil
		}
	}

	now := time.Now().UTC()
	_, err = d.client.Collection(d.userCollectionName).Doc(input.UserID).
		Collection("recently_viewed").NewDoc().
		Create(ctx, RecentlyViewed{
			HumanID:  input.HumanID,
			ViewedAt: now,
		})
	if err != nil {
		return fmt.Errorf("unable to mark human %v as viewed by %v: %w", input.HumanID, input.UserID, err)
	}

	return nil
}
