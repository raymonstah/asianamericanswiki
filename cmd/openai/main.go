package main

import (
	"context"
	"fmt"
	"log"
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
	Name     string
}

func run(c *cli.Context) error {
	ctx := c.Context
	fsClient, err := firestore.NewClient(ctx, api.ProjectID)
	if err != nil {
		return fmt.Errorf("unable to create firestore client: %w", err)
	}

	client := openai.New(c.String("open-ai-token"))
	h := Handler{
		OpenAI:   client,
		FSClient: fsClient,
		Name:     c.String("name"),
	}

	if err := h.generate(ctx); err != nil {
		return err
	}

	return nil
}

func (h *Handler) generate(ctx context.Context) error {
	humanDAO := humandao.NewDAO(h.FSClient)
	path := strings.ReplaceAll(h.Name, " ", "-")
	path = strings.ToLower(path)
	human, err := humanDAO.Human(ctx, humandao.HumanInput{Path: path})
	if err != nil {
		return err
	}

	newDescription, err := h.OpenAI.Generate(ctx, openai.GenerateInput{
		Tags: human.Tags,
		Name: h.Name,
	})
	if err != nil {
		return fmt.Errorf("unable to generate description: %w", err)
	}

	fmt.Println("New description:", newDescription)
	fmt.Println("Accept new description? (y/n)")

	var userInput string
	_, err = fmt.Scan(&userInput)
	if err != nil {
		return fmt.Errorf("unable to scan user input: %w", err)
	}

	if strings.ToUpper(userInput) != "Y" {
		fmt.Println("Aborting")
		return nil
	}

	human.Description = newDescription
	if err := humanDAO.UpdateHuman(ctx, human); err != nil {
		return err
	}

	fmt.Println("Successfully updated human")
	return nil
}
