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
	dao := NewDAO(client, WithHumanCollectionName(humanCollection))

	t.Cleanup(func() {
		ctx := context.Background()
		humanDocs, err := client.Collection(humanCollection).DocumentRefs(ctx).GetAll()
		assert.NoError(t, err)
		for _, doc := range humanDocs {
			_, err := doc.Delete(ctx)
			assert.NoError(t, err)
		}
	})

	do(ctx, dao)
}

func TestDAO_HumanNotFound(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		human, err := dao.Human(ctx, HumanInput{HumanID: "human123"})
		assert.EqualError(t, err, "human not found: human123")
		assert.Zero(t, human)
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

func TestHumanAge(t *testing.T) {
	tcs := map[string]struct {
		Human       Human
		ExpectedAge string
	}{
		"no-dob":                        {ExpectedAge: ""},
		"no-dod-partial-dob-year":       {Human: Human{DOB: "2000"}, ExpectedAge: "24 y/o"},
		"no-dod-partial-dob-year-month": {Human: Human{DOB: "1999-12"}, ExpectedAge: "24 y/o"},
		"no-dod-full-dob":               {Human: Human{DOB: "2000-01-01"}, ExpectedAge: "24 y/o"},
		"full-dod-full-dob":             {Human: Human{DOB: "1940-11-27", DOD: "1973-07-20"}, ExpectedAge: "died at 32 y/o"},
	}

	date20240315 := time.Date(2024, 3, 15, 0, 0, 0, 0, time.Local)

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			age, err := tc.Human.CurrentAge(date20240315)
			assert.NoError(t, err)
			assert.Equal(t, tc.ExpectedAge, age)
		})
	}
}
