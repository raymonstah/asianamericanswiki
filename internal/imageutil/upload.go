package imageutil

import (
	"bytes"
	"context"
	"fmt"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"

	"cloud.google.com/go/storage"
	"github.com/disintegration/imaging"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
)

type Uploader struct {
	storageClient *storage.Client
	humanDAO      *humandao.DAO
	storageURL    string
}

func NewUploader(storageClient *storage.Client, humanDAO *humandao.DAO, storageURL string) *Uploader {
	return &Uploader{
		storageClient: storageClient,
		humanDAO:      humanDAO,
		storageURL:    storageURL,
	}
}

func (u *Uploader) UploadHumanImages(ctx context.Context, human humandao.Human, rawImage []byte) (humandao.Human, error) {
	// Generate thumbnail
	src, err := imaging.Decode(bytes.NewReader(rawImage))
	if err != nil {
		return human, fmt.Errorf("unable to decode image for thumbnail: %w", err)
	}

	thumb := imaging.Thumbnail(src, 256, 256, imaging.Lanczos)
	thumb = imaging.Sharpen(thumb, 0.5)

	var thumbBuf bytes.Buffer
	if err := jpeg.Encode(&thumbBuf, thumb, &jpeg.Options{Quality: 95}); err != nil {
		return human, fmt.Errorf("unable to encode thumbnail to jpeg: %w", err)
	}
	thumbRaw := thumbBuf.Bytes()

	// Upload to GCS
	objectID := fmt.Sprintf("%s/original.webp", human.ID)
	obj := u.storageClient.Bucket(api.ImagesStorageBucket).Object(objectID)
	writer := obj.NewWriter(ctx)
	if _, err := writer.Write(rawImage); err != nil {
		return human, fmt.Errorf("unable to upload original image: %w", err)
	}
	if err := writer.Close(); err != nil {
		return human, err
	}

	thumbObjectID := fmt.Sprintf("%s/thumbnail.webp", human.ID)
	thumbObj := u.storageClient.Bucket(api.ImagesStorageBucket).Object(thumbObjectID)
	thumbWriter := thumbObj.NewWriter(ctx)
	if _, err := thumbWriter.Write(thumbRaw); err != nil {
		return human, fmt.Errorf("unable to upload thumbnail: %w", err)
	}
	if err := thumbWriter.Close(); err != nil {
		return human, err
	}

	human.Images.Featured = fmt.Sprintf("%v/%v/%s", u.storageURL, api.ImagesStorageBucket, objectID)
	human.Images.Thumbnail = fmt.Sprintf("%v/%v/%s", u.storageURL, api.ImagesStorageBucket, thumbObjectID)
	
	// If it was already AI generated, keep it true. 
	// The caller can set it to true if they know it's AI generated.
	if err := u.humanDAO.UpdateHuman(ctx, human); err != nil {
		return human, fmt.Errorf("unable to update human with image URLs: %w", err)
	}

	return human, nil
}
