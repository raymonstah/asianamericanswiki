package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	firebase "firebase.google.com/go/v4"
	"github.com/go-chi/httplog"
	"github.com/urfave/cli/v2"

	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/functions/api/server"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/raymonstah/asianamericanswiki/internal/openai"
	"github.com/raymonstah/asianamericanswiki/internal/userdao"
)

func main() {
	app := &cli.App{
		Name: "api server for AsianAmericans.wiki",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "port", EnvVars: []string{"PORT"}, Value: 3000},
			&cli.BoolFlag{Name: "local"},
			&cli.BoolFlag{Name: "no-auth"},
			&cli.StringFlag{Name: "git-hash", EnvVars: []string{"GIT_HASH"}, Value: "latest"},
			&cli.StringFlag{Name: "open-ai-token", EnvVars: []string{"OPEN_AI_TOKEN"}},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(c *cli.Context) error {
	ctx := c.Context
	if c.Bool("local") {
		if err := setupEmulatorEnvironmentVariables(); err != nil {
			return fmt.Errorf("unable to setup local environment variables: %w", err)
		}
	}

	logger := httplog.NewLogger(api.ProjectID, httplog.Options{
		Concise:         true,
		JSON:            true,
		TimeFieldFormat: time.RFC3339,
	})

	app, err := firebase.NewApp(ctx, &firebase.Config{
		ProjectID: api.ProjectID,
	})
	if err != nil {
		return fmt.Errorf("unable to create firestore app: %w", err)
	}

	var authClient server.Authorizer
	if c.Bool("no-auth") {
		logger.Info().Msg("using no-op authorizer")
		authClient = server.NoOpAuthorizer{}
	} else {
		authClient, err = app.Auth(ctx)
		if err != nil {
			return fmt.Errorf("unable to create auth client: %w", err)
		}
	}

	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("unable to create storage client: %w", err)
	}

	fsClient, err := firestore.NewClient(ctx, api.ProjectID)
	if err != nil {
		return fmt.Errorf("unable to create firestore client: %w", err)
	}

	humansDAO := humandao.NewDAO(fsClient)
	userDAO := userdao.NewDAO(fsClient)
	openAiClient := openai.New(c.String("open-ai-token"))

	config := server.Config{
		OpenAIClient:  openAiClient,
		AuthClient:    authClient,
		HumansDAO:     humansDAO,
		UsersDAO:      userDAO,
		Logger:        logger,
		Version:       c.String("git-hash"),
		StorageClient: storageClient,
	}

	mux := server.NewServer(config)

	address := fmt.Sprintf(":%v", c.Int("port"))
	s := http.Server{
		Addr:              address,
		Handler:           mux,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      90 * time.Second,
	}

	logger.Info().Str("port", c.String("port")).Msg("starting server")
	return s.ListenAndServe()
}

func setupEmulatorEnvironmentVariables() error {
	if err := os.Setenv("FIRESTORE_EMULATOR_HOST", "localhost:8080"); err != nil {
		return err
	}
	if err := os.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "localhost:8081"); err != nil {
		return err
	}
	if err := os.Setenv("STORAGE_EMULATOR_HOST", "localhost:9199"); err != nil {
		return err
	}
	return nil
}
