package main

import (
	"context"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"

	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
)

var opts struct {
	Dry            bool
	HumansDir      string
	LocalFirestore bool
}

func main() {
	app := &cli.App{
		Name: "A ClI tool to migrate all humans from markdown to JSON, so we can store them in Firestore.",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "dry", Destination: &opts.Dry},
			&cli.PathFlag{Name: "human-dir", Destination: &opts.HumansDir},
			&cli.BoolFlag{Name: "local", Destination: &opts.LocalFirestore},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type Handler struct {
	Dry       bool
	HumansDir string
	HumanDAO  *humandao.DAO
}

func run(c *cli.Context) error {
	ctx := c.Context
	if opts.LocalFirestore {
		if err := os.Setenv("FIRESTORE_EMULATOR_HOST", "localhost:8080"); err != nil {
			log.Fatal("failed to set FIRESTORE_EMULATOR_HOST environment variable", err)
		}
	}

	fsClient, err := firestore.NewClient(ctx, api.ProjectID)
	if err != nil {
		return fmt.Errorf("unable to create firestore client: %w", err)
	}
	humanDAO := humandao.NewDAO(fsClient)
	h := Handler{
		Dry:       opts.Dry,
		HumansDir: opts.HumansDir,
		HumanDAO:  humanDAO,
	}
	err = h.Do(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (h *Handler) Do(ctx context.Context) error {
	var humans []humandao.Human
	err := filepath.WalkDir(h.HumansDir, func(path string, d fs.DirEntry, err error) error {
		if d.Name() != "index.md" {
			return nil
		}

		lastSlash := strings.LastIndex(path, "/")
		fileName := path[len(h.HumansDir)+1 : lastSlash]
		human, err := convert(path, fileName)
		if err != nil {
			return fmt.Errorf("unable to convert %v: %w", fileName, err)
		}
		humans = append(humans, human)
		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to walk humans directory: %v: %w", h.HumansDir, err)
	}

	workers := make(chan struct{}, 16)
	for i := 0; i < 16; i++ {
		workers <- struct{}{}
	}
	group, ctx := errgroup.WithContext(ctx)
	for i, human := range humans {
		i := i
		human := human
		group.Go(func() error {
			<-workers
			defer func() { workers <- struct{}{} }()
			fmt.Printf("%v - Adding %q to firestore..\n", i, human.Name)
			if !h.Dry {
				if err := h.HumanDAO.UpdateHuman(ctx, human); err != nil {
					return err
				}
			} else {
				fmt.Println("dry run -- skipping..")
			}
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return fmt.Errorf("unable to add to firestore: %w", err)
	}

	fmt.Println("done.")
	return nil
}

func convert(path string, fileName string) (humandao.Human, error) {
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return humandao.Human{}, fmt.Errorf("unable to read file: %v: %w", path, err)
	}
	var (
		tickers    = `---`
		s          = string(raw)
		startIndex = strings.Index(s, tickers)
	)
	if startIndex == -1 {
		return humandao.Human{}, fmt.Errorf("starting '---' of front matter not found")
	}

	endIndex := len(tickers) + strings.Index(s[startIndex+len(tickers):], tickers)
	if endIndex == -1 {
		return humandao.Human{}, fmt.Errorf("ending '---' of front matter not found")
	}

	var humanFrontMatter struct {
		ID            string   `yaml:"id"`
		Title         string   `yaml:"title"`
		Date          string   `yaml:"date"`
		DOB           string   `yaml:"dob,omitempty"`
		DOD           string   `yaml:"dod,omitempty"`
		Tags          []string `yaml:"tags,omitempty"`
		Website       string   `yaml:"website,omitempty"`
		Ethnicity     []string `yaml:"ethnicity,omitempty"`
		BirthLocation string   `yaml:"birthLocation,omitempty"`
		Location      []string `yaml:"location,omitempty"`
		InfluencedBy  []string `yaml:"influencedBy,omitempty"`
		Twitter       string   `yaml:"twitter,omitempty"`
		FeaturedImage string   `yaml:"featured_image,omitempty"`
		Draft         bool     `yaml:"draft,omitempty"`
		AIGenerated   bool     `yaml:"ai_generated,omitempty"`
	}
	frontMatter := s[startIndex : endIndex+len(tickers)]
	description := s[endIndex+len(tickers):]
	if err := yaml.Unmarshal([]byte(frontMatter), &humanFrontMatter); err != nil {
		return humandao.Human{}, err
	}
	if humanFrontMatter.DOB == "YYYY-MM-DD" {
		humanFrontMatter.DOB = ""
	}
	if humanFrontMatter.DOD == "YYYY-MM-DD" {
		humanFrontMatter.DOD = ""
	}

	return humandao.Human{
		ID:            humanFrontMatter.ID,
		Name:          humanFrontMatter.Title,
		Path:          fileName,
		CreatedAt:     parseTime(humanFrontMatter.Date),
		DOB:           humanFrontMatter.DOB,
		DOD:           humanFrontMatter.DOD,
		Tags:          humanFrontMatter.Tags,
		Website:       humanFrontMatter.Website,
		Ethnicity:     humanFrontMatter.Ethnicity,
		BirthLocation: humanFrontMatter.BirthLocation,
		Location:      humanFrontMatter.Location,
		InfluencedBy:  humanFrontMatter.InfluencedBy,
		Twitter:       humanFrontMatter.Twitter,
		FeaturedImage: humanFrontMatter.FeaturedImage,
		Draft:         humanFrontMatter.Draft,
		AIGenerated:   humanFrontMatter.AIGenerated,
		Description:   description,
	}, nil
}

func parseTime(t string) time.Time {
	got, err := time.Parse("2006-01-02T15:04:05", t)
	if err == nil {
		return got
	}

	got, err = time.Parse(time.RFC3339, t)
	if err == nil {
		return got
	}
	panic(t)
}
