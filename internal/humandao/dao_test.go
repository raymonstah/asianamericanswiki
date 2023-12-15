package humandao

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/segmentio/ksuid"
	"github.com/tj/assert"
	"golang.org/x/sync/errgroup"

	"github.com/raymonstah/asianamericanswiki/functions/api"
)

func TestMain(m *testing.M) {
	if err := os.Setenv("FIRESTORE_EMULATOR_HOST", "localhost:8080"); err != nil {
		log.Fatal("failed to set FIRESTORE_EMULATOR_HOST environment variable", err)
	}
	m.Run()
}

func WithDAO(t *testing.T, do func(ctx context.Context, dao *DAO)) {
	t.Parallel()
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
		human, err := dao.AddHuman(ctx, AddHumanInput{Name: "Raymond", Gender: GenderMale})
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
		assert.Equal(t, n, human.ReactionCount[string(ReactionKindFire)])

		for _, reaction := range reactions {
			err = dao.ReactUndo(ctx, ReactUndoInput{UserID: "abc", ReactionID: reaction.ID})
			assert.NoError(t, err)
		}

		human, err = dao.Human(ctx, HumanInput{HumanID: human.ID})
		assert.NoError(t, err)
		assert.Equal(t, 0, human.ReactionCount[string(ReactionKindFire)])
	})
}

func TestDAOReactions(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		human, err := dao.AddHuman(ctx, AddHumanInput{Name: "Raymond", Gender: GenderMale})
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
		assert.EqualError(t, err, "human not found: human123")
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
		human, err := dao.AddHuman(ctx, AddHumanInput{Name: "Raymond", Gender: GenderMale})
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

func TestDAO_CreatedBy(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		userID := "user123"
		human, err := dao.AddHuman(ctx, AddHumanInput{Name: "Raymond", CreatedBy: userID, Gender: GenderMale})
		assert.NoError(t, err)

		t.Run("has-something-created", func(t *testing.T) {
			humans, err := dao.CreatedBy(ctx, CreatedByInput{
				CreatedBy: userID,
				Limit:     10,
				Offset:    0,
			})
			assert.NoError(t, err)
			assert.Len(t, humans, 1)
			got := humans[0]
			assert.Equal(t, human.ID, got.ID)
		})
		t.Run("has-nothing-created", func(t *testing.T) {
			humans, err := dao.CreatedBy(ctx, CreatedByInput{
				CreatedBy: "random-user",
				Limit:     10,
				Offset:    0,
			})
			assert.NoError(t, err)
			assert.Empty(t, humans)
		})
	})
}

func TestDAO_Drafts(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		userID := "user123"
		human, err := dao.AddHuman(ctx, AddHumanInput{Name: "Raymond", CreatedBy: userID, Draft: true, Gender: GenderMale})
		assert.NoError(t, err)
		_, err = dao.AddHuman(ctx, AddHumanInput{Name: "Foo", CreatedBy: userID, Draft: false, Gender: GenderFemale})
		assert.NoError(t, err)

		humans, err := dao.Drafts(ctx, DraftsInput{
			Limit:  10,
			Offset: 0,
		})
		assert.NoError(t, err)
		assert.Len(t, humans, 1)
		got := humans[0]
		assert.Equal(t, human.ID, got.ID)
	})
}

func TestDAO_UserDrafts(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		userID := "user123"
		human, err := dao.AddHuman(ctx, AddHumanInput{Name: "Raymond", CreatedBy: userID, Draft: true, Gender: GenderMale})
		assert.NoError(t, err)
		_, err = dao.AddHuman(ctx, AddHumanInput{Name: "Foo", CreatedBy: userID, Draft: false, Gender: GenderFemale})
		assert.NoError(t, err)

		humans, err := dao.UserDrafts(ctx, UserDraftsInput{
			UserID: userID,
			Limit:  10,
			Offset: 0,
		})
		assert.NoError(t, err)
		assert.Len(t, humans, 1)

		got := humans[0]
		assert.Equal(t, human.ID, got.ID)

		humans, err = dao.UserDrafts(ctx, UserDraftsInput{
			UserID: "fake-user",
			Limit:  10,
			Offset: 0,
		})
		assert.NoError(t, err)
		assert.Empty(t, humans)
	})
}

func TestDAO_Publish(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		t.Run("invalid-human-id", func(t *testing.T) {
			err := dao.Publish(ctx, PublishInput{
				HumanID: "foo-bar",
				UserID:  "user-123",
			})
			assert.Equal(t, ErrHumanNotFound, err)
		})
		userID := "user123"
		human, err := dao.AddHuman(ctx, AddHumanInput{Name: "Raymond", CreatedBy: userID, Draft: true, Gender: GenderMale})
		assert.NoError(t, err)
		assert.True(t, human.Draft)

		err = dao.Publish(ctx, PublishInput{
			HumanID: human.ID,
			UserID:  userID,
		})
		assert.NoError(t, err)
		human, err = dao.Human(ctx, HumanInput{HumanID: human.ID})
		assert.NoError(t, err)
		assert.False(t, human.Draft)
		assert.Equal(t, userID, human.PublishedBy)
	})
}

