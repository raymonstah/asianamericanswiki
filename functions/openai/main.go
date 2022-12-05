package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/raymonstah/asianamericanswiki/internal/openai"
)

func main() {
	app := &cli.App{
		Name: "api server for AsianAmericans.wiki",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "port", EnvVars: []string{"PORT"}, Value: 3000},
			&cli.BoolFlag{Name: "local"},
			&cli.StringFlag{Name: "open-ai-token", EnvVars: []string{"OPEN_AI_TOKEN"}},
			&cli.StringFlag{Name: "name", EnvVars: []string{"NAME"}},
			&cli.StringFlag{Name: "tags", EnvVars: []string{"TAGS"}},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(c *cli.Context) error {
	ctx := context.Background()
	client := openai.New(c.String("open-ai-token"))
	response, err := client.Generate(ctx, openai.GenerateInput{
		Tags: c.StringSlice("tags"),
		Name: c.String("name"),
	})
	if err != nil {
		return err
	}
	fmt.Println(response)

	return nil
}
