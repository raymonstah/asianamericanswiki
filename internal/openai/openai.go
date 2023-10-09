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

var prompt = `
Your task is to write a few paragraphs about %v.
Here are some tags to help you identify this person: "%v".
Your tone should neutral, like a biographer or Wikipedia style.
Focus on providing factual information based on reliable sources. You do not need to site your sources at the end.
Try to limit your response to two to four paragraphs. In your paragraphs you should include:
1. What is their ethnicity and background?
2. Where are they from?
3. What are they known for?
4. Why should people care about them?
5. What is their impact on Asian Americans?

Do not make up answers. If the person is too ambiguous, because there are multiple people with the same name,
or because you don't know anything about this person, respond with the text: "error: ", and tell me why.
`

type GenerateInput struct {
	Tags []string
	Name string
}

func (c *Client) Generate(ctx context.Context, input GenerateInput) (string, error) {
	tagWithCommas := strings.Join(input.Tags, ", ")
	prompt := fmt.Sprintf(prompt, input.Name, tagWithCommas)
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

	response := resp.Choices[0].Message.Content
	if strings.HasPrefix(strings.ToLower(response), "error: ") {
		return "", fmt.Errorf("unable to generate description: %v", response)
	}

	return strings.TrimSpace(response), nil
}

var jsonPrompt = `
Your task is to provide a JSON response about a person, %v.
Here are some tags to help you identify this person: "%v".

Provide a JSON response with the following keys, for the person %q:
* name: the name of the person
* dob: the date of birth of the person in the format "YYYY-MM-DD". If you know only the year, use "YYYY". If you know only the year and month, use "YYYY-MM".
* dod: the date of death of the person, if they died.
* ethnicity: an array containing the ethnicity of the person. Provide multiple if they are mixed.
* location: an array of locations where the person was born, lived or is based out of.
* tags: an array of relevant tags to help identify the person, such as "actor", "activist", "politician", etc.
* website: the website of the person, if they have one.
* twitter: the twitter handle of the person, if they have one.

If any keys are missing, use an empty string instead.
Do not make up answers. If the person is too ambiguous, because there are multiple people with the same name,
or because you don't know anything about this person, respond with the text: "error: ", and tell me why.
`

type GenerateCreateRequest struct {
	Tags []string
	Name string
}

func (c *Client) GenerateCreateRequest(ctx context.Context, input GenerateCreateRequest) ([]byte, error) {
	tagWithCommas := strings.Join(input.Tags, ", ")
	prompt := fmt.Sprintf(jsonPrompt, input.Name, tagWithCommas)
	resp, err := c.openAiClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:     openai.GPT3Dot5Turbo,
		MaxTokens: 300,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create chat completion: %w", err)
	}

	response := resp.Choices[0].Message.Content
	if strings.HasPrefix(strings.ToLower(response), "error: ") {
		return nil, fmt.Errorf("unable to generate description: %v", response)
	}

	return []byte(response), nil
}
