package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/firestore"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/rs/zerolog"
	"github.com/segmentio/ksuid"
)

type Server struct {
	dao    *humandao.DAO
	logger zerolog.Logger
}

type MCPHuman struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Aliases     []string `json:"aliases,omitempty"`
	Path        string   `json:"path"`
	Draft       bool     `json:"draft"`
	DOB         string   `json:"dob,omitempty"`
	DOD         string   `json:"dod,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Ethnicity   []string `json:"ethnicity,omitempty"`
	Description string   `json:"description,omitempty"`
	Gender      string   `json:"gender,omitempty"`
	Instagram   string   `json:"instagram,omitempty"`
	Twitter     string   `json:"twitter,omitempty"`
	Website     string   `json:"website,omitempty"`
	IMDB        string   `json:"imdb,omitempty"`
}

func toMCPHuman(h humandao.Human) MCPHuman {
	return MCPHuman{
		ID:          h.ID,
		Name:        h.Name,
		Aliases:     h.Aliases,
		Path:        h.Path,
		Draft:       h.Draft,
		DOB:         h.DOB,
		DOD:         h.DOD,
		Tags:        h.Tags,
		Ethnicity:   h.Ethnicity,
		Description: h.Description,
		Gender:      string(h.Gender),
		Instagram:   h.Socials.Instagram,
		Twitter:     h.Socials.X,
		Website:     h.Socials.Website,
		IMDB:        h.Socials.IMDB,
	}
}

type HumansResponse struct {
	Humans []MCPHuman `json:"humans"`
}

type HumanResponse struct {
	Human MCPHuman `json:"human"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type SearchInput struct {
	Query string `json:"query" jsonschema:"The search query to filter Asian Americans by (e.g. 'Chinese rappers', 'actresses')"`
}

func (s *Server) searchAsianAmericans(ctx context.Context, req *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, HumansResponse, error) {
	humans, err := s.dao.ListHumans(ctx, humandao.ListHumansInput{Limit: 1000})
	if err != nil {
		return nil, HumansResponse{}, fmt.Errorf("failed to list humans: %w", err)
	}

	query := strings.ToLower(input.Query)
	terms := strings.Fields(query)
	var filtered []MCPHuman

	stopWords := map[string]bool{
		"who": true, "are": true, "some": true, "famous": true, "is": true, "a": true, "an": true, "the": true, "of": true, "in": true,
	}

	for _, h := range humans {
		matches := 0
		validTerms := 0
		for _, term := range terms {
			if stopWords[term] {
				continue
			}
			validTerms++
			termToCheck := term
			if len(term) > 4 && strings.HasSuffix(term, "es") {
				termToCheck = term[:len(term)-2]
			} else if len(term) > 3 && strings.HasSuffix(term, "s") && !strings.HasSuffix(term, "ss") {
				termToCheck = term[:len(term)-1]
			}

			found := strings.Contains(strings.ToLower(h.Name), termToCheck) ||
				strings.Contains(strings.ToLower(h.Description), termToCheck)
			if !found {
				for _, eth := range h.Ethnicity {
					if strings.Contains(strings.ToLower(eth), termToCheck) {
						found = true
						break
					}
				}
			}
			if !found {
				for _, tag := range h.Tags {
					if strings.Contains(strings.ToLower(tag), termToCheck) {
						found = true
						break
					}
				}
			}

			if found {
				matches++
			}
		}

		if validTerms > 0 && matches == validTerms {
			filtered = append(filtered, toMCPHuman(h))
		}
	}

	return nil, HumansResponse{Humans: filtered}, nil
}

type GetInput struct {
	ID   string `json:"id,omitempty" jsonschema:"The ID of the human"`
	Path string `json:"path,omitempty" jsonschema:"The URN path of the human (e.g. 'bruce-lee')"`
}

