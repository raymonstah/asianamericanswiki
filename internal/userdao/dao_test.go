package userdao

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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()

	client, err := firestore.NewClient(ctx, api.ProjectID)
	assert.NoError(t, err)

	userCollection := "users-" + ksuid.New().String()
	dao := NewDAO(client, WithUserCollectionName(userCollection))

	t.Cleanup(func() {
		ctx := context.Background()
		userDocs, err := client.Collection(userCollection).DocumentRefs(ctx).GetAll()
		assert.NoError(t, err)
		for _, doc := range userDocs {
			_, err := doc.Delete(ctx)
			assert.NoError(t, err)
		}
	})

	do(ctx, dao)
}

func TestDAO_RecentlyViewed(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		var (
			userID   = ksuid.New().String()
			human1ID = ksuid.New().String()
			human2ID = ksuid.New().String()
			human3ID = ksuid.New().String()
		)
		// Create an empty user
		_, err := dao.client.Collection(dao.userCollectionName).Doc(userID).Create(ctx, map[string]interface{}{})
		assert.NoError(t, err)

		user, err := dao.User(ctx, userID, WithRecentlyViewed())
		assert.NoError(t, err)
		assert.Empty(t, user.RecentlyViewed)

		err = dao.ViewHuman(ctx, ViewHumanInput{UserID: userID, HumanID: human1ID})
		assert.NoError(t, err)
		err = dao.ViewHuman(ctx, ViewHumanInput{UserID: userID, HumanID: human2ID})
		assert.NoError(t, err)
		err = dao.ViewHuman(ctx, ViewHumanInput{UserID: userID, HumanID: human3ID})
		assert.NoError(t, err)

		// Verify that the user has 3 recently viewed humans
		user, err = dao.User(ctx, userID, WithRecentlyViewed())
		assert.NoError(t, err)
		assert.Equal(t, userID, user.ID)
		assert.Len(t, user.RecentlyViewed, 3)
		assert.Equal(t, human3ID, user.RecentlyViewed[0].HumanID)
		assert.Equal(t, human2ID, user.RecentlyViewed[1].HumanID)
		assert.Equal(t, human1ID, user.RecentlyViewed[2].HumanID)

		t.Run("UserNotFound", func(t *testing.T) {
			err = dao.ViewHuman(ctx, ViewHumanInput{UserID: "notfound", HumanID: human1ID})
			assert.NoError(t, err)
		})
	})
}

func TestDAO_SaveHuman(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		var (
			userID   = ksuid.New().String()
			human1ID = ksuid.New().String()
			human2ID = ksuid.New().String()
			human3ID = ksuid.New().String()
		)
		// Create an empty user
		_, err := dao.client.Collection(dao.userCollectionName).Doc(userID).Create(ctx, map[string]interface{}{})
		assert.NoError(t, err)

		user, err := dao.User(ctx, userID, WithSaved())
		assert.NoError(t, err)
		assert.Empty(t, user.Saved)

		err = dao.SaveHuman(ctx, SaveHumanInput{UserID: userID, HumanID: human1ID})
		assert.NoError(t, err)
		err = dao.SaveHuman(ctx, SaveHumanInput{UserID: userID, HumanID: human2ID})
		assert.NoError(t, err)
		err = dao.SaveHuman(ctx, SaveHumanInput{UserID: userID, HumanID: human3ID})
		assert.NoError(t, err)

		// Verify that the user has 3 saved humans
		user, err = dao.User(ctx, userID, WithSaved())
		assert.NoError(t, err)
		assert.Equal(t, userID, user.ID)
		assert.Len(t, user.Saved, 3)
		assert.Equal(t, human3ID, user.Saved[0].HumanID)
		assert.Equal(t, human2ID, user.Saved[1].HumanID)
		assert.Equal(t, human1ID, user.Saved[2].HumanID)

		t.Run("UserNotFound", func(t *testing.T) {
			err = dao.SaveHuman(ctx, SaveHumanInput{UserID: "notfound", HumanID: human1ID})
			assert.NoError(t, err)
		})
	})
}

func TestDAO_SaveHuman_IgnoreDupe(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		var (
			userID   = ksuid.New().String()
			human1ID = ksuid.New().String()
		)
		// Create an empty user
		_, err := dao.client.Collection(dao.userCollectionName).Doc(userID).Create(ctx, map[string]interface{}{})
		assert.NoError(t, err)

		user, err := dao.User(ctx, userID, WithSaved())
		assert.NoError(t, err)
		assert.Empty(t, user.Saved)

		err = dao.SaveHuman(ctx, SaveHumanInput{UserID: userID, HumanID: human1ID})
		assert.NoError(t, err)

		err = dao.SaveHuman(ctx, SaveHumanInput{UserID: userID, HumanID: human1ID})
		assert.NoError(t, err)

		// Verify that the user has 3 saved humans
		user, err = dao.User(ctx, userID, WithSaved())
		assert.NoError(t, err)
		assert.Equal(t, userID, user.ID)
		assert.Len(t, user.Saved, 1)
		assert.Equal(t, human1ID, user.Saved[0].HumanID)
	})
}

func TestDAO_SaveHuman_UnsaveHuman(t *testing.T) {
	WithDAO(t, func(ctx context.Context, dao *DAO) {
		var (
			userID   = ksuid.New().String()
			human1ID = ksuid.New().String()
		)
		// Create an empty user
		_, err := dao.client.Collection(dao.userCollectionName).Doc(userID).Create(ctx, map[string]interface{}{})
		assert.NoError(t, err)

		err = dao.SaveHuman(ctx, SaveHumanInput{UserID: userID, HumanID: human1ID})
		assert.NoError(t, err)

		err = dao.UnsaveHuman(ctx, UnsaveHumanInput{UserID: userID, HumanID: human1ID})
		assert.NoError(t, err)

		user, err := dao.User(ctx, userID, WithSaved())
		assert.NoError(t, err)
		assert.Empty(t, user.Saved)
	})
}

func TestDAO_ViewHuman_IgnoreDupes(t *testing.T) {
	var (
		userID  = ksuid.New().String()
		humanID = ksuid.New().String()
	)

	WithDAO(t, func(ctx context.Context, dao *DAO) {
		// Create an empty user
		_, err := dao.client.Collection(dao.userCollectionName).Doc(userID).Create(ctx, map[string]interface{}{})
		assert.NoError(t, err)

		user, err := dao.User(ctx, userID, WithRecentlyViewed())
		assert.NoError(t, err)
		assert.Empty(t, user.Saved)

		for i := 0; i < 10; i++ {
			err := dao.ViewHuman(ctx, ViewHumanInput{UserID: userID, HumanID: humanID})
			assert.NoError(t, err)
		}

		user, err = dao.User(ctx, userID, WithRecentlyViewed())
		assert.NoError(t, err)
		assert.Len(t, user.RecentlyViewed, 1)

	})
}
