package humandao

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/go-chi/httplog"
	"github.com/segmentio/ksuid"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrHumanNotFound      = errors.New("human not found")
	ErrHumanAlreadyExists = errors.New("human already exists")
)

type ReactionCount map[string]int

type Human struct {
	ID            string        `firestore:"-"`
	Name          string        `firestore:"name"`
	Path          string        `firestore:"urn_path"`
	ReactionCount ReactionCount `firestore:"reaction_count"`
	DOB           string        `firestore:"dob,omitempty"`
	DOD           string        `firestore:"did,omitempty"`
	Tags          []string      `firestore:"tags,omitempty"`
	Website       string        `firestore:"website,omitempty"`
	Ethnicity     []string      `firestore:"ethnicity,omitempty"`
	BirthLocation string        `firestore:"birth_location,omitempty"`
	Location      []string      `firestore:"location,omitempty"`
	InfluencedBy  []string      `firestore:"influenced_by,omitempty"`
	Twitter       string        `firestore:"twitter,omitempty"`
	FeaturedImage string        `firestore:"featured_image,omitempty"`
	Draft         bool          `firestore:"draft,omitempty"`
	AIGenerated   bool          `firestore:"ai_generated,omitempty"`
	Description   string        `firestore:"description,omitempty"`

	CreatedAt time.Time `firestore:"created_at"`
	CreatedBy string    `firestore:"created_by,omitempty"`

	UpdatedAt   time.Time `firestore:"updated_at"`
	PublishedBy string    `firestore:"published_by,omitempty"`
	PublishedAt time.Time `firestore:"published_at,omitempty"`
}

type Reaction struct {
	ID           string       `firestore:"-"`
	UserID       string       `firestore:"user_id,omitempty"`
	HumanID      string       `firestore:"human_id,omitempty"`
	ReactionKind ReactionKind `firestore:"reaction_kind,omitempty"`
	CreatedAt    time.Time    `firestore:"created_at,omitempty"`
}

type HumanInput struct {
	HumanID string
	Path    string
}

func (d *DAO) Human(ctx context.Context, input HumanInput) (human Human, err error) {
	logger := httplog.LogEntry(ctx)
	var doc *firestore.DocumentSnapshot
	if input.HumanID != "" {
		doc, err = d.client.Collection(d.humanCollection).Doc(input.HumanID).Get(ctx)
	} else if input.Path != "" {
		doc, err = d.client.Collection(d.humanCollection).Where("urn_path", "==", input.Path).
			Documents(ctx).Next()
	}
	if err != nil {
		if status.Code(err) == codes.NotFound || err == iterator.Done {
			logger.Error().Err(err).Interface("input", input).Msg("human not found")
			return Human{}, ErrHumanNotFound
		}
		logger.Err(err).Interface("input", input).Msg("unable to get human")
		return Human{}, fmt.Errorf("unable to get human: %w", err)
	}

	human, err = convertHumanDoc(doc)
	if err != nil {
		return Human{}, fmt.Errorf("unable to convert human: %w", err)
	}

	human.ID = doc.Ref.ID
	return human, nil
}

func (d *DAO) UpdateHuman(ctx context.Context, human Human) error {
	human.UpdatedAt = time.Now()
	_, err := d.client.Collection(d.humanCollection).
		Doc(human.ID).
		Set(ctx, human)
	if err != nil {
		return fmt.Errorf("unable to update human: %v (%v): %w", human.Name, human.ID, err)
	}

	return nil
}

type AddHumanInput struct {
	HumanID     string
	Name        string
	DOB         string
	DOD         string
	Ethnicity   []string
	Description string
	Location    []string
	Website     string
	Twitter     string
	Tags        []string
	Draft       bool
	CreatedBy   string
}

func (d *DAO) AddHuman(ctx context.Context, input AddHumanInput) (Human, error) {
	path := strings.ToLower(strings.ReplaceAll(input.Name, " ", "-"))
	if input.Name == "" {
		return Human{}, fmt.Errorf("name must be provided")
	}

	_, err := d.Human(ctx, HumanInput{Path: path})
	if err != nil {
		if err != ErrHumanNotFound {
			return Human{}, fmt.Errorf("error checking if human (%v) exists: %w", path, err)
		}
	}
	if err == nil {
		return Human{}, ErrHumanAlreadyExists
	}

	human := Human{
		Name:        input.Name,
		DOB:         input.DOB,
		DOD:         input.DOD,
		Ethnicity:   input.Ethnicity,
		Description: input.Description,
		Location:    input.Location,
		Website:     input.Website,
		Twitter:     input.Twitter,
		Tags:        input.Tags,
		Draft:       input.Draft,
		CreatedBy:   input.CreatedBy,
		Path:        path,
	}

	if input.HumanID == "" {
		input.HumanID = ksuid.New().String()
	}

	_, err = d.client.Collection(d.humanCollection).Doc(input.HumanID).Create(ctx, human)
	if err != nil {
		return Human{}, fmt.Errorf("unable to create human: %w", err)
	}

	human.ID = input.HumanID
	return human, nil
}

var (
	ErrUnauthorized = errors.New("user is not authorized to perform this operation")
)

type ReactionKind string

var (
	ReactionKindLove   ReactionKind = "love"
	ReactionKindFire   ReactionKind = "fire"
	ReactionKindJoy    ReactionKind = "joy"
	ReactionKindFlower ReactionKind = "flower"
)
var validReactionKinds = map[ReactionKind]struct{}{
	ReactionKindLove:   {},
	ReactionKindFire:   {},
	ReactionKindJoy:    {},
	ReactionKindFlower: {},
}

var AllReactionKinds = []ReactionKind{ReactionKindLove, ReactionKindFire, ReactionKindJoy}

