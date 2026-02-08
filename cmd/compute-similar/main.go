package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"cloud.google.com/go/firestore"
	"github.com/google/generative-ai-go/genai"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/urfave/cli/v3"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/filters"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
	"google.golang.org/api/option"
)

// https://ai.google.dev/gemini-api/docs/models/gemini#model-variations
const generativeModelName = "gemini-1.5-flash"
const embeddingModelName = "text-embedding-004"

var opts struct {
	Dry          bool
	GeminiAPIKey string
}

func main() {
	cmd := &cli.Command{
		Name: "A CLI tool to find Asian Americans who are similar.",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "dry", Destination: &opts.Dry},
			&cli.StringFlag{
				Name:        "gemini-api-key",
				Required:    true,
				Destination: &opts.GeminiAPIKey,
				Sources:     cli.EnvVars("GEMINI_API_KEY"),
			},
		},
		Action: action,
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func action(ctx context.Context, cmd *cli.Command) error {
	fsClient, err := firestore.NewClient(ctx, api.ProjectID)
	if err != nil {
		return fmt.Errorf("unable to create firestore client: %w", err)
	}
	humanDAO := humandao.NewDAO(fsClient)

	genaiClient, err := genai.NewClient(ctx, option.WithAPIKey(opts.GeminiAPIKey))
	if err != nil {
		return fmt.Errorf("unable to create genai client: %w", err)
	}

	weaviateClient, err := initWeaviate(ctx)
	if err != nil {
		return fmt.Errorf("unable to initiate weaviate client: %w", err)
	}

	handler := Handler{
		fsClient:       fsClient,
		humanDAO:       humanDAO,
		genModel:       genaiClient.GenerativeModel(generativeModelName),
		embModel:       genaiClient.EmbeddingModel(embeddingModelName),
		weaviateClient: weaviateClient,
	}

	humans, err := humanDAO.ListHumans(ctx, humandao.ListHumansInput{
		Limit: 500,
	})
	if err != nil {
		return fmt.Errorf("unable to list humans: %w", err)
	}

	if err := handler.build(ctx, humans); err != nil {
		return fmt.Errorf("unable to build index of humans: %w", err)
	}

	for _, human := range humans {
		results, err := handler.findSimilar(ctx, human, 3)
		if err != nil {
			return fmt.Errorf("unable to find similar humans: %w", err)
		}

		fmt.Println("Found similar humans for", results.SourceHuman.Name)
		ids := make([]string, 0, len(results.SimilarHumans))
		for _, similarHuman := range results.SimilarHumans {
			ids = append(ids, similarHuman.ID)
			fmt.Printf("\t%s\n", similarHuman.Name)
		}

		human.Similar = ids
		if opts.Dry {
			continue
		}
		if err := humanDAO.UpdateHuman(ctx, human); err != nil {
			return fmt.Errorf("unable to update human: %w", err)
		}
		fmt.Printf("Updated %s - %s\n", human.Name, human.ID)
	}
	return nil

}

func initWeaviate(ctx context.Context) (*weaviate.Client, error) {
	weaviateClient, err := weaviate.NewClient(weaviate.Config{
		Host:   "127.0.0.1:9035",
		Scheme: "http",
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create weaviate client: %w", err)
	}

	// purge existing data
	err = weaviateClient.Schema().ClassDeleter().WithClassName("Human").Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to delete all objects in weaviate: %w", err)
	}

	// Define a schema for a class "Product"
	classObj := &models.Class{
		Class: "Human",
		Properties: []*models.Property{
			{
				Name:     "ksuid",
				DataType: []string{"string"},
			},
			{
				Name:     "name",
				DataType: []string{"string"},
			},
		},
	}

	// Add the schema to Weaviate
	err = weaviateClient.Schema().ClassCreator().WithClass(classObj).Do(context.Background())
	if err != nil {
		log.Fatalf("Failed to create schema: %v", err)
	}

	return weaviateClient, nil
}

type Handler struct {
	fsClient       *firestore.Client
	humanDAO       *humandao.DAO
	weaviateClient *weaviate.Client
	genModel       *genai.GenerativeModel
	embModel       *genai.EmbeddingModel

	allHumansByID          map[string]humandao.Human // internal ksuid -> human
	internalIDToWeaviateID map[string]string         // internal ksuid -> weaviate id
}

type SimilarResults struct {
	SourceHuman   humandao.Human
	SimilarHumans []humandao.Human
}

