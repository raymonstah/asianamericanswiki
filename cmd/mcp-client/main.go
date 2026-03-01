package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func processQuery(ctx context.Context, session *mcp.ClientSession, query string) {
	if query == "" {
		return
	}

	// Pre-process query to remove stop words and punctuation
	words := strings.Fields(query)
	var filtered []string
	stopWords := map[string]bool{
		"who": true, "was": true, "the": true, "on": true, "in": true, "of": true,
		"is": true, "are": true, "a": true, "an": true, "at": true, "by": true,
		"what": true, "where": true, "when": true, "how": true,
	}

	for _, w := range words {
		cleanW := strings.TrimRight(w, "?!.,")
		if !stopWords[strings.ToLower(cleanW)] {
			filtered = append(filtered, cleanW)
		}
	}

	cleanedQuery := strings.Join(filtered, " ")
	if len(filtered) == 0 && len(words) > 0 {
		cleanedQuery = query
	}

	// Call the search tool
	params := &mcp.CallToolParams{
		Name:      "search-asian-americans",
		Arguments: map[string]any{"query": cleanedQuery},
	}
	res, err := session.CallTool(ctx, params)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	if res.IsError {
		fmt.Println("The server reported an error.")
		return
	}

	// Process results
	if len(res.Content) > 0 {
		if tc, ok := res.Content[0].(*mcp.TextContent); ok {
			fmt.Printf("Response:\n%s\n", tc.Text)
		}
	}
}

func main() {
	ctx := context.Background()

	// Create a new client.
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v2.0.0"}, nil)

	// Connect to the server.
	// We use "go run" to point to the new location.
	cmd := exec.Command("go", "run", "cmd/mcp-server/main.go")
	cmd.Stderr = os.Stderr
	transport := &mcp.CommandTransport{Command: cmd}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer func() { _ = session.Close() }()

	if len(os.Args) > 1 {
		query := strings.Join(os.Args[1:], " ")
		processQuery(ctx, session, query)
		return
	}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Welcome to the Asian Americans Wiki MCP Client!")
	fmt.Println("Type 'quit' to exit.")

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		input := scanner.Text()
		input = strings.TrimSpace(input)

		if input == "quit" || input == "exit" {
			fmt.Println("Goodbye!")
			break
		}
		
		processQuery(ctx, session, input)
	}
}