func ToReactionKind(kind string) (ReactionKind, error) {
	if _, ok := validReactionKinds[ReactionKind(kind)]; !ok {
		return "", fmt.Errorf("invalid reaction kind")
	}

	return ReactionKind(kind), nil
}

type ReactInput struct {
	UserID       string
	HumanID      string
	ReactionKind ReactionKind
}

func (d *DAO) React(ctx context.Context, input ReactInput) (Reaction, error) {
	data := Reaction{
		UserID:       input.UserID,
		HumanID:      input.HumanID,
		ReactionKind: input.ReactionKind,
		CreatedAt:    time.Now(),
	}

	err := d.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		reactionRef := d.client.Collection(d.reactionCollection).NewDoc()
		if err := tx.Create(reactionRef, data); err != nil {
			return fmt.Errorf("unable to create reaction: %w", err)
		}
		humanRef := d.client.Collection(d.humanCollection).Doc(input.HumanID)
		if err := tx.Update(humanRef, []firestore.Update{
			{
				Path:  fmt.Sprintf("reaction_count.%v", input.ReactionKind),
				Value: firestore.Increment(1),
			},
		}); err != nil {
			return fmt.Errorf("unable to update reaction count: %w", err)
		}
		data.ID = reactionRef.ID
		return nil
	})
	if err != nil {
		return Reaction{}, err
	}

	return data, nil
}

type ReactUndoInput struct {
	UserID     string
	ReactionID string
}

func (d *DAO) ReactUndo(ctx context.Context, input ReactUndoInput) error {
	doc := d.client.Collection(d.reactionCollection).Doc(input.ReactionID)
	reactionRef, err := doc.Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil
		}
		return fmt.Errorf("unable to find reaction by id: %v: %w", input.ReactionID, err)
	}
	reaction, err := convertReactionDoc(reactionRef)
	if err != nil {
		return err
	}

	if reaction.UserID != input.UserID {
		return ErrUnauthorized
	}

	err = d.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if err := tx.Delete(doc, firestore.Exists); err != nil {
			return fmt.Errorf("error deleting reaction: %w", err)
		}
		humanRef := d.client.Collection(d.humanCollection).Doc(reaction.HumanID)
		if err := tx.Update(humanRef, []firestore.Update{
			{
				Path:  fmt.Sprintf("reaction_count.%v", reaction.ReactionKind),
				Value: firestore.Increment(-1),
			},
		}); err != nil {
			return fmt.Errorf("error decrementing reaction count: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("error running transaction: %w", err)
	}

	return nil
}

type GetReactionsInput struct {
	UserID string
}

func (d *DAO) GetReactions(ctx context.Context, input GetReactionsInput) ([]Reaction, error) {
	docs, err := d.client.Collection(d.reactionCollection).
		Where("user_id", "==", input.UserID).
		OrderBy("created_at", firestore.Asc).
		Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("unable to get reactions for user %s: %w", input.UserID, err)
	}

	return convertReactionDocs(docs)
}

type ListHumansInput struct {
	Limit  int
	Offset int
}

func (d *DAO) ListHumans(ctx context.Context, input ListHumansInput) ([]Human, error) {
	docs, err := d.client.Collection(d.humanCollection).
		OrderBy("created_at", firestore.Desc).
		Offset(input.Offset).
		Limit(input.Limit).
		Documents(ctx).
		GetAll()
	if err != nil {
		return nil, fmt.Errorf("unable to get humans: %w", err)
	}

	return convertHumansDocs(docs)
}

type CreatedByInput struct {
	CreatedBy string
	Limit     int
	Offset    int
}

func (d *DAO) CreatedBy(ctx context.Context, input CreatedByInput) ([]Human, error) {
	docs, err := d.client.Collection(d.humanCollection).
		Where("created_by", "==", input.CreatedBy).
		OrderBy("created_at", firestore.Desc).
		Offset(input.Offset).
		Limit(input.Limit).
		Documents(ctx).
		GetAll()
	if err != nil {
		return nil, fmt.Errorf("unable to get humans: %w", err)
	}

	return convertHumansDocs(docs)
}

type UserDraftsInput struct {
	Limit  int
	Offset int
	UserID string
}

func (d *DAO) UserDrafts(ctx context.Context, input UserDraftsInput) ([]Human, error) {
	docs, err := d.client.Collection(d.humanCollection).
		Where("draft", "==", true).
		Where("created_by", "==", input.UserID).
		OrderBy("created_at", firestore.Desc).
		Offset(input.Offset).
		Limit(input.Limit).
		Documents(ctx).
		GetAll()
	if err != nil {
		return nil, fmt.Errorf("unable to get humans: %w", err)
	}

	return convertHumansDocs(docs)
}

type DraftsInput struct {
	Limit  int
	Offset int
}

func (d *DAO) Drafts(ctx context.Context, input DraftsInput) ([]Human, error) {
	docs, err := d.client.Collection(d.humanCollection).
		Where("draft", "==", true).
		OrderBy("created_at", firestore.Asc).
		Offset(input.Offset).
		Limit(input.Limit).
		Documents(ctx).
		GetAll()
	if err != nil {
		return nil, fmt.Errorf("unable to get humans: %w", err)
	}

	return convertHumansDocs(docs)
}

type PublishInput struct {
	HumanID string
	UserID  string
}

func (d *DAO) Publish(ctx context.Context, input PublishInput) error {
	now := time.Now()
	_, err := d.client.Collection(d.humanCollection).
		Doc(input.HumanID).
		Update(ctx, []firestore.Update{
			{Path: "draft", Value: false},
			{Path: "published_by", Value: input.UserID},
			{Path: "published_at", Value: now},
		})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return ErrHumanNotFound
		}
		return fmt.Errorf("unable to update human: %v: %w", input.HumanID, err)
	}

	return nil
}
