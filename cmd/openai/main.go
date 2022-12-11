package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/raymonstah/asianamericanswiki/internal/openai"
)

func main() {
	app := &cli.App{
		Name: "cli app to generate descriptions for existing Asian Americans",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "local"},
			&cli.StringFlag{Name: "open-ai-token", EnvVars: []string{"OPEN_AI_TOKEN"}},
			&cli.StringFlag{Name: "name", EnvVars: []string{"NAME"}},
			&cli.StringFlag{Name: "tags", EnvVars: []string{"TAGS"}},
			&cli.BoolFlag{Name: "dry"},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type Handler struct {
	Dry    bool
	OpenAi *openai.Client
	Dir    string
	Name   string
	Tags   []string
}

func run(c *cli.Context) error {
	ctx := c.Context
	client := openai.New(c.String("open-ai-token"))
	h := Handler{
		Dry:    c.Bool("dry"),
		OpenAi: client,
		Dir:    "content/humans",
		Name:   c.String("name"),
		Tags:   c.StringSlice("tags"),
	}
	err := h.walkContent(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (h *Handler) walkContent(ctx context.Context) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	humansDir := filepath.Join(wd, h.Dir)
	fmt.Println("searching for", h.Name)
	err = filepath.WalkDir(humansDir, func(path string, d fs.DirEntry, err error) error {
		if d.Name() != "index.md" {
			return nil
		}

		lastSlash := strings.LastIndex(path, "/")
		name := path[len(humansDir)+1 : lastSlash]
		name = strings.ReplaceAll(name, "-", " ")
		if strings.ToLower(name) != strings.ToLower(h.Name) {
			return nil
		}

		if h.Dry {
			fmt.Println("dry run -- would perform update on", name)
			return nil
		}
		fmt.Println("Human found:", path)

		rawContent, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("unable to read file: %w", err)
		}

		newDescription, err := h.OpenAi.Generate(ctx, openai.GenerateInput{
			Tags: h.Tags,
			Name: h.Name,
		})
		if err != nil {
			return fmt.Errorf("unable to generate description: %w", err)
		}

		fmt.Println("New description generated:", newDescription)
		newContent, err := overwriteDescription(rawContent, newDescription)
		if err != nil {
			return err
		}

		if err := os.WriteFile(path, newContent, 0644); err != nil {
			return fmt.Errorf("unable to overwrite content for %v: %w", path, err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to walk humans directory: %v: %w", humansDir, err)
	}

	return nil
}

func overwriteDescription(content []byte, newDescription string) ([]byte, error) {
	var (
		tickers    = `---`
		s          = string(content)
		startIndex = strings.Index(s, tickers)
	)
	if startIndex == -1 {
		return nil, fmt.Errorf("starting '---' of front matter not found")
	}

	endIndex := len(tickers) + strings.Index(s[startIndex+len(tickers):], tickers)
	if endIndex == -1 {
		return nil, fmt.Errorf("ending '---' of front matter not found")
	}

	frontMatter := s[startIndex : endIndex+len(tickers)]
	newContent := frontMatter + "\n\n" + newDescription
	return []byte(newContent), nil
}
