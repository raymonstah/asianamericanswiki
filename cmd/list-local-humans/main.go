package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/firestore"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
)

func main() {
	ctx := context.Background()
	if err := os.Setenv("FIRESTORE_EMULATOR_HOST", "127.0.0.1:8080"); err != nil {
		log.Fatalf("failed to set env: %v", err)
	}

	client, err := firestore.NewClient(ctx, api.ProjectID)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	defer func() { _ = client.Close() }()

	dao := humandao.NewDAO(client)
	humans, err := dao.ListHumans(ctx, humandao.ListHumansInput{
		Limit:         10,
		IncludeDrafts: true,
	})
	if err != nil {
		log.Fatalf("failed to list humans: %v", err)
	}

	fmt.Println("Recent humans in local emulator:")
	for _, h := range humans {
		fmt.Printf("- %s (ID: %s, Path: %s)\n", h.Name, h.ID, h.Path)
	}
}
