package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"cloud.google.com/go/firestore"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"
)

var opts struct {
}

func main() {
	app := &cli.App{
		Name:   "A CLI migration tool to backfill the Humans.Socials struct.",
		Flags:  []cli.Flag{},
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
	allHumans, err := h.humanDAO.ListHumans(ctx, humandao.ListHumansInput{
		Limit:  300,
		Offset: 0,
	})
	if err != nil {
		return fmt.Errorf("unable to get all humans: %w", err)
	}

	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(16)
	for _, human := range allHumans {
		human := human
		group.Go(func() error {
			human.Socials.Website = human.Website
			human.Socials.X = human.Twitter
			err := h.humanDAO.UpdateHuman(ctx, human)
			if err != nil {
				return fmt.Errorf("unable to update human: %w", err)
			}
			slog.Info("updated human",
				slog.String("name", human.Name),
				slog.String("website", human.Website),
				slog.String("x", human.Socials.X))

			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}

	slog.Info("done")
	return nil
}
