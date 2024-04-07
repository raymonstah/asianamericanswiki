package openai

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-json-experiment/json"
	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
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
Your tone should neutral, like a biographer or how a Wikipedia article is written.
Focus on providing factual information based on reliable sources. You do not need to site your sources at the end.
Try to limit your response to two to four paragraphs. In your paragraphs you should include:
1. What is their ethnicity and background?
2. Where are they from?
3. What are they known for?
4. What is their impact on Asian Americans?

Save your paragraphs as the JSON key "description", replacing new lines with double line breaks: "\n\n".

Along with the "description" key, Provide a JSON response with the following keys:
* name: the name of the person
* gender: the gender of the person, one of ["male", "female", "nonbinary"]
* dob: the date of birth of the person in the format "YYYY-MM-DD". If you know only the year, use "YYYY". If you know only the year and month, use "YYYY-MM".
* dod: the date of death of the person, if they died.
* ethnicity: an array containing the ethnicity of the person. Provide multiple if they are mixed. Examples include: ["Chinese", "Korean", "Vietnamese"].
* location: an array of cities where the person was born, lived or is based out of.
* tags: an array of relevant tags to help identify the person, such as "actor", "activist", "politician", etc.
* website: the website of the person, if they have one.
* twitter: the twitter handle of the person, if they have one, in the format of "https://twitter.com/{handle}"

Your output should follow the following JSON template between the triple dashes:
---
{
	"name": "",
	"gender": "",
	"dob": "",
	"dod": "",
	"ethnicity": [],
	"description": "",
	"location": [],
	"website": "",
	"twitter": "",
	"tags": []
}
---

If any keys are missing, use an empty string instead.
Do not make up answers. If the person is too ambiguous, because there are multiple people with the same name,
or because you don't know anything about this person, respond with the text: "error: ", and tell me why.

Ensure your json response is valid. Don't forget adding a comma after the "description" key.
`

type GenerateHumanRequest struct {
	Tags []string
	Name string
}

type GeneratedHumanResponse struct {
	Name        string   `json:"name"`
	DOB         string   `json:"dob"`
	DOD         string   `json:"dod"`
	Ethnicity   []string `json:"ethnicity"`
	Description string   `json:"description"`
	Location    []string `json:"location"`
	Website     string   `json:"website"`
	Twitter     string   `json:"twitter"`
	Tags        []string `json:"tags"`
	Gender      string   `json:"gender"`
}

func (c *Client) GenerateHuman(ctx context.Context, input GenerateHumanRequest) (GeneratedHumanResponse, error) {
	slog.Info("generating human from openai", "name", input.Name, "tags", input.Tags)
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
		return GeneratedHumanResponse{}, fmt.Errorf("unable to create chat completion: %w", err)
	}

	response := resp.Choices[0].Message.Content
	if strings.HasPrefix(strings.ToLower(response), "error: ") {
		return GeneratedHumanResponse{}, fmt.Errorf("unable to generate response: %v", response)
	}

	var generatedHuman GeneratedHumanResponse
	if err := json.Unmarshal([]byte(response), &generatedHuman, json.DefaultOptionsV2()); err != nil {
		return GeneratedHumanResponse{}, fmt.Errorf("unable to unmarshal response: %w", err)
	}

	return generatedHuman, nil
}

type AddHumanRequest struct {
	Name        string   `json:"name"`
	DOB         string   `json:"dob"`
	DOD         string   `json:"dod"`
	Ethnicity   []string `json:"ethnicity"`
	Description string   `json:"description"`
	Gender      string   `json:"gender"`
}

type FromTextInput struct {
	Data string
}

// FromText generates a human add request by invoking OpenAI with a function so that it can conform to a jsonspec.
func (c *Client) FromText(ctx context.Context, input FromTextInput) (AddHumanRequest, error) {
	completion, err := c.openAiClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: "gpt-3.5-turbo",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: fmt.Sprintf("Given the data below, help me generate an add human request.\n\n%s", input.Data),
			},
		},
		Tools: []openai.Tool{
			{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name: "generate-human",
					Parameters: jsonschema.Definition{
						Type: jsonschema.Object,
						Properties: map[string]jsonschema.Definition{
							"name": {
								Type:        jsonschema.String,
								Description: "The name of the human",
							},
							"dob": {
								Type:        jsonschema.String,
								Description: "The date of birth of the human, in the format of YYYY-MM-DD",
							},
							"dod": {
								Type:        jsonschema.String,
								Description: "The date the human died (if applicable), in the format of YYYY-MM-DD",
							},
							"description": {
								Type:        jsonschema.String,
								Description: "A brief summary of the human, in no more than 250 words.",
							},
							"gender": {
								Type:        jsonschema.String,
								Description: "The gender of the human",
								Enum:        []string{"male", "female", "nonbinary"},
							},
							"ethnicity": {
								Type:        jsonschema.Array,
								Description: "A list of the human's ethnicity.",
								Items: &jsonschema.Definition{
									Type:        jsonschema.String,
									Description: "An ethnicity",
								},
							},
						},
						Required: []string{"name", "gender"},
					},
				},
			},
		},
	})
	if err != nil {
		return AddHumanRequest{}, fmt.Errorf("unable to generate human from openAI: %w", err)
	}

	response := completion.Choices[0].Message.ToolCalls[0].Function.Arguments
	var addHumanRequest AddHumanRequest
	if err := json.Unmarshal([]byte(response), &addHumanRequest); err != nil {
		return AddHumanRequest{}, fmt.Errorf("unable to unmarshal json from openAI: %w", err)
	}

	for i, ethnicity := range addHumanRequest.Ethnicity {
		addHumanRequest.Ethnicity[i] = strings.ToLower(ethnicity)
	}

	return addHumanRequest, nil
}