func (h *Handler) build(ctx context.Context, humans []humandao.Human) error {
	h.allHumansByID = make(map[string]humandao.Human, len(humans))
	for _, human := range humans {
		h.allHumansByID[human.ID] = human
	}
	h.internalIDToWeaviateID = make(map[string]string, len(humans))

	// split docs into batches of 100
	humansByBatch := make([][]humandao.Human, 0, (len(humans)+99)/100)
	for i := 0; i < len(humans); i += 100 {
		end := min(i+100, len(humans))
		humansByBatch = append(humansByBatch, humans[i:end])
	}

	for _, humans := range humansByBatch {
		// Use the batch embedding API to embed all documents at once.
		batch := h.embModel.NewBatch()
		for _, human := range humans {
			batch.AddContent(
				// Name suggests people with similar names -- not ideal.
				// genai.Text(fmt.Sprintf("Name: %s", human.Name)),
				genai.Text(fmt.Sprintf("Gender: %s", human.Gender)),
				genai.Text(fmt.Sprintf("Ethnicity: %s", strings.Join(human.Ethnicity, ","))),
				genai.Text(fmt.Sprintf("Tags: %s", strings.Join(human.Tags, ","))),
				genai.Text(fmt.Sprintf("Location: %s", strings.Join(human.Location, ","))),
				genai.Text(human.Description),
			)
		}

		log.Printf("invoking embedding model with %v documents", len(humans))
		rsp, err := h.embModel.BatchEmbedContents(ctx, batch)
		if err != nil {
			return err
		}
		if len(rsp.Embeddings) != len(humans) {
			return fmt.Errorf("embedded batch size mismatch")
		}

		// Convert our documents - along with their embedding vectors - into types
		// used by the Weaviate client library.
		objects := make([]*models.Object, 0, len(humans))
		for i, human := range humans {
			objects = append(objects, &models.Object{
				Class: "Human",
				Properties: map[string]any{
					"ksuid": human.ID,
					"name":  human.Name,
				},
				Vector: rsp.Embeddings[i].Values,
			})
		}

		// Store documents with embeddings in the Weaviate DB.
		log.Printf("storing %v objects in weaviate", len(objects))
		resp, err := h.weaviateClient.Batch().ObjectsBatcher().WithObjects(objects...).Do(ctx)
		if err != nil {
			return err
		}
		for _, r := range resp {
			fmt.Println(r.ID, r.Properties.(map[string]any)["name"])
			h.internalIDToWeaviateID[r.Properties.(map[string]any)["ksuid"].(string)] = r.ID.String()
		}
	}
	return nil

}

func (h *Handler) findSimilar(ctx context.Context, human humandao.Human, count int) (SimilarResults, error) {
	// find similar humans
	weaviateHumanID := h.internalIDToWeaviateID[human.ID]
	response, err := h.weaviateClient.GraphQL().Get().
		WithClassName("Human").
		WithFields(
			graphql.Field{Name: "ksuid"},
		).
		WithNearObject(h.weaviateClient.GraphQL().NearObjectArgBuilder().
			WithID(weaviateHumanID)).
		WithWhere(filters.Where().
			WithPath([]string{"id"}).
			WithOperator(filters.NotEqual).
			WithValueString(weaviateHumanID)).
		WithLimit(count).
		Do(ctx)
	if err != nil {
		return SimilarResults{}, fmt.Errorf("unable to invoke weaviate for find near human: %w", err)
	}

	data, ok := response.Data["Get"]
	if !ok {
		return SimilarResults{}, fmt.Errorf("get key not found in result")
	}
	doc, ok := data.(map[string]any)
	if !ok {
		return SimilarResults{}, fmt.Errorf("get key unexpected type")
	}

	humanResponse, ok := doc["Human"].([]any)
	if !ok {
		return SimilarResults{}, fmt.Errorf("human is not a list of results")
	}

	var similarHumans []humandao.Human
	for _, human := range humanResponse {
		m, ok := human.(map[string]any)
		if !ok {
			return SimilarResults{}, fmt.Errorf("human is not a map")
		}
		similarHumanID := m["ksuid"].(string)
		human, ok := h.allHumansByID[similarHumanID]
		if !ok {
			return SimilarResults{}, fmt.Errorf("human %s not found", similarHumanID)
		}
		similarHumans = append(similarHumans, human)
	}

	return SimilarResults{
		SourceHuman:   human,
		SimilarHumans: similarHumans,
	}, nil
}
