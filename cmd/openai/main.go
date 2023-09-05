package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"

	"cloud.google.com/go/firestore"
	"github.com/urfave/cli/v2"

	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/raymonstah/asianamericanswiki/internal/openai"
)

func main() {
	app := &cli.App{
		Name: "cli app to generate descriptions for existing Asian Americans",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "open-ai-token", EnvVars: []string{"OPEN_AI_TOKEN"}},
			&cli.StringFlag{Name: "name", EnvVars: []string{"NAME"}},
			&cli.BoolFlag{Name: "scan"},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type Handler struct {
	FSClient *firestore.Client
	OpenAI   *openai.Client
	HumanDAO *humandao.DAO
	Name     string
	Scan     bool
}

func run(c *cli.Context) error {
	ctx := c.Context
	fsClient, err := firestore.NewClient(ctx, api.ProjectID)
	if err != nil {
		return fmt.Errorf("unable to create firestore client: %w", err)
	}

	client := openai.New(c.String("open-ai-token"))
	humanDAO := humandao.NewDAO(fsClient)
	h := Handler{
		HumanDAO: humanDAO,
		OpenAI:   client,
		FSClient: fsClient,
		Name:     c.String("name"),
		Scan:     c.Bool("scan"),
	}

	if err := h.do(ctx); err != nil {
		return err
	}

	return nil
}

func (h *Handler) do(ctx context.Context) error {
	var humans []humandao.Human
	if h.Name != "" {
		path := strings.ReplaceAll(h.Name, " ", "-")
		path = strings.ToLower(path)
		human, err := h.HumanDAO.Human(ctx, humandao.HumanInput{Path: path})
		if err != nil {
			return err
		}
		humans = append(humans, human)
	}

	if h.Scan {
		log.Println("Scanning for humans with short descriptions...")
		humansWithShortDescriptions, err := h.DoScan(ctx)
		if err != nil {
			return err
		}
		humans = append(humans, humansWithShortDescriptions...)

	}

	for _, human := range humans {
		if err := h.generate(ctx, human); err != nil {
			return err
		}
	}

	return nil
}

func (h *Handler) generate(ctx context.Context, human humandao.Human) error {
	log.Println("Generating description for:", human.Name)
	fmt.Println("Old description:", human.Description)
	fmt.Println()

	newDescription, err := h.OpenAI.Generate(ctx, openai.GenerateInput{
		Tags: human.Tags,
		Name: human.Name,
	})
	if err != nil {
		return fmt.Errorf("unable to generate description: %w", err)
	}

	fmt.Println("New description:", newDescription)
	fmt.Println()
	fmt.Println("Accept new description? (y/n)")

	var userInput string
	_, err = fmt.Scan(&userInput)
	if err != nil {
		return fmt.Errorf("unable to scan user input: %w", err)
	}

	if strings.ToUpper(userInput) != "Y" {
		log.Println("Aborting")
		return nil
	}

	human.Description = newDescription
	if err := h.HumanDAO.UpdateHuman(ctx, human); err != nil {
		return err
	}

	log.Println("Successfully updated human")
	return nil
}

func (h *Handler) DoScan(ctx context.Context) ([]humandao.Human, error) {
	var humans []humandao.Human
	offset := 0
	for {
		hs, err := h.HumanDAO.ListHumans(ctx, humandao.ListHumansInput{
			Limit:  50,
			Offset: offset,
		})
		if err != nil {
			return nil, err
		}

		if len(hs) == 0 {
			break
		}
		offset += 50

		humans = append(humans, hs...)
	}

	var humansWithShortDescriptions []humandao.Human
	for _, human := range humans {
		// skip over humans with a long enough description
		if len(human.Description) > 105 {
			log.Println("skipping over:", human.Name)
			continue
		}

		humansWithShortDescriptions = append(humansWithShortDescriptions, human)
	}

	// randomly shuffle the humansWithShortDescriptions
	for i := range humansWithShortDescriptions {
		j := i + int(rand.Int63())%(len(humansWithShortDescriptions)-i)
		humansWithShortDescriptions[i], humansWithShortDescriptions[j] = humansWithShortDescriptions[j], humansWithShortDescriptions[i]
	}

	return humansWithShortDescriptions, nil
}
