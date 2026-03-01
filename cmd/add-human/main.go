package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"cloud.google.com/go/firestore"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/urfave/cli/v2"
)

func main() {
	var opts struct {
		Name        string
		DOB         string
		Ethnicity   string
		Description string
		Location    string
		Instagram   string
		Tags        string
		Gender      string
		UseProd     bool
	}

	app := &cli.App{
		Name:  "add-human",
		Usage: "Manually add a human to the database",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Required: true, Destination: &opts.Name},
			&cli.StringFlag{Name: "dob", Usage: "YYYY-MM-DD", Destination: &opts.DOB},
			&cli.StringFlag{Name: "ethnicity", Usage: "Comma separated ethnicities", Destination: &opts.Ethnicity},
			&cli.StringFlag{Name: "description", Destination: &opts.Description},
			&cli.StringFlag{Name: "location", Usage: "Comma separated locations", Destination: &opts.Location},
			&cli.StringFlag{Name: "instagram", Destination: &opts.Instagram},
			&cli.StringFlag{Name: "tags", Usage: "Comma separated tags", Destination: &opts.Tags},
			&cli.StringFlag{Name: "gender", Value: "nonbinary", Destination: &opts.Gender},
			&cli.BoolFlag{Name: "use-prod", Value: false, Destination: &opts.UseProd},
		},
		Action: func(c *cli.Context) error {
			if !opts.UseProd {
				if err := os.Setenv("FIRESTORE_EMULATOR_HOST", "127.0.0.1:8080"); err != nil {
					return err
				}
			} else {
				if err := os.Unsetenv("FIRESTORE_EMULATOR_HOST"); err != nil {
					return err
				}
			}

			client, err := firestore.NewClient(c.Context, api.ProjectID)
			if err != nil {
				return err
			}
			defer func() { _ = client.Close() }()
			dao := humandao.NewDAO(client)

			ethnicities := strings.Split(opts.Ethnicity, ",")
			for i, e := range ethnicities {
				ethnicities[i] = strings.TrimSpace(e)
			}

			locations := strings.Split(opts.Location, ",")
			for i, l := range locations {
				locations[i] = strings.TrimSpace(l)
			}

			tags := strings.Split(opts.Tags, ",")
			for i, t := range tags {
				tags[i] = strings.TrimSpace(t)
			}

			input := humandao.AddHumanInput{
				Name:        opts.Name,
				DOB:         opts.DOB,
				Ethnicity:   ethnicities,
				Description: opts.Description,
				Location:    locations,
				Instagram:   opts.Instagram,
				Tags:        tags,
				Gender:      humandao.Gender(opts.Gender),
				Draft:       false,
			}

			human, err := dao.AddHuman(c.Context, input)
			if err != nil {
				return err
			}

			fmt.Printf("Successfully added %s (ID: %s)\n", human.Name, human.ID)
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
