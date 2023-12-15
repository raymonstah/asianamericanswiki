package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/segmentio/ksuid"
	"github.com/urfave/cli/v2"
	"gopkg.in/loremipsum.v1"
)

var opts struct {
	N       int
	UseProd bool
	Dry     bool
}

func main() {
	app := &cli.App{
		Name: "A CLI tool to make a generate humans for testing purposes.",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "n", Usage: "how many humans to generate", Destination: &opts.N},
			&cli.BoolFlag{Name: "use-prod", Usage: "pull data from production", Destination: &opts.UseProd},
			&cli.BoolFlag{Name: "dry", Usage: "dry run", Destination: &opts.Dry},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type Handler struct {
	localFirestore *firestore.Client
	prodFirestore  *firestore.Client
}

func run(c *cli.Context) error {
	ctx := c.Context
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: api.ProjectID})
	if err != nil {
		return fmt.Errorf("failed to create firebase app: %w", err)
	}

	if err := os.Setenv("FIRESTORE_EMULATOR_HOST", "localhost:8080"); err != nil {
		return err
	}

	localFirestore, err := app.Firestore(ctx)
	if err != nil {
		return fmt.Errorf("failed to create local firestore client: %w", err)
	}

	// unset so that we can create a client pointed to the real firestore servers.
	if err := os.Unsetenv("FIRESTORE_EMULATOR_HOST"); err != nil {
		return err
	}

	prodFirestore, err := app.Firestore(ctx)
	if err != nil {
		return fmt.Errorf("failed to create production firestore client: %w", err)
	}

	h := Handler{
		localFirestore: localFirestore,
		prodFirestore:  prodFirestore,
	}

	if err := h.Do(ctx); err != nil {
		return err
	}

	return nil
}

func (h *Handler) Do(ctx context.Context) error {
	if opts.UseProd {
		localSnapshots, err := h.localFirestore.Collection("humans").Documents(ctx).GetAll()
		if err != nil {
			return fmt.Errorf("unable to get local documents: %w", err)
		}

		if len(localSnapshots) > 0 {
			return fmt.Errorf("local firestore is not empty")
		}

		snapshots, err := h.prodFirestore.Collection("humans").Documents(ctx).GetAll()
		if err != nil {
			return fmt.Errorf("unable to get production documents: %w", err)
		}
		for i, snapshot := range snapshots {
			data := snapshot.Data()
			if opts.Dry {
				fmt.Printf("would add %v: %v\n", i, data["name"])
				continue
			}

			_, err := h.localFirestore.Collection("humans").Doc(snapshot.Ref.ID).Set(ctx, data)
			if err != nil {
				return fmt.Errorf("unable to set document: %w", err)
			}
			log.Default().Println("added", data["name"])
		}
	} else {
		return h.generateNew(ctx)
	}

	log.Default().Println("done.")
	return nil
}

func (h *Handler) generateNew(ctx context.Context) error {
	dao := humandao.NewDAO(h.localFirestore)
	generator := loremipsum.New()
	for i := 0; i < opts.N; i++ {
		_, err := dao.AddHuman(ctx, humandao.AddHumanInput{
			Name:        fmt.Sprintf("Human %v", ksuid.New().String()),
			Gender:      humandao.GenderNonBinary,
			Ethnicity:   []string{"Chinese"},
			Website:     "https://example.com",
			Twitter:     "https://twitter.com",
			Location:    []string{"San Francisco", "CA"},
			Tags:        []string{"tag1", "tag2"},
			Draft:       false,
			DOB:         "1990-01-01",
			Description: generator.Paragraphs(5),
		})
		if err != nil {
			return fmt.Errorf("unable to create human: %w", err)
		}
	}
	return nil
}
