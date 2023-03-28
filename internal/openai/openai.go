package openai

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

type Client struct {
	openAiClient *openai.Client
}

func New(token string) *Client {
	openAiClient := openai.NewClient(token)
	return &Client{
		openAiClient: openAiClient,
	}
}

type GenerateInput struct {
	Tags []string
	Name string
}

func (c *Client) Generate(ctx context.Context, input GenerateInput) (string, error) {
	tagWithCommas := strings.Join(input.Tags, ", ")
	prompt := fmt.Sprintf("Write a wikipedia entry about Asian American %v, %v", tagWithCommas, input.Name)
	req := openai.CompletionRequest{
		Model:     openai.GPT4,
		MaxTokens: 500,
		Prompt:    prompt,
		BestOf:    3,
	}

	resp, err := c.openAiClient.CreateCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("unable to create completion: %w", err)
	}

	return strings.TrimSpace(resp.Choices[0].Text), nil
}
