package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"github.com/go-chi/httplog"
	"github.com/urfave/cli/v2"

	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/functions/api/server"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
)

func main() {
	app := &cli.App{
		Name: "api server for AsianAmericans.wiki",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "port", EnvVars: []string{"PORT"}, Value: 3000},
			&cli.BoolFlag{Name: "local"},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(c *cli.Context) error {
	if c.Bool("local") {
		if err := setupEmulatorEnvironmentVariables(); err != nil {
			return fmt.Errorf("unable to setup local environment variables: %w", err)
		}
	}

	logger := httplog.NewLogger(api.ProjectID, httplog.Options{
		Concise: true,
		JSON:    true,
	})

	app, err := firebase.NewApp(c.Context, &firebase.Config{
		ProjectID: api.ProjectID,
	})
	if err != nil {
		return fmt.Errorf("unable to create firestore app: %w", err)
	}

	authClient, err := app.Auth(c.Context)
	if err != nil {
		return fmt.Errorf("unable to create auth client: %w", err)
	}

	fsClient, err := firestore.NewClient(c.Context, api.ProjectID)
	if err != nil {
		return fmt.Errorf("unable to create firestore client: %w", err)
	}

	humansDAO := humandao.NewDAO(fsClient)
	config := server.Config{
		AuthClient: authClient,
		HumansDAO:  humansDAO,
		Logger:     logger,
	}

	mux := server.NewServer(config)

	address := fmt.Sprintf(":%v", c.Int("port"))
	s := http.Server{
		Addr:              address,
		Handler:           mux,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       5 * time.Second,
	}

	return s.ListenAndServe()
}

func setupEmulatorEnvironmentVariables() error {
	if err := os.Setenv("FIRESTORE_EMULATOR_HOST", "localhost:8080"); err != nil {
		return err
	}
	if err := os.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "localhost:8081"); err != nil {
		return err
	}
	return nil
}