package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
)


func main() {
	ctx := context.Background()
	if err := run(ctx); err != nil {
		log.Fatalf("error running committer: %s", err)
	}

}

func run(ctx context.Context) error {
	port := 6969
	server := http.Server{
		Addr:    fmt.Sprintf(":%v", port),
		Handler: http.HandlerFunc(Handle),
	}

	log.Printf("starting server on port %v", port)
	server.ListenAndServe()
	return nil
}