func (s *Server) getHuman(ctx context.Context, req *mcp.CallToolRequest, input GetInput) (*mcp.CallToolResult, HumanResponse, error) {
	human, err := s.dao.Human(ctx, humandao.HumanInput{HumanID: input.ID, Path: input.Path})
	if err != nil {
		return nil, HumanResponse{}, fmt.Errorf("failed to get human: %w", err)
	}
	return nil, HumanResponse{Human: toMCPHuman(human)}, nil
}

type AddInput struct {
	Name        string   `json:"name" jsonschema:"Full name of the person"`
	Aliases     []string `json:"aliases,omitempty" jsonschema:"Alternative names or nicknames"`
	DOB         string   `json:"dob,omitempty" jsonschema:"Date of birth (YYYY-MM-DD)"`
	DOD         string   `json:"dod,omitempty" jsonschema:"Date of death (YYYY-MM-DD)"`
	Ethnicity   []string `json:"ethnicity" jsonschema:"List of ethnicities"`
	Description string   `json:"description" jsonschema:"Short biography or description"`
	Tags        []string `json:"tags,omitempty" jsonschema:"Relevant tags (e.g. actor, musician)"`
	Gender      string   `json:"gender" jsonschema:"male, female, or nonbinary"`
	Instagram   string   `json:"instagram,omitempty" jsonschema:"Instagram profile URL"`
	Twitter     string   `json:"twitter,omitempty" jsonschema:"Twitter profile URL"`
	Website     string   `json:"website,omitempty" jsonschema:"Personal website"`
	IMDB        string   `json:"imdb,omitempty" jsonschema:"IMDB profile URL"`
}

func (s *Server) addHuman(ctx context.Context, req *mcp.CallToolRequest, input AddInput) (*mcp.CallToolResult, MessageResponse, error) {
	humanID := ksuid.New().String()
	if err := validateSocials(input.Instagram, input.Twitter, input.Website, input.IMDB); err != nil {
		return nil, MessageResponse{}, err
	}
	human, err := s.dao.AddHuman(ctx, humandao.AddHumanInput{
		HumanID:     humanID,
		Name:        input.Name,
		Aliases:     input.Aliases,
		DOB:         input.DOB,
		DOD:         input.DOD,
		Ethnicity:   input.Ethnicity,
		Description: input.Description,
		Tags:        input.Tags,
		Gender:      humandao.Gender(input.Gender),
		Instagram:   input.Instagram,
		Twitter:     input.Twitter,
		Website:     input.Website,
		IMDB:        input.IMDB,
		Draft:       true, // Agents should probably contribute as drafts first
		CreatedBy:   "mcp-agent",
	})
	if err != nil {
		return nil, MessageResponse{}, fmt.Errorf("failed to add human: %w", err)
	}
	return nil, MessageResponse{Message: fmt.Sprintf("Successfully added human %s (ID: %s) as a draft.", human.Name, human.ID)}, nil
}

type UpdateInput struct {
	ID          string   `json:"id" jsonschema:"The ID of the human to update"`
	Name        string   `json:"name,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
	Draft       *bool    `json:"draft,omitempty"`
	DOB         string   `json:"dob,omitempty"`
	DOD         string   `json:"dod,omitempty"`
	Ethnicity   []string `json:"ethnicity,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Gender      string   `json:"gender,omitempty"`
	Instagram   string   `json:"instagram,omitempty" jsonschema:"Instagram profile URL"`
	Twitter     string   `json:"twitter,omitempty" jsonschema:"Twitter profile URL"`
	Website     string   `json:"website,omitempty"`
	IMDB        string   `json:"imdb,omitempty"`
}

