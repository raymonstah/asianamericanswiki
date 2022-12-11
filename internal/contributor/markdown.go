package contributor

import (
	"bytes"
	"time"

	"gopkg.in/yaml.v3"
)

// GenerateMarkdown generates a markdown byte string.
func GenerateMarkdown(fm FrontMatterInput, description string) ([]byte, error) {
	var stream bytes.Buffer
	humanYaml, err := yaml.Marshal(fm.yaml())
	if err != nil {
		return nil, err
	}

	stream.WriteString("---\n")
	stream.Write(humanYaml)
	stream.WriteString("---\n\n")
	stream.WriteString(description)

	return stream.Bytes(), nil
}

type Post struct {
	FrontMatter FrontMatterInput
	Description string
}

// FrontMatterInput is the stuff at the beginning of all hugo markdown files which provides metadata.
type FrontMatterInput struct {
	Name          string
	Date          time.Time
	Aliases       []string
	Dob           time.Time
	Tags          []string
	Website       string
	Ethnicity     []string
	BirthLocation string
	Location      []string
	Twitter       string
	Draft         bool
	AIGenerated   bool
}

type frontMatterOutput struct {
	Name          string   `yaml:"title"`
	Date          string   `yaml:"date"`
	Aliases       []string `yaml:"aka,flow"`
	Dob           string   `yaml:"dob"`
	Tags          []string `yaml:"tags,flow"`
	Website       string   `yaml:"website"`
	Ethnicity     []string `yaml:"ethnicity,flow"`
	BirthLocation string   `yaml:"birthLocation"`
	Location      []string `yaml:"location,flow"`
	Twitter       string   `yaml:"twitter"`
	Draft         bool     `yaml:"draft"`
	AIGenerated   bool     `yaml:"ai_generated"`
}

func (frontMatter FrontMatterInput) yaml() frontMatterOutput {
	output := frontMatterOutput{
		Name:          frontMatter.Name,
		Date:          frontMatter.Date.Format("2006-01-02T15:04:05"),
		Aliases:       frontMatter.Aliases,
		Dob:           birthdate(frontMatter.Dob),
		Tags:          frontMatter.Tags,
		Website:       frontMatter.Website,
		Ethnicity:     frontMatter.Ethnicity,
		BirthLocation: frontMatter.BirthLocation,
		Location:      frontMatter.Location,
		Twitter:       frontMatter.Twitter,
		Draft:         frontMatter.Draft,
		AIGenerated:   frontMatter.AIGenerated,
	}

	return output
}

const birthdateLayout = "2006-01-02"

func birthdate(date time.Time) string {
	if date.IsZero() {
		return "YYYY-MM-DD"
	}

	return date.Format(birthdateLayout)
}
