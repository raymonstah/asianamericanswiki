package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/raymonstah/asianamericanswiki/internal/imageutil"
	"github.com/raymonstah/asianamericanswiki/internal/xai"
	"github.com/rs/zerolog"
	"github.com/segmentio/ksuid"
)

type Server struct {
	dao       *humandao.DAO
	logger    zerolog.Logger
	xaiClient *xai.Client
	uploader  *imageutil.Uploader
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
	SourceImage string   `json:"source_image,omitempty" jsonschema:"Direct URL to a high-quality portrait image to be used as a source for xAI cinematic portrait generation"`
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

	if input.SourceImage != "" {
		human, err = s.generateAndUploadImage(ctx, human, input.SourceImage)
		if err != nil {
			s.logger.Error().Err(err).Str("id", human.ID).Msg("Failed to generate image during addHuman")
			return nil, MessageResponse{Message: fmt.Sprintf("Successfully added human %s (ID: %s) as a draft, but failed to generate image: %v", human.Name, human.ID, err)}, nil
		}
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
	SourceImage string   `json:"source_image,omitempty" jsonschema:"Direct URL to a high-quality portrait image to be used as a source for xAI cinematic portrait generation"`
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

	if input.SourceImage != "" {
		human, err = s.generateAndUploadImage(ctx, human, input.SourceImage)
		if err != nil {
			s.logger.Error().Err(err).Str("id", human.ID).Msg("Failed to generate image during updateHuman")
			return nil, MessageResponse{Message: fmt.Sprintf("Successfully updated human %s (ID: %s), but failed to generate image: %v", human.Name, human.ID, err)}, nil
		}
	}

	return nil, MessageResponse{Message: fmt.Sprintf("Successfully updated human %s (ID: %s).", human.Name, human.ID)}, nil
}

func (s *Server) generateAndUploadImage(ctx context.Context, human humandao.Human, sourceURL string) (humandao.Human, error) {
	if s.xaiClient == nil || s.uploader == nil {
		return human, fmt.Errorf("xAI client or image uploader not configured")
	}

	prompt := xai.DefaultImagePrompt(human.Name)

	s.logger.Info().Str("source_url", sourceURL).Msg("Downloading source image")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return human, fmt.Errorf("unable to create request for source image: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return human, fmt.Errorf("unable to download source image: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return human, fmt.Errorf("unexpected status code downloading source image: %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return human, fmt.Errorf("failed to read source image: %w", err)
	}
	
	base64Data := base64.StdEncoding.EncodeToString(data)
	mimeType := http.DetectContentType(data)
	baseImage := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

	s.logger.Info().Msg("Requesting image generation from xAI")
	imageURLs, err := s.xaiClient.GenerateImage(ctx, xai.GenerateImageInput{
		Prompt: prompt,
		N:      1,
		Image:  baseImage,
	})
	if err != nil {
		return human, fmt.Errorf("unable to generate image: %w", err)
	}
	if len(imageURLs) == 0 {
		return human, fmt.Errorf("no image URLs returned from xAI")
	}

	s.logger.Info().Str("url", imageURLs[0]).Msg("Downloading generated image")
	resp, err = http.Get(imageURLs[0])
	if err != nil {
		return human, fmt.Errorf("unable to download generated image: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	rawImage, err := io.ReadAll(resp.Body)
	if err != nil {
		return human, fmt.Errorf("unable to read generated image: %w", err)
	}

	s.logger.Info().Msg("Uploading image to storage")
	return s.uploader.UploadHumanImages(ctx, human, rawImage)
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
	
	var xClient *xai.Client
	xaiToken := os.Getenv("XAI_API_KEY")
	if xaiToken != "" {
		xClient = xai.New(xaiToken)
	}

	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create storage client")
	}
	
	storageURL := "https://storage.googleapis.com"
	if os.Getenv("STORAGE_EMULATOR_HOST") != "" {
		storageURL = "http://" + os.Getenv("STORAGE_EMULATOR_HOST")
	}
	uploader := imageutil.NewUploader(storageClient, dao, storageURL)

	mcpServer := &Server{
		dao:       dao,
		logger:    logger,
		xaiClient: xClient,
		uploader:  uploader,
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
