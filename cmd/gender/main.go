package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"

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
		Name: "A CLI tool to cartoonize an image.",
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
	humans = withoutGender(humans)
	for _, human := range humans {
		log.Printf("name: %v\n", human.Name)
		gender, err := getGenderForHuman()
		if err != nil {
			return err
		}
		human.Gender = gender
		log.Printf("set %v (%v) to %v\n", human.Name, human.ID, human.Gender)
		if !opts.Dry {
			if err := h.humanDAO.UpdateHuman(ctx, human); err != nil {
				return fmt.Errorf("unable to update human: %w", err)
			}
		}
	}

	log.Println("done")
	return nil
}

func withoutGender(humans []humandao.Human) []humandao.Human {
	var withoutGender []humandao.Human
	for _, human := range humans {
		if human.Gender == "" {
			withoutGender = append(withoutGender, human)
		}
	}
	return withoutGender
}

func readChar() (byte, error) {
	reader := bufio.NewReader(os.Stdin)
	char, _, err := reader.ReadRune()
	if err != nil {
		return 0, err
	}
	return byte(char), nil
}

func getGenderForHuman() (humandao.Gender, error) {
	var gender humandao.Gender
	for {
		fmt.Print("Enter a single character (M, F, or X): ")
		char, err := readChar()
		if err != nil {
			return "", fmt.Errorf("unable to read character: %w", err)
		}

		switch char {
		case 'M', 'm':
			gender = humandao.GenderMale
		case 'F', 'f':
			gender = humandao.GenderFemale
		case 'X', 'x':
			gender = humandao.GenderNonBinary
		default:
			fmt.Println("Invalid input. Please enter 'M', 'F', or 'X'.")
		}

		if gender != "" {
			return gender, nil
		}
	}
}
