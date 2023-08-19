package main

import (
	"context"
	"fmt"
	"log"
	"os"

	firebase "firebase.google.com/go/v4"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/segmentio/ksuid"
	"github.com/urfave/cli/v2"
	"gopkg.in/loremipsum.v1"
)

var opts struct {
	N int
}

func main() {
	app := &cli.App{
		Name: "A CLI tool to make a generate humans for testing purposes.",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "n", Usage: "how many humans to generate", Destination: &opts.N},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type Handler struct {
	dao *humandao.DAO
	n   int
}

func run(c *cli.Context) error {
	ctx := c.Context
	if err := os.Setenv("FIRESTORE_EMULATOR_HOST", "localhost:8080"); err != nil {
		return err
	}

	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: api.ProjectID})
	if err != nil {
		return fmt.Errorf("failed to create firebase app: %w", err)
	}

	fsClient, err := app.Firestore(ctx)
	if err != nil {
		return fmt.Errorf("failed to create firestore client: %w", err)
	}

	h := Handler{
		dao: humandao.NewDAO(fsClient),
		n:   opts.N,
	}

	if err := h.Do(ctx); err != nil {
		return err
	}

	return nil
}

func (h *Handler) Do(ctx context.Context) error {
	generator := loremipsum.New()
	for i := 0; i < h.n; i++ {
		_, err := h.dao.AddHuman(ctx, humandao.AddHumanInput{
			Name:        fmt.Sprintf("Human %v", ksuid.New().String()),
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
