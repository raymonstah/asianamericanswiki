package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"os"
	"regexp"
	"strings"

	"cloud.google.com/go/firestore"
	"github.com/go-json-experiment/json"
	"github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"

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
			&cli.StringSliceFlag{Name: "tags"},
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
	Tags     []string // optional, used when creating a new human.
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
		Tags:     c.StringSlice("tags"),
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
			if errors.Is(err, humandao.ErrHumanNotFound) {
				if err := h.addNew(ctx, h.Name); err != nil {
					return err
				}
				return nil
			}
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

type HumanCreateRequest struct {
	Name        string   `json:"name,omitempty"`
	DOB         string   `json:"dob,omitempty"`
	DOD         string   `json:"dod,omitempty"`
	Ethnicity   []string `json:"ethnicity,omitempty"`
	Description string   `json:"description,omitempty"`
	Location    []string `json:"location,omitempty"`
	Website     string   `json:"website,omitempty"`
	Twitter     string   `json:"twitter,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

func (h *Handler) addNew(ctx context.Context, name string) error {
	tags := strings.Join(h.Tags, ", ")
	slog.Info("Adding new human:", slog.String("name", name), slog.String("tags", tags))
	group, groupctx := errgroup.WithContext(ctx)
	var (
		description string
		request     HumanCreateRequest
	)
	group.Go(func() error {
		d, err := h.OpenAI.Generate(groupctx, openai.GenerateInput{
			Tags: h.Tags,
			Name: name,
		})
		if err != nil {
			return fmt.Errorf("unable to generate description: %w", err)
		}
		description = replaceSingleNewlineWithDoubleNewlines(d)
		return nil
	})
	group.Go(func() error {
		response, err := h.OpenAI.GenerateCreateRequest(groupctx, openai.GenerateCreateRequest{
			Tags: h.Tags,
			Name: name,
		})
		if err != nil {
			return fmt.Errorf("unable to generate json from openai: %w", err)
		}
		if err := json.Unmarshal(response, &request); err != nil {
			return err
		}
		return nil
	})
	if err := group.Wait(); err != nil {
		return err
	}

	input := humandao.AddHumanInput{
		Name:        request.Name,
		DOB:         request.DOB,
		DOD:         request.DOD,
		Ethnicity:   request.Ethnicity,
		Description: description,
		Location:    request.Location,
		Website:     request.Website,
		Twitter:     request.Twitter,
		Tags:        request.Tags,
	}

	fmt.Printf("Name: %v\n", input.Name)
	fmt.Printf("DOB: %v\n", input.DOB)
	fmt.Printf("DOD: %v\n", input.DOD)
	fmt.Printf("Ethnicity: %v\n", input.Ethnicity)
	fmt.Printf("Location: %v\n", input.Location)
	fmt.Printf("Website: %v\n", input.Website)
	fmt.Printf("Twitter: %v\n", input.Twitter)
	fmt.Printf("Tags: %v\n", input.Tags)
	fmt.Printf("Description: %v\n", input.Description)

	fmt.Println("Add new human? (y/n)")
	var userInput string
	_, err := fmt.Scan(&userInput)
	if err != nil {
		return fmt.Errorf("unable to scan user input: %w", err)
	}

	if strings.ToUpper(userInput) != "Y" {
		slog.Info("Aborting")
		return nil
	}

	human, err := h.HumanDAO.AddHuman(ctx, input)
	if err != nil {
		return fmt.Errorf("unable to add human: %w", err)
	}
	slog.Info("Successfully added human", slog.String("id", human.ID), slog.String("name", human.Name))
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

	newDescription = replaceSingleNewlineWithDoubleNewlines(newDescription)
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

func replaceSingleNewlineWithDoubleNewlines(input string) string {
	// Define a regular expression pattern to match a single newline character
	pattern := regexp.MustCompile(`(?m)([^\r\n])\r?\n`)

	// Replace a single newline with double newlines
	output := pattern.ReplaceAllString(input, "$1\n\n")

	return output
}
