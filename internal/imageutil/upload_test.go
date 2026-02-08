package imageutil

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"testing"
	"time"

	"bytes"
	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/segmentio/ksuid"
	"github.com/tj/assert"
)

func TestMain(m *testing.M) {
	if err := os.Setenv("FIRESTORE_EMULATOR_HOST", "localhost:8080"); err != nil {
		panic(err)
	}
	if err := os.Setenv("STORAGE_EMULATOR_HOST", "localhost:9199"); err != nil {
		panic(err)
	}
	m.Run()
}

func TestUploader_UploadHumanImages(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Setup Firestore
	fsClient, err := firestore.NewClient(ctx, api.ProjectID)
	assert.NoError(t, err)
	humanCollection := ksuid.New().String()
	dao := humandao.NewDAO(fsClient, humandao.WithHumanCollectionName(humanCollection))

	// Setup Storage
	storageClient, err := storage.NewClient(ctx)
	assert.NoError(t, err)

	uploader := NewUploader(storageClient, dao)

	// Create a test human
	human, err := dao.AddHuman(ctx, humandao.AddHumanInput{
		Name:   "Test Person",
		Gender: humandao.GenderNonBinary,
	})
	assert.NoError(t, err)

	// Create a dummy image
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{255, 0, 0, 255}}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	err = png.Encode(&buf, img)
	assert.NoError(t, err)
	rawImage := buf.Bytes()

	// Upload
	updatedHuman, err := uploader.UploadHumanImages(ctx, human, rawImage)
	assert.NoError(t, err)

	// Verify URLs
	assert.NotEmpty(t, updatedHuman.Images.Featured)
	assert.NotEmpty(t, updatedHuman.Images.Thumbnail)
	assert.Contains(t, updatedHuman.Images.Featured, human.ID)
	assert.Contains(t, updatedHuman.Images.Thumbnail, human.ID)

	// Verify storage objects exist
	bucket := storageClient.Bucket(api.ImagesStorageBucket)
	
	// Check original
	_, err = bucket.Object(human.ID + "/original.webp").Attrs(ctx)
	assert.NoError(t, err)

	// Check thumbnail
	_, err = bucket.Object(human.ID + "/thumbnail.webp").Attrs(ctx)
	assert.NoError(t, err)

	// Verify Firestore record updated
	gotHuman, err := dao.Human(ctx, humandao.HumanInput{HumanID: human.ID})
	assert.NoError(t, err)
	assert.Equal(t, updatedHuman.Images.Featured, gotHuman.Images.Featured)
	assert.Equal(t, updatedHuman.Images.Thumbnail, gotHuman.Images.Thumbnail)
}
