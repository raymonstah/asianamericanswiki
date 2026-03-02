package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/segmentio/ksuid"
	"github.com/urfave/cli/v2"
	"google.golang.org/api/iterator"
	"gopkg.in/loremipsum.v1"
)

var opts struct {
	N       int
	UseProd bool
	Force   bool
	Dry     bool
}

func main() {
	app := &cli.App{
		Name: "A CLI tool to make a generate humans for testing purposes.",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "n", Usage: "how many humans to generate", Destination: &opts.N},
			&cli.BoolFlag{Name: "use-prod", Usage: "pull data from production", Destination: &opts.UseProd},
			&cli.BoolFlag{Name: "force", Usage: "overwrite local data", Destination: &opts.Force},
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

	if err := os.Setenv("FIRESTORE_EMULATOR_HOST", "127.0.0.1:8080"); err != nil {
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
		if opts.Force {
			log.Default().Println("purging local data...")
			if err := h.purgeLocal(ctx); err != nil {
				return fmt.Errorf("unable to purge local data: %w", err)
			}
		} else {
			localSnapshots, err := h.localFirestore.Collection("humans").Documents(ctx).GetAll()
			if err != nil {
				return fmt.Errorf("unable to get local documents: %w", err)
			}

			if len(localSnapshots) > 0 {
				return fmt.Errorf("local firestore is not empty (use --force to overwrite)")
			}
		}

		log.Default().Println("fetching production data...")
		snapshots, err := h.prodFirestore.Collection("humans").Documents(ctx).GetAll()
		if err != nil {
			return fmt.Errorf("unable to get production documents: %w", err)
		}

		log.Default().Printf("copying %d humans to local firestore...\n", len(snapshots))
		bw := h.localFirestore.BulkWriter(ctx)
		for i, snapshot := range snapshots {
			data := snapshot.Data()
			if opts.Dry {
				fmt.Printf("would add %v: %v\n", i, data["name"])
				continue
			}

			if _, err := bw.Set(h.localFirestore.Collection("humans").Doc(snapshot.Ref.ID), data); err != nil {
				return fmt.Errorf("unable to set document: %w", err)
			}
			if (i+1)%500 == 0 {
				log.Default().Printf("writing %d humans...\n", i+1)
			}
		}

		if !opts.Dry {
			bw.End()
		}
	} else {
		return h.generateNew(ctx)
	}

	log.Default().Println("done.")
	return nil
}

func (h *Handler) purgeLocal(ctx context.Context) error {
	iter := h.localFirestore.Collection("humans").Documents(ctx)
	bw := h.localFirestore.BulkWriter(ctx)
	count := 0
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		if _, err := bw.Delete(doc.Ref); err != nil {
			return err
		}
		count++
		if count%500 == 0 {
			log.Default().Printf("purging %d humans...\n", count)
		}
	}

	bw.End()
	log.Default().Printf("purged %d documents\n", count)
	return nil
}

func (h *Handler) generateNew(ctx context.Context) error {
	log.Default().Printf("generating %d new humans...\n", opts.N)
	generator := loremipsum.New()
	bw := h.localFirestore.BulkWriter(ctx)
	for i := 0; i < opts.N; i++ {
		name := fmt.Sprintf("Human %v", ksuid.New().String())
		path := humandao.Slug(name)
		humanID := ksuid.New().String()
		human := humandao.Human{
			ID:     humanID,
			Name:   name,
			Gender: humandao.GenderNonBinary,
			Ethnicity: []string{
				"Chinese",
			},
			Socials: humandao.Socials{
				Website: "https://example.com",
				X:       "https://twitter.com",
			},
			Location: []string{
				"San Francisco",
				"CA",
			},
			Tags: []string{
				"tag1",
				"tag2",
			},
			Draft:       false,
			DOB:         "1990-01-01",
			Description: generator.Paragraphs(5),
			Path:        path,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if _, err := bw.Set(h.localFirestore.Collection("humans").Doc(humanID), human); err != nil {
			return err
		}
		if (i+1)%500 == 0 {
			log.Default().Printf("generated %d humans...\n", i+1)
		}
	}

	bw.End()
	return nil
}
