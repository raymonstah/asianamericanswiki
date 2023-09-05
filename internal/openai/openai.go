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
	prompt := fmt.Sprintf("Write a two paragraph summary about Asian American %v focusing on the factual information. Use the following tags:, %v", input.Name, tagWithCommas)
	resp, err := c.openAiClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:     openai.GPT3Dot5Turbo,
		MaxTokens: 500,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("unable to create chat completion: %w", err)
	}

	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}
