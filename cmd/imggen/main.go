package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/cartoonize"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/urfave/cli/v2"
)

var bucketName = "asianamericanswiki-images"
var opts struct {
	Image string
	Name  string
	Debug bool
	AsIs  bool
}

func main() {
	app := &cli.App{
		Name: "A CLI tool to cartoonize an image.",
		Flags: []cli.Flag{
			&cli.PathFlag{Name: "image", Required: true, Destination: &opts.Image},
			&cli.StringFlag{Name: "name", Required: true, Destination: &opts.Name},
			&cli.BoolFlag{Name: "debug", Destination: &opts.Debug},
			&cli.BoolFlag{Name: "as-is", Destination: &opts.AsIs},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type Handler struct {
	storageClient *storage.Client
	fsClient      *firestore.Client
	humanDAO      *humandao.DAO
}

func run(c *cli.Context) error {
	ctx := c.Context
	fsClient, err := firestore.NewClient(ctx, api.ProjectID)
	if err != nil {
		return fmt.Errorf("unable to create firestore client: %w", err)
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("unable to create storage client: %w", err)
	}

	humanDAO := humandao.NewDAO(fsClient)
	h := Handler{
		fsClient:      fsClient,
		storageClient: client,
		humanDAO:      humanDAO,
	}

	if err := h.Do(ctx); err != nil {
		return err
	}

	return nil
}

func (h *Handler) Do(ctx context.Context) error {
	path := strings.ToLower(opts.Name)
	path = strings.ReplaceAll(path, " ", "-")
	human, err := h.humanDAO.Human(ctx, humandao.HumanInput{Path: path})
	if err != nil {
		return fmt.Errorf("unable to get human: %w", err)
	}

	var raw []byte
	id := human.ID
	if !opts.AsIs {
		cartoonizeClient := cartoonize.Client{
			Debug: opts.Debug,
		}
		raw, err = cartoonizeClient.Do(opts.Image)
		if err != nil {
			return fmt.Errorf("unable to cartoonize image for %v: %w", human.Name, err)
		}
	} else {
		raw, err = os.ReadFile(opts.Image)
		if err != nil {
			return err
		}
	}

	imgName := fmt.Sprintf("%v.jpg", id)
	obj := h.storageClient.Bucket(bucketName).Object(imgName)
	writer := obj.NewWriter(ctx)
	if _, err := writer.Write(raw); err != nil {
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	featuredImageURL := fmt.Sprintf("https://storage.googleapis.com/%v/%v", bucketName, imgName)
	human.FeaturedImage = featuredImageURL
	if err := h.humanDAO.UpdateHuman(ctx, human); err != nil {
		return fmt.Errorf("unable to update human: %w", err)
	}

	log.Println("done: ", featuredImageURL)
	return nil
}
