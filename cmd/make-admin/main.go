package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli/v2"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/raymonstah/asianamericanswiki/functions/api"
)

var opts struct {
	UID       string
	LocalAuth bool
}

func main() {
	app := &cli.App{
		Name: "A CLI tool to make a person an admin.",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "uid", Destination: &opts.UID},
			&cli.BoolFlag{Name: "local", Destination: &opts.LocalAuth},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type Handler struct {
	UID  string
	auth *auth.Client
}

func run(c *cli.Context) error {
	ctx := c.Context
	if opts.LocalAuth {
		if err := os.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "localhost:8081"); err != nil {
			return err
		}
	}

	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: api.ProjectID})
	if err != nil {
		return fmt.Errorf("failed to create firebase app: %w", err)
	}

	authClient, err := app.Auth(ctx)
	if err != nil {
		return fmt.Errorf("failed to create auth client: %w", err)
	}

	h := Handler{
		UID:  opts.UID,
		auth: authClient,
	}

	err = h.Do(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (h *Handler) Do(ctx context.Context) error {
	customClaims := map[string]interface{}{"admin": true}
	log.Default().Printf("Setting custom claims for user %v\n", h.UID)
	err := h.auth.SetCustomUserClaims(ctx, h.UID, customClaims)
	if err != nil {
		return fmt.Errorf("error setting custom claims for user %v: %w", h.UID, err)
	}

	userRecord, err := h.auth.GetUser(ctx, h.UID)
	if err != nil {
		return fmt.Errorf("error getting user: %w", err)
	}
	log.Default().Printf("User: %v\n", userRecord.DisplayName)
	log.Default().Printf("Custom Claims:")
	for k, v := range userRecord.CustomClaims {
		log.Default().Printf("\t%v - %v", k, v)
	}
	return nil
}
