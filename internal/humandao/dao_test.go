package humandao

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/segmentio/ksuid"
	"github.com/tj/assert"

	"github.com/raymonstah/asianamericanswiki/functions/api"
)

func TestMain(m *testing.M) {
	if err := os.Setenv("FIRESTORE_EMULATOR_HOST", "localhost:8080"); err != nil {
		log.Fatal("failed to set FIRESTORE_EMULATOR_HOST environment variable", err)
	}
	m.Run()
}

func WithDAO(t *testing.T, do func(ctx context.Context, dao *DAO)) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	client, err := firestore.NewClient(ctx, api.ProjectID)
	assert.NoError(t, err)

	humanCollection := ksuid.New().String()
	reactionCollection := ksuid.New().String()
	dao := NewDAO(client, WithHumanCollectionName(humanCollection), WithReactionCollectionName(reactionCollection))

	t.Cleanup(func() {
		ctx := context.Background()
		humanDocs, err := client.Collection(humanCollection).DocumentRefs(ctx).GetAll()
		assert.NoError(t, err)
		reactionDocs, err := client.Collection(reactionCollection).DocumentRefs(ctx).GetAll()
		assert.NoError(t, err)
		docs := append(humanDocs, reactionDocs...)
		for _, doc := range docs {
			_, err := doc.Delete(ctx)
			assert.NoError(t, err)
		}

	})

	do(ctx, dao)
}

func TestDAO(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		human, err := dao.AddHuman(ctx, AddHumanInput{})
		assert.NoError(t, err)

		n := 100
		var reactions []Reaction
		for i := 0; i < n; i++ {
			reaction, err := dao.React(ctx, ReactInput{UserID: "abc", HumanID: human.ID, ReactionKind: ReactionKindFire})
			assert.NoError(t, err)
			reactions = append(reactions, reaction)
		}

		human, err = dao.Human(ctx, HumanInput{HumanID: human.ID})
		assert.NoError(t, err)
		assert.Equal(t, n, human.ReactionCount[ReactionKindFire])

		for _, reaction := range reactions {
			err = dao.ReactUndo(ctx, ReactUndoInput{UserID: "abc", ReactionID: reaction.ID})
			assert.NoError(t, err)
		}

		human, err = dao.Human(ctx, HumanInput{HumanID: human.ID})
		assert.NoError(t, err)
		assert.Equal(t, 0, human.ReactionCount[ReactionKindFire])
	})
}

func TestDAOReactions(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		human, err := dao.AddHuman(ctx, AddHumanInput{})
		assert.NoError(t, err)

		userID := "user123"
		reaction, err := dao.React(ctx, ReactInput{
			UserID:       userID,
			HumanID:      human.ID,
			ReactionKind: ReactionKindFire,
		})
		assert.NoError(t, err)
		assert.NotZero(t, reaction.ID)

		reactions, err := dao.GetReactions(ctx, GetReactionsInput{UserID: userID})
		assert.NoError(t, err)
		assert.Len(t, reactions, 1)

		err = dao.ReactUndo(ctx, ReactUndoInput{UserID: userID, ReactionID: reaction.ID})
		assert.NoError(t, err)

		reactions, err = dao.GetReactions(ctx, GetReactionsInput{UserID: userID})
		assert.NoError(t, err)
		assert.Len(t, reactions, 0)
	})
}

func TestDAO_HumanNotFound(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		human, err := dao.Human(ctx, HumanInput{HumanID: "human123"})
		assert.EqualError(t, err, "human not found")
		assert.Zero(t, human)
	})
}

func TestDAO_ReactionNotFound(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		err := dao.ReactUndo(ctx, ReactUndoInput{UserID: "user123", ReactionID: "fake-reaction-id"})
		assert.NoError(t, err)
	})
}

func TestDAO_ReactionUndo_Unauthorized(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		human, err := dao.AddHuman(ctx, AddHumanInput{})
		assert.NoError(t, err)

		userID := "user123"
		reaction, err := dao.React(ctx, ReactInput{
			UserID:       userID,
			HumanID:      human.ID,
			ReactionKind: ReactionKindFire,
		})
		assert.NoError(t, err)
		assert.NotZero(t, reaction.ID)

		err = dao.ReactUndo(ctx, ReactUndoInput{UserID: "fake-user", ReactionID: reaction.ID})
		assert.EqualError(t, err, "user is not authorized to perform this operation")
	})
}
