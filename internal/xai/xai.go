package xai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	openai "github.com/sashabaranov/go-openai"
)

type Client struct {
	client *openai.Client
	token  string
}

func New(token string) *Client {
	config := openai.DefaultConfig(token)
	config.BaseURL = "https://api.x.ai/v1"
	return &Client{
		client: openai.NewClientWithConfig(config),
		token:  token,
	}
}

type GenerateImageInput struct {
	Prompt string
	N      int
	Image  string // URL or base64 string
}

type imageEditRequest struct {
	Model       string `json:"model"`
	Image       any    `json:"image"`
	Prompt      string `json:"prompt"`
	N           int    `json:"n,omitempty"`
	AspectRatio string `json:"aspect_ratio,omitempty"`
}

func (c *Client) GenerateImage(ctx context.Context, input GenerateImageInput) ([]string, error) {
	var imageVal any = input.Image
	// xAI API expects a struct with a "url" key for the image field, 
	// which can be either a public URL or a data:image/... base64 string.
	if input.Image != "" {
		imageVal = struct {
			URL string `json:"url"`
		}{URL: input.Image}
	}

	reqBody := imageEditRequest{
		Model:       "grok-imagine-image",
		Image:       imageVal,
		Prompt:      input.Prompt,
		N:           input.N,
		AspectRatio: "1:1",
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.x.ai/v1/images/edits", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("xai api error (status %d): %s", resp.StatusCode, string(body))
	}

	var imageResp openai.ImageResponse
	if err := json.NewDecoder(resp.Body).Decode(&imageResp); err != nil {
		return nil, err
	}

	var urls []string
	for _, d := range imageResp.Data {
		urls = append(urls, d.URL)
	}

	return urls, nil
}

type AnalyzeImagesInput struct {
	ImageURLs []string
	Prompt    string
}

func (c *Client) AnalyzeImages(ctx context.Context, input AnalyzeImagesInput) (string, error) {
	messages := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleUser,
			MultiContent: []openai.ChatMessagePart{
				{
					Type: openai.ChatMessagePartTypeText,
					Text: input.Prompt,
				},
			},
		},
	}

	for _, url := range input.ImageURLs {
		messages[0].MultiContent = append(messages[0].MultiContent, openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{
				URL: url,
			},
		})
	}

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    "grok-2-vision-1212",
		Messages: messages,
	})
	if err != nil {
		return "", fmt.Errorf("unable to analyze images: %w", err)
	}

	return resp.Choices[0].Message.Content, nil
}