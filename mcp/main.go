package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Socials struct {
	Instagram string `json:"instagram"`
	X         string `json:"x"`
	Website   string `json:"website"`
	IMDB      string `json:"imdb"`
}

type Human struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	DOB         string   `json:"dob"`
	DOD         string   `json:"dod"`
	Tags        []string `json:"tags"`
	Ethnicity   []string `json:"ethnicity"`
	Image       string   `json:"image"`
	Description string   `json:"description"`
	Socials     Socials  `json:"socials"`
	Gender      string   `json:"gender"`
}

type HumansResponse struct {
	Humans []Human `json:"humans"`
}

type SearchInput struct {
	Query string `json:"query" jsonschema:"The search query to filter Asian Americans by (e.g. 'Chinese rappers', 'actresses')"`
}

func fetchData() (HumansResponse, error) {
	resp, err := http.Get("https://asianamericans.wiki/api/v1/humans")
	if err != nil {
		return HumansResponse{}, fmt.Errorf("failed to fetch data: %w", err)
	}
	defer resp.Body.Close()

	var data HumansResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return HumansResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}
	return data, nil
}



func SearchAsianAmericans(ctx context.Context, req *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, HumansResponse, error) {
	data, err := fetchData()
	if err != nil {
		return nil, HumansResponse{}, err
	}

	query := strings.ToLower(input.Query)
	terms := strings.Fields(query)
	filtered := []Human{}

	// Basic stop words to ignore to make natural language queries work better
	stopWords := map[string]bool{
		"who": true, "are": true, "some": true, "famous": true, "is": true, "a": true, "an": true, "the": true, "of": true, "in": true,
	}

	for _, h := range data.Humans {
		// Calculate a match score
		matches := 0

		// Simple search strategy:
		// 1. We want to match as many meaningful terms as possible.
		// 2. We check Name, Ethnicity, Tags, and Description.

		// Helper to check if a term exists in the human's data
		termInHuman := func(term string) bool {
			// Check Name
			if strings.Contains(strings.ToLower(h.Name), term) {
				return true
			}
			// Check Ethnicity
			for _, eth := range h.Ethnicity {
				if strings.Contains(strings.ToLower(eth), term) {
					return true
				}
			}
			// Check Tags
			for _, tag := range h.Tags {
				if strings.Contains(strings.ToLower(tag), term) {
					return true
				}
			}
			// Check Description (lower weight usually, but simple boolean here)
			if strings.Contains(strings.ToLower(h.Description), term) {
				return true
			}
			return false
		}

		validTerms := 0
		for _, term := range terms {
			if stopWords[term] {
				continue
			}
			
			// Simple plural handling
			termToCheck := term
			if len(term) > 4 && strings.HasSuffix(term, "es") {
				termToCheck = term[:len(term)-2]
			} else if len(term) > 3 && strings.HasSuffix(term, "s") && !strings.HasSuffix(term, "ss") {
				termToCheck = term[:len(term)-1]
			}

			validTerms++
			if termInHuman(termToCheck) {
				matches++
			}
		}

		// If all significant terms matched (AND logic equivalent), include them.
		// If validTerms is 0 (e.g. query was just "Who are"), we might return empty or all.
		// Let's return empty to avoid noise.
		if validTerms > 0 && matches == validTerms {
			filtered = append(filtered, h)
		}
	}

	return nil, HumansResponse{Humans: filtered}, nil
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: "asian-americans-wiki", Version: "v1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "search-asian-americans", Description: "Search for Asian Americans by name, ethnicity, or tags"}, SearchAsianAmericans)

	log.Println("Starting server...")
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}