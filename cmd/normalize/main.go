package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"unicode"

	"cloud.google.com/go/firestore"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/urfave/cli/v2"
)

var opts struct {
	Dry bool
}

func main() {
	app := &cli.App{
		Name: "A CLI tool to normalize asian american data.",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "dry", Destination: &opts.Dry},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type Handler struct {
	fsClient *firestore.Client
	humanDAO *humandao.DAO
}

func run(c *cli.Context) error {
	ctx := c.Context
	fsClient, err := firestore.NewClient(ctx, api.ProjectID)
	if err != nil {
		return fmt.Errorf("unable to create firestore client: %w", err)
	}

	humanDAO := humandao.NewDAO(fsClient)
	h := Handler{
		fsClient: fsClient,
		humanDAO: humanDAO,
	}

	if err := h.Do(ctx); err != nil {
		return err
	}

	return nil
}

func (h *Handler) Do(ctx context.Context) error {
	// Get all humans
	humans, err := h.humanDAO.ListHumans(ctx, humandao.ListHumansInput{
		Limit: 1000,
	})
	if err != nil {
		return fmt.Errorf("unable to list humans: %w", err)
	}

	allEthnicities := make(map[string]struct{})
	for _, human := range humans {
		needUpdate := false
		for i, ethnicity := range human.Ethnicity {
			allEthnicities[ethnicity] = struct{}{}
			// check if string starts with upper

			if unicode.IsUpper(rune(ethnicity[0])) {
				needUpdate = true
				human.Ethnicity[i] = strings.ToLower(ethnicity)
			}

		}
		if needUpdate {
			log.Println("would update human", human.Name, human.Ethnicity)
		}

		if !opts.Dry {
			err := h.humanDAO.UpdateHuman(ctx, human)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
