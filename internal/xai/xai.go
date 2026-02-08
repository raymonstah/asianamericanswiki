package xai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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
	var imageVal any
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
		Model:    "grok-3",
		Messages: messages,
	})
	if err != nil {
		return "", fmt.Errorf("unable to analyze images: %w", err)
	}

	return resp.Choices[0].Message.Content, nil
}

func DefaultImagePrompt(name string) string {
	return fmt.Sprintf("A cinematic portrait of %s, dark gray background, high quality, 8k, highly detailed, professional lighting.", name)
}

	

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

	

	var humanPrompt = `

	Your task is to write a few paragraphs about %v.

	Here are some tags to help you identify this person: "%v".

	Your tone should neutral, like a biographer or how a Wikipedia article is written.

	Focus on providing factual information based on reliable sources. You do not need to site your sources at the end.

	Try to limit your response to two to four paragraphs. In your paragraphs you should include:

	1. What is their ethnicity and background?

	2. Where are they from?

	3. What are they known for?

	4. What is their impact on Asian Americans?\n\n	IMPORTANT: \n	- Provide the individual\'s full legal name if known (e.g., "Ryan Higa" instead of "Nigahiga"). \n	- If the subject is a group or company instead of an individual human, respond with "error: subject is not an individual human".

	

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

	

	func (c *Client) GenerateHuman(ctx context.Context, input GenerateHumanRequest) (GeneratedHumanResponse, error) {

		tagWithCommas := strings.Join(input.Tags, ", ")

		p := fmt.Sprintf(humanPrompt, input.Name, tagWithCommas)

				resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{

					Model: "grok-3",

					Messages: []openai.ChatCompletionMessage{

						{Role: openai.ChatMessageRoleUser, Content: p},

					},

				})

		

		if err != nil {

			return GeneratedHumanResponse{}, fmt.Errorf("unable to create chat completion: %w", err)

		}

	

		response := resp.Choices[0].Message.Content

		// Extract JSON if it's wrapped in triple dashes

		if strings.Contains(response, "---") {

			parts := strings.Split(response, "---")

			if len(parts) >= 3 {

				response = parts[1]

			}

		}

	

		if strings.HasPrefix(strings.ToLower(response), "error: ") {

			return GeneratedHumanResponse{}, fmt.Errorf("unable to generate response: %v", response)

		}

	

		var generatedHuman GeneratedHumanResponse

		if err := json.Unmarshal([]byte(response), &generatedHuman); err != nil {

			return GeneratedHumanResponse{}, fmt.Errorf("unable to unmarshal response: %w", err)

		}

	

		return generatedHuman, nil

	}

	

	type BrainstormInput struct {

		Query string

	}

	

	func (c *Client) Brainstorm(ctx context.Context, input BrainstormInput) ([]string, error) {

	

		prompt := fmt.Sprintf("Your task is to provide a list of notable Asian Americans for the following query: %v. "+

	

			"Return only the full names of individual people, one per line. Do not include groups, companies, or artistic aliases if the full name is known. Do not include any other text.", input.Query)

	

	

	

		resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{

	

	

			Model: "grok-3",

			Messages: []openai.ChatCompletionMessage{

				{Role: openai.ChatMessageRoleUser, Content: prompt},

			},

		})

		if err != nil {

			return nil, fmt.Errorf("unable to create chat completion: %w", err)

		}

	

		lines := strings.Split(resp.Choices[0].Message.Content, "\n")

		var names []string

		for _, line := range lines {

			name := strings.TrimSpace(line)

			if name != "" {

				names = append(names, name)

			}

		}

	

		return names, nil

	}

	