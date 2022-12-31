package humandao

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/segmentio/ksuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrHumanNotFound = errors.New("human not found")
)

type Human struct {
	ID            string               `firestore:"-"`
	Name          string               `firestore:"name"`
	Path          string               `firestore:"path"`
	ReactionCount map[ReactionKind]int `firestore:"reactionCount"`
	DOB           string               `firestore:"dob,omitempty"`
	DOD           string               `firestore:"did,omitempty"`
	Tags          []string             `firestore:"tags,omitempty"`
	Website       string               `firestore:"website,omitempty"`
	Ethnicity     []string             `firestore:"ethnicity,omitempty"`
	BirthLocation string               `firestore:"birthLocation,omitempty"`
	Location      []string             `firestore:"location,omitempty"`
	InfluencedBy  []string             `firestore:"influencedBy,omitempty"`
	Twitter       string               `firestore:"twitter,omitempty"`
	FeaturedImage string               `firestore:"featured_image,omitempty"`
	Draft         bool                 `firestore:"draft,omitempty"`
	AIGenerated   bool                 `firestore:"ai_generated,omitempty"`
	Description   string               `firestore:"description,omitempty"`

	CreatedAt time.Time `firestore:"created_at"`
	UpdatedAt time.Time `firestore:"updated_at"`
}

type Reaction struct {
	ID           string       `firestore:"-"`
	UserID       string       `firestore:"user_id,omitempty"`
	HumanID      string       `firestore:"human_id,omitempty"`
	ReactionKind ReactionKind `firestore:"reaction_kind,omitempty"`
	CreatedAt    time.Time    `firestore:"created_at,omitempty"`
}

type DAO struct {
	client             *firestore.Client
	humanCollection    string
	reactionCollection string
}

type Option func(d *DAO)

func WithHumanCollectionName(name string) Option {
	return func(d *DAO) {
		d.humanCollection = name
	}
}

func WithReactionCollectionName(name string) Option {
	return func(d *DAO) {
		d.reactionCollection = name
	}
}

func NewDAO(client *firestore.Client, options ...Option) *DAO {
	dao := &DAO{
		client:             client,
		humanCollection:    "humans",
		reactionCollection: "reactions",
	}

	for _, opt := range options {
		opt(dao)
	}

	return dao
}

type HumanInput struct {
	HumanID string
}

func (d *DAO) Human(ctx context.Context, input HumanInput) (Human, error) {
	doc, err := d.client.Collection(d.humanCollection).Doc(input.HumanID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return Human{}, ErrHumanNotFound
		}
		return Human{}, fmt.Errorf("unable to get human: %w", err)
	}
	m := doc.Data()
	human := convertHuman(m)

	human.ID = doc.Ref.ID
	return human, nil
}

func convertHuman(m map[string]interface{}) Human {
	var human Human
	reactionCount := m["reactionCount"].(map[string]interface{})
	human.ReactionCount = make(map[ReactionKind]int, len(reactionCount))
	for k, v := range reactionCount {
		human.ReactionCount[ReactionKind(k)] = int(v.(int64))
	}
	return human
}

type AddHumanInput struct {
	HumanID string
	Name    string
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

func (d *DAO) AddHuman(ctx context.Context, input AddHumanInput) (Human, error) {
	human := Human{
		Name: input.Name,
	}
	if input.HumanID == "" {
		input.HumanID = ksuid.New().String()
	}

	_, err := d.client.Collection(d.humanCollection).Doc(input.HumanID).Create(ctx, human)
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
	ReactionKindHeart ReactionKind = "❤️"
	ReactionKindFire  ReactionKind = "🔥"
	ReactionKindJoy   ReactionKind = "😂"
)
var validReactionKinds = map[ReactionKind]struct{}{
	ReactionKindHeart: {},
	ReactionKindFire:  {},
	ReactionKindJoy:   {},
}

var AllReactionKinds = []ReactionKind{ReactionKindHeart, ReactionKindFire, ReactionKindJoy}

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

	batch := d.client.Batch()
	reactionRef := d.client.Collection(d.reactionCollection).NewDoc()
	batch.Create(reactionRef, data)
	humanRef := d.client.Collection(d.humanCollection).Doc(input.HumanID)
	batch = batch.Update(humanRef, []firestore.Update{
		{
			Path:  fmt.Sprintf("reactionCount.%v", input.ReactionKind),
			Value: firestore.Increment(1),
		},
	})

	_, err := batch.Commit(ctx)
	if err != nil {
		return Reaction{}, err
	}

	data.ID = reactionRef.ID
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
	reaction, err := convertDoc(reactionRef)
	if err != nil {
		return err
	}

	if reaction.UserID != input.UserID {
		return ErrUnauthorized
	}

	batch := d.client.Batch()
	batch = batch.Delete(doc, firestore.Exists)
	humanRef := d.client.Collection(d.humanCollection).Doc(reaction.HumanID)
	batch = batch.Update(humanRef, []firestore.Update{
		{
			Path:  fmt.Sprintf("reactionCount.%v", reaction.ReactionKind),
			Value: firestore.Increment(-1),
		},
	})

	_, err = batch.Commit(ctx)
	if err != nil {
		return err
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

	return convertDocs(docs)
}

func convertDocs(docs []*firestore.DocumentSnapshot) ([]Reaction, error) {
	reactions := make([]Reaction, 0, len(docs))
	for _, doc := range docs {
		reaction, err := convertDoc(doc)
		if err != nil {
			return nil, err
		}
		reactions = append(reactions, reaction)
	}

	return reactions, nil
}

func convertDoc(doc *firestore.DocumentSnapshot) (Reaction, error) {
	var reaction Reaction
	if err := doc.DataTo(&reaction); err != nil {
		return Reaction{}, fmt.Errorf("unable to convert document to reaction: %w", err)
	}
	reaction.ID = doc.Ref.ID
	return reaction, nil
}
