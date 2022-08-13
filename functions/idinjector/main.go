package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/firestore"
	"github.com/segmentio/ksuid"
	"github.com/urfave/cli/v2"

	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
)

var (
	flagAddToFireStore       = "add-to-firestore"
	flagFirestoreCredentials = "firestore-credentials"
	flagDryRun               = "dry-run"
	dir                      = "content/humans"
	ErrAlreadyHasID          = errors.New("id already exists")
)

func main() {
	app := &cli.App{
		Action: action,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  flagAddToFireStore,
				Usage: "add new humans to firestore",
			},
			&cli.BoolFlag{
				Name:  flagDryRun,
				Usage: "dry run adding to firestore",
			},
			&cli.StringFlag{
				Name:  flagFirestoreCredentials,
				Usage: "service account json credentials",
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func action(c *cli.Context) error {
	ctx := c.Context
	newHumans, err := walkContent(dir)
	if err != nil {
		return err
	}

	if c.Bool(flagAddToFireStore) {
		//credentials := []byte(c.String(flagFirestoreCredentials))
		fsClient, err := firestore.NewClient(ctx, api.ProjectID)
		if err != nil {
			return fmt.Errorf("unable to create firestore client: %w", err)
		}
		dao := humandao.NewDAO(fsClient)
		for _, human := range newHumans {
			if c.Bool(flagDryRun) {
				log.Printf("add human %v with id %v\n", human.Name, human.ID)
				continue
			}

			_, err := dao.AddHuman(ctx, humandao.AddHumanInput{HumanID: human.ID, Name: human.Name})
			if err != nil {
				return fmt.Errorf("unable to add %v: %w", human.Name, err)
			}
		}
	}

	return nil
}

func walkContent(dir string) ([]Human, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	projectDirectory := filepath.Dir(filepath.Dir(wd))
	humansDir := filepath.Join(projectDirectory, dir)

	var newHumans []Human
	err = filepath.WalkDir(humansDir, func(path string, d fs.DirEntry, err error) error {
		if d.Name() != "index.md" {
			return nil
		}
		rawContent, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("unable to read file: %w", err)
		}

		human, err := injectID(rawContent, ksuid.New().String())
		if err != nil {
			if errors.Is(err, ErrAlreadyHasID) {
				return nil
			}
			return fmt.Errorf("unable to inject ID: %w", err)
		}

		newHumans = append(newHumans, human)
		if err := os.WriteFile(path, human.NewContent, 0644); err != nil {
			return fmt.Errorf("unable to overwrite content for %v: %w", path, err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("unable to walk humans directory: %v: %w", humansDir, err)
	}

	return newHumans, nil
}

type Human struct {
	ID         string
	Name       string
	NewContent []byte
}

// injectID parses markdown content, and returns a human object containing the new markdown with an id injected.
func injectID(content []byte, id string) (Human, error) {
	var (
		tickers    = `---`
		s          = string(content)
		startIndex = strings.Index(s, tickers)
	)
	if startIndex == -1 {
		return Human{}, fmt.Errorf("starting '---' of front matter not found")
	}

	endIndex := len(tickers) + strings.Index(s[startIndex+len(tickers):], tickers)
	if endIndex == -1 {
		return Human{}, fmt.Errorf("ending '---' of front matter not found")
	}

	var (
		human       = Human{ID: id}
		frontMatter = s[startIndex : endIndex+len(tickers)]
		idLine      = fmt.Sprintf("id: %q", id)
		lines       = strings.Split(frontMatter, "\n")
	)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Index(line, "id:") != -1 {
			return Human{}, ErrAlreadyHasID
		}
		if v, hasName := parseName(line); hasName {
			human.Name = v
		}

	}

	lines = append([]string{lines[0]}, append([]string{idLine}, lines[1:]...)...)
	frontMatter = strings.Join(lines, "\n")
	newContent := frontMatter + s[endIndex+len(tickers):]
	human.NewContent = []byte(newContent)
	return human, nil
}

func parseName(line string) (string, bool) {
	if strings.Index(line, "title:") == 0 {
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			return "", false
		}
		name := parts[1]
		name = strings.TrimSpace(name)
		name = strings.TrimPrefix(name, `"`)
		name = strings.TrimPrefix(name, `'`)
		name = strings.TrimSuffix(name, `"`)
		name = strings.TrimSuffix(name, `'`)
		return name, true
	}

	return "", false
}