func (s *Server) updateHuman(ctx context.Context, req *mcp.CallToolRequest, input UpdateInput) (*mcp.CallToolResult, MessageResponse, error) {
	human, err := s.dao.Human(ctx, humandao.HumanInput{HumanID: input.ID})
	if err != nil {
		return nil, MessageResponse{}, fmt.Errorf("failed to get human for update: %w", err)
	}

	if err := validateSocials(input.Instagram, input.Twitter, input.Website, input.IMDB); err != nil {
		return nil, MessageResponse{}, err
	}

	if input.Name != "" {
		human.Name = input.Name
	}
	if len(input.Aliases) > 0 {
		human.Aliases = input.Aliases
	}
	if input.Draft != nil {
		human.Draft = *input.Draft
	}
	if input.DOB != "" {
		human.DOB = input.DOB
	}
	if input.DOD != "" {
		human.DOD = input.DOD
	}
	if len(input.Ethnicity) > 0 {
		human.Ethnicity = input.Ethnicity
	}
	if input.Description != "" {
		human.Description = input.Description
	}
	if len(input.Tags) > 0 {
		human.Tags = input.Tags
	}
	if input.Gender != "" {
		human.Gender = humandao.Gender(input.Gender)
	}
	if input.Instagram != "" {
		human.Socials.Instagram = input.Instagram
	}
	if input.Twitter != "" {
		human.Socials.X = input.Twitter
	}
	if input.Website != "" {
		human.Socials.Website = input.Website
	}
	if input.IMDB != "" {
		human.Socials.IMDB = input.IMDB
	}

	err = s.dao.UpdateHuman(ctx, human)
	if err != nil {
		return nil, MessageResponse{}, fmt.Errorf("failed to update human: %w", err)
	}
	return nil, MessageResponse{Message: fmt.Sprintf("Successfully updated human %s (ID: %s).", human.Name, human.ID)}, nil
}

func validateSocials(instagram, twitter, website, imdb string) error {
	fields := []struct {
		name  string
		value string
	}{
		{"instagram", instagram},
		{"twitter", twitter},
		{"website", website},
		{"imdb", imdb},
	}

	for _, field := range fields {
		if field.value != "" && !strings.HasPrefix(field.value, "http") {
			return fmt.Errorf("%s must be a full URL (starting with http or https)", field.name)
		}
	}
	return nil
}

type ListInput struct {
	Limit  int `json:"limit,omitempty" jsonschema:"Max number of results (default 50, max 100)"`
	Offset int `json:"offset,omitempty" jsonschema:"Number of results to skip"`
}

func (s *Server) listHumans(ctx context.Context, req *mcp.CallToolRequest, input ListInput) (*mcp.CallToolResult, HumansResponse, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	humans, err := s.dao.ListHumans(ctx, humandao.ListHumansInput{
		Limit:  limit,
		Offset: input.Offset,
	})
	if err != nil {
		return nil, HumansResponse{}, fmt.Errorf("failed to list humans: %w", err)
	}
	var out []MCPHuman
	for _, h := range humans {
		out = append(out, toMCPHuman(h))
	}
	return nil, HumansResponse{Humans: out}, nil
}

func main() {
	ctx := context.Background()
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	fsClient, err := firestore.NewClient(ctx, api.ProjectID)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create firestore client")
	}
	defer func() { _ = fsClient.Close() }()

	dao := humandao.NewDAO(fsClient)
	mcpServer := &Server{
		dao:    dao,
		logger: logger,
	}

	implementation := &mcp.Implementation{
		Name:    "asian-americans-wiki",
		Version: "v2.0.0",
	}

	s := mcp.NewServer(implementation, nil)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "search-asian-americans",
		Description: "Search for Asian Americans by name, ethnicity, or tags",
	}, mcpServer.searchAsianAmericans)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get-human",
		Description: "Get detailed information about a specific Asian American by ID or path",
	}, mcpServer.getHuman)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "add-human",
		Description: "Contribute a new Asian American entry to the wiki (as a draft)",
	}, mcpServer.addHuman)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update-human",
		Description: "Update an existing Asian American entry",
	}, mcpServer.updateHuman)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list-humans",
		Description: "List Asian Americans with pagination",
	}, mcpServer.listHumans)

	if err := s.Run(ctx, &mcp.StdioTransport{}); err != nil {
		logger.Fatal().Err(err).Msg("MCP server failed")
	}
}