func TestDAO_List(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		userID := "user123"
		n := 10
		for i := 0; i < n; i++ {
			_, err := dao.AddHuman(ctx, AddHumanInput{
				Name:      fmt.Sprintf("%v", ksuid.New().String()),
				Draft:     false,
				CreatedBy: userID,
				Gender:    GenderFemale,
			})
			assert.NoError(t, err)
		}
		_, err := dao.AddHuman(ctx, AddHumanInput{
			Name:      fmt.Sprintf("%v", ksuid.New().String()),
			Draft:     true,
			CreatedBy: userID,
			Gender:    GenderFemale,
		})
		assert.NoError(t, err)
		n++

		humans, err := dao.ListHumans(ctx, ListHumansInput{
			Limit:  n,
			Offset: 0,
		})
		assert.NoError(t, err)
		assert.Len(t, humans, n-1)
	})
}

func TestDAO_List_Paginate(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		userID := "user123"
		n := 100
		ids := make([]string, 0, n)
		for i := 0; i < n; i++ {
			human, err := dao.AddHuman(ctx, AddHumanInput{
				Name:      fmt.Sprintf("%v", ksuid.New().String()),
				Draft:     false,
				CreatedBy: userID,
				Gender:    GenderFemale,
			})
			assert.NoError(t, err)
			ids = append(ids, human.ID)
		}

		for i := 0; i < n; i += 10 {
			humans, err := dao.ListHumans(ctx, ListHumansInput{
				Limit:  n / 10,
				Offset: i,
			})

			assert.NoError(t, err)
			assert.Len(t, humans, n/10)
			// we got them in descending order
			for idx, human := range humans {
				reverseIdx := n - i - idx - 1
				assert.Equal(t, ids[reverseIdx], human.ID)
			}
		}
	})
}

func TestDAO_Delete(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		human, err := dao.AddHuman(ctx, AddHumanInput{Name: "Foo Bar", Gender: GenderFemale})
		assert.NoError(t, err)
		gotHuman, err := dao.Human(ctx, HumanInput{HumanID: human.ID})
		assert.NoError(t, err)
		assert.Equal(t, human.ID, gotHuman.ID)

		err = dao.Delete(ctx, DeleteInput{HumanID: human.ID})
		assert.NoError(t, err)

		_, err = dao.Human(ctx, HumanInput{HumanID: human.ID})
		assert.EqualError(t, err, fmt.Sprintf("human not found: %v", human.ID))
	})
}

func TestDAO_HumansByID(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		var ids []string
		numHumans := 10
		for i := 0; i < numHumans; i++ {
			human, err := dao.AddHuman(ctx, AddHumanInput{Name: fmt.Sprintf("Human-%v", i), Gender: GenderFemale})
			assert.NoError(t, err)
			ids = append(ids, human.ID)
		}

		humans, err := dao.HumansByID(ctx, HumansByIDInput{HumanIDs: ids})
		assert.NoError(t, err)
		assert.Len(t, humans, numHumans)
		gotIDs := make([]string, 0, len(humans))

		for _, human := range humans {
			gotIDs = append(gotIDs, human.ID)
		}

		assert.Equal(t, ids, gotIDs)
	})
}

func TestDAO_Affiliates(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		assertions := func(human Human, err error) {
			assert.NoError(t, err)
			assert.Len(t, human.Affiliates, 3)
			assert.NotEmpty(t, human.Affiliates[0].ID)
			assert.NotEqual(t, human.Affiliates[0].ID, human.Affiliates[1].ID)
			assert.NotEqual(t, human.Affiliates[1].ID, human.Affiliates[2].ID)
		}

		human, err := dao.AddHuman(ctx, AddHumanInput{
			Name:   "Human",
			Gender: GenderFemale,
			Affiliates: []Affiliate{
				{URL: "https://url.com/1"},
				{URL: "https://url.com/2"},
				{URL: "https://url.com/3"},
			}})

		assertions(human, err)

		t.Run("find-should-include-affiliates-too", func(t *testing.T) {
			human, err := dao.Human(ctx, HumanInput{HumanID: human.ID})
			assertions(human, err)
		})
	})
}

func TestDAO_View(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		human, err := dao.AddHuman(ctx, AddHumanInput{Name: "Foo Bar", Gender: GenderFemale})
		assert.NoError(t, err)
		n := 100
		group, ctx := errgroup.WithContext(ctx)
		group.SetLimit(16)
		for i := 0; i < n; i++ {
			group.Go(func() error {
				err := dao.View(ctx, ViewInput{human.ID})
				assert.NoError(t, err)
				return nil
			})
		}
		err = group.Wait()
		assert.NoError(t, err)

		gotHuman, err := dao.Human(context.TODO(), HumanInput{HumanID: human.ID})
		assert.NoError(t, err)
		assert.EqualValues(t, n, gotHuman.Views)
	})
}

func TestDAO_MostViewed(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		n := 15
		for i := 0; i < n; i++ {
			human, err := dao.AddHuman(ctx, AddHumanInput{Name: fmt.Sprintf("Human %v", i), Gender: GenderFemale})
			assert.NoError(t, err)
			human.Views = int64(i)
			err = dao.UpdateHuman(ctx, human)
			assert.NoError(t, err)
		}

		humans, err := dao.ListHumans(ctx, ListHumansInput{
			Limit:     10,
			Offset:    0,
			OrderBy:   "views",
			Direction: firestore.Desc,
		})
		assert.NoError(t, err)
		for i, human := range humans {
			assert.EqualValues(t, n-i-1, human.Views)
		}
	})
}
