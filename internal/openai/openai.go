package openai

import (
	"context"
	"fmt"
	"strings"

	gogpt "github.com/sashabaranov/go-gpt3"
)

type Client struct {
	openAiClient *gogpt.Client
}

func New(token string) *Client {
	openAiClient := gogpt.NewClient(token)
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
	req := gogpt.CompletionRequest{
		Model:     "text-davinci-003",
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