package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/davidbyttow/govips/v2/vips"
	firebase "firebase.google.com/go/v4"
	"github.com/go-chi/httplog"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"

	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/functions/api/server"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/raymonstah/asianamericanswiki/internal/openai"
	"github.com/raymonstah/asianamericanswiki/internal/userdao"
	"github.com/raymonstah/asianamericanswiki/internal/xai"
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
			&cli.StringFlag{Name: "xai-api-key", EnvVars: []string{"XAI_API_KEY"}},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(c *cli.Context) error {
	vips.LoggingSettings(nil, vips.LogLevelError)
	vips.Startup(nil)
	defer vips.Shutdown()

	ctx := c.Context
	local := c.Bool("local")
	logger := httplog.NewLogger(api.ProjectID, httplog.Options{
		Concise:         true,
		JSON:            true,
		TimeFieldFormat: time.RFC3339,
	})

	if local {
		if err := setupEmulatorEnvironmentVariables(logger); err != nil {
			return fmt.Errorf("unable to setup local environment variables: %w", err)
		}
	}

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
	xaiClient := xai.New(c.String("xai-api-key"))

	config := server.Config{
		OpenAIClient:  openAiClient,
		XAIClient:     xaiClient,
		AuthClient:    authClient,
		HumanDAO:      humansDAO,
		UserDAO:       userDAO,
		Logger:        logger,
		Version:       c.String("git-hash"),
		StorageClient: storageClient,
		Local:         local,
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

func setupEmulatorEnvironmentVariables(logger zerolog.Logger) error {
	defer func(start time.Time) {
		logger.Info().Dur("elapsed", time.Since(start)).Msg("set up emulator env vars")
		fmt.Println("set up emulator env vars..")
	}(time.Now())

	if err := os.Setenv("FIRESTORE_EMULATOR_HOST", "localhost:8080"); err != nil {
		return err
	}
	//	if err := os.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "localhost:8081"); err != nil {
	//		return err
	//	}
	if err := os.Setenv("STORAGE_EMULATOR_HOST", "localhost:9199"); err != nil {
		return err
	}
	return nil
}
