package imguploaded

import (
	"context"
	"fmt"
	"log"
	"strings"

	"cloud.google.com/go/firestore"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/googleapis/google-cloudevents-go/cloud/storagedata"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"google.golang.org/protobuf/encoding/protojson"
)

func init() {
	// Register a CloudEvent function with the Functions Framework
	functions.CloudEvent("ImgUploaded", ImgUploaded)
}

func ImgUploaded(ctx context.Context, e event.Event) error {
	log.Printf("Event ID: %s", e.ID())
	log.Printf("Event Type: %s", e.Type())

	var data storagedata.StorageObjectData
	if err := protojson.Unmarshal(e.Data(), &data); err != nil {
		return fmt.Errorf("protojson.Unmarshal: %w", err)
	}

	log.Printf("Bucket: %s", data.GetBucket())
	log.Printf("File: %s", data.GetName())
	log.Printf("Metageneration: %d", data.GetMetageneration())
	log.Printf("Created: %s", data.GetTimeCreated().AsTime())
	log.Printf("Updated: %s", data.GetUpdated().AsTime())

	fsClient, err := firestore.NewClient(ctx, firestore.DetectProjectID)
	if err != nil {
		return fmt.Errorf("firestore.NewClient: %w", err)
	}
	humanDAO := humandao.NewDAO(fsClient)
	handler := Handler{humanDAO: humanDAO}
	return handler.do(ctx, data.GetBucket(), data.GetName())
}

type Handler struct {
	humanDAO *humandao.DAO
}

func (h Handler) do(ctx context.Context, bucket, fileName string) error {
	// parse the file name to get the human id, in the shape of {id}.jpg
	index := strings.Index(fileName, ".")
	if index == -1 {
		return fmt.Errorf("unable to parse file name: %s", fileName)
	}
	humanID := fileName[:index]
	human, err := h.humanDAO.Human(ctx, humandao.HumanInput{
		HumanID: humanID,
	})
	if err != nil {
		return fmt.Errorf("humanDAO.Human: %w", err)
	}

	if human.FeaturedImage != "" {
		log.Printf("Human %v (%v) already has an image", human.Name, human.ID)
		return nil
	}

	// update the human with the image url
	human.FeaturedImage = fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucket, fileName)
	log.Printf("Updating %v (%v) with image url: %v", human.Name, human.ID, human.FeaturedImage)
	if err := h.humanDAO.UpdateHuman(ctx, human); err != nil {
		return fmt.Errorf("humanDAO.UpdateHuman: %w", err)
	}

	return nil
}
