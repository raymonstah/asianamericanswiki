package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"slices"
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
		Commands: []*cli.Command{
			{
				Name:   "ethnicity",
				Usage:  "normalize ethnicity data",
				Action: ethnicity,
			},
			{
				Name:   "tags",
				Usage:  "normalize tags data",
				Action: tags,
			},
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "dry", Destination: &opts.Dry},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type Handler struct {
	fsClient *firestore.Client
	humanDAO *humandao.DAO
}

func prepareHandler(ctx context.Context) (Handler, error) {
	fsClient, err := firestore.NewClient(ctx, api.ProjectID)
	if err != nil {
		return Handler{}, fmt.Errorf("unable to create firestore client: %w", err)
	}

	humanDAO := humandao.NewDAO(fsClient)
	h := Handler{
		fsClient: fsClient,
		humanDAO: humanDAO,
	}

	return h, nil
}

func ethnicity(c *cli.Context) error {
	ctx := c.Context
	h, err := prepareHandler(ctx)
	if err != nil {
		return err
	}
	if err := h.Ethnicity(ctx); err != nil {
		return err
	}

	return nil
}

func tags(c *cli.Context) error {
	ctx := c.Context
	h, err := prepareHandler(ctx)
	if err != nil {
		return err
	}
	if err := h.Tags(ctx); err != nil {
		return err
	}

	return nil
}

func (h *Handler) Ethnicity(ctx context.Context) error {
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

func (h *Handler) Tags(ctx context.Context) error {
	// Get all humans
	humans, err := h.humanDAO.ListHumans(ctx, humandao.ListHumansInput{
		Limit: 1000,
	})
	if err != nil {
		return fmt.Errorf("unable to list humans: %w", err)
	}

	// Lowercase all Tags
	for _, human := range humans {
		needUpdate := false
		previousTags := slices.Clone(human.Tags)
		for i, tag := range human.Tags {
			// ensure tags are lowercase
			if strings.ToLower(tag) != tag {
				needUpdate = true
				human.Tags[i] = strings.ToLower(tag)
			}

			// ensure tag doesn't contain multiple values
			parts := strings.Split(tag, ",")
			if len(parts) > 1 {
				needUpdate = true
				human.Tags[i] = parts[0]
				human.Tags = append(human.Tags, parts[1:]...)
			}

			normalizeValues := map[string]string{
				"technologist":      "technology",
				"tech":              "technology",
				"software engineer": "technology",
				"youTuber":          "youtuber",
				"olympics":          "olympian",
				"photography":       "photographer",
				"music":             "musician",
				"activism":          "activist",
				"actress":           "actor",
				"comedy":            "comedian",
				"youtube":           "youtuber",
				"entertainment":     "entertainer",
				"lgbt":              "lgbtq",
			}

			normalizedTag, ok := normalizeValues[tag]
			if ok {
				needUpdate = true
				human.Tags[i] = normalizedTag
			}
		}

		if needUpdate {
			log.Printf("would update %v's tags", human.Name)
			log.Printf("\tbefore: %v", previousTags)
			log.Printf("\tafter: %v", human.Tags)

			if !opts.Dry {
				err := h.humanDAO.UpdateHuman(ctx, human)
				if err != nil {
					return err
				}
				log.Println("successfully updated", human.Name)
			}
		}
	}

	return nil
}
