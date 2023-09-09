package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"cloud.google.com/go/firestore"
	"github.com/urfave/cli/v2"

	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
)

// Define a regular expression pattern to match a single newline character
var pattern = regexp.MustCompile(`(?m)([^\r\n])\r?\n`)

func main() {
	app := &cli.App{
		Name: "cli app to fix descriptions that contains single new lines for existing Asian Americans",
		Flags: []cli.Flag{
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

	humanDAO := humandao.NewDAO(fsClient)
	h := Handler{
		HumanDAO: humanDAO,
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

	newDescription := replaceSingleNewlineWithDoubleNewlines(human.Description)
	fmt.Println("New description:", newDescription)
	fmt.Println()
	fmt.Println("Accept new description? (y/n)")

	var userInput string
	_, err := fmt.Scan(&userInput)
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

	var humansWithSingleLineDescriptions []humandao.Human
	for _, human := range humans {
		if containsSingleNewlines(human.Description) {
			humansWithSingleLineDescriptions = append(humansWithSingleLineDescriptions, human)
		}

	}

	return humansWithSingleLineDescriptions, nil
}

func replaceSingleNewlineWithDoubleNewlines(input string) string {

	// Replace a single newline with double newlines
	output := pattern.ReplaceAllString(input, "$1\n\n")

	return output
}

func containsSingleNewlines(input string) bool {

	// Use the pattern to find matches in the input string
	matches := pattern.FindAllString(input, -1)

	// If matches are found, it contains single newlines
	return len(matches) > 0
}
