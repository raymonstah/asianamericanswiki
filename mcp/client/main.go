package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Human and Socials structs to parse the response
type Socials struct {
	Instagram string `json:"instagram"`
	X         string `json:"x"`
	Website   string `json:"website"`
	IMDB      string `json:"imdb"`
}

type Human struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
}

type HumansResponse struct {
	Humans []Human `json:"humans"`
}

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
			// Parse JSON
			var response HumansResponse
			if err := json.Unmarshal([]byte(tc.Text), &response); err != nil {
				fmt.Printf("I received a response, but it wasn't in the format I expected:\n%s\n", tc.Text)
			} else {
				// Format nicely
				count := len(response.Humans)
				if count == 0 {
					fmt.Println("I'm sorry, I couldn't find any results matching your query.")
				} else {
					intro := fmt.Sprintf("I found %d results for you.", count)
					if count == 1 {
						intro = "I found one person matching your search."
					}
					fmt.Printf("%s\n\n", intro)

					for _, h := range response.Humans {
						// Clean description
						desc := strings.ReplaceAll(h.Description, "\n", " ")
						desc = strings.TrimSpace(desc)

						// Truncate description if too long
						if len(desc) > 600 {
							desc = desc[:597] + "..."
						}
						fmt.Printf("%s\n%s\n\n", h.Name, desc)
					}
				}
			}
		}
	}
}

func main() {
	ctx := context.Background()

	// Create a new client.
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)

	// Connect to the server.
	cmd := exec.Command("./mcp-server")
	cmd.Stderr = os.Stderr // Pass server's stderr to our stderr for debugging
	transport := &mcp.CommandTransport{Command: cmd}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer session.Close()

	if len(os.Args) > 1 {
		query := strings.Join(os.Args[1:], " ")
		processQuery(ctx, session, query)
		return
	}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Welcome! Ask me anything about Asian Americans.")
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