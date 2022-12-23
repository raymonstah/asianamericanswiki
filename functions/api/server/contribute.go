package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/httplog"

	"github.com/raymonstah/asianamericanswiki/internal/contributor"
	"github.com/raymonstah/asianamericanswiki/internal/openai"
)

type ContributeRequest struct {
	Name          string   `json:"name"`
	Aliases       []string `json:"aka"`
	Dob           string   `json:"dob"` // format 2000-12-01
	Tags          []string `json:"tags"`
	Website       string   `json:"website"`
	Ethnicity     []string `json:"ethnicity"`
	BirthLocation string   `json:"birthLocation"`
	Location      []string `json:"location"`
	Twitter       string   `json:"twitter"`
	Draft         bool     `json:"draft"`
	Description   string   `json:"description"`
}

type ContributeResponse struct {
	Link string `json:"link,omitempty"`
}

func validateContributeRequest(r io.Reader) (contributor.Post, error) {
	var input ContributeRequest
	if err := json.NewDecoder(r).Decode(&input); err != nil {
		return contributor.Post{}, err
	}

	if input.Name == "" {
		return contributor.Post{}, fmt.Errorf("name is required")
	}

	asBirthdate, err := toBirthdate(input.Dob)
	if err != nil {
		return contributor.Post{}, fmt.Errorf("birthday invalid: %w", err)
	}

	return contributor.Post{
		FrontMatter: contributor.FrontMatterInput{
			Name:          input.Name,
			Date:          time.Now(),
			Aliases:       input.Aliases,
			Dob:           asBirthdate,
			Tags:          input.Tags,
			Website:       input.Website,
			Ethnicity:     input.Ethnicity,
			BirthLocation: input.BirthLocation,
			Location:      input.Location,
			Twitter:       input.Twitter,
			Draft:         input.Draft,
		},
		Description: input.Description,
	}, nil
}

func (s Server) Contribute(w http.ResponseWriter, r *http.Request) (err error) {
	var (
		ctx   = r.Context()
		oplog = httplog.LogEntry(r.Context())
	)
	defer func(start time.Time) {
		oplog.Err(err).
			Str("request", "Contribute").
			Dur("duration", time.Since(start).Round(time.Millisecond)).
			Msg("completed request")
	}(time.Now())

	post, err := validateContributeRequest(r.Body)
	if err != nil {
		return NewBadRequestError(err)
	}

	if r.URL.Query().Get("test") != "" {
		switch r.URL.Query().Get("test") {
		case "dupe":
			return ErrorResponse{
				Status: http.StatusUnprocessableEntity,
				Err:    contributor.ErrBranchAlreadyExists,
			}
		default:
			w.WriteHeader(http.StatusCreated)
			resp := ContributeResponse{Link: "https://github.com/raymonstah/asianamericanswiki/pulls/1"}
			s.writeData(w, http.StatusOK, resp)
			return nil
		}
	}

	if post.Description == "" {
		generatedDescription, err := s.contributor.OpenAI.Generate(ctx, openai.GenerateInput{
			Tags: post.FrontMatter.Tags,
			Name: post.FrontMatter.Name,
		})
		if err != nil {
			return NewInternalServerError(fmt.Errorf("error generating description: %w", err))
		}

		post.FrontMatter.AIGenerated = true
		post.Description = generatedDescription
	}

	content, err := contributor.GenerateMarkdown(post.FrontMatter, post.Description)
	if err != nil {
		return NewInternalServerError(fmt.Errorf("unable to generate markdown: %w", err))
	}

	nameWithDashes := strings.ReplaceAll(post.FrontMatter.Name, " ", "-")
	path := fmt.Sprintf("content/humans/%s/index.md", nameWithDashes)
	url, err := s.contributor.PullRequestService.CreatePRWithContent(ctx, contributor.CreatePRWithContentInput{
		Name:        post.FrontMatter.Name,
		Path:        path,
		Content:     content,
		Branch:      strings.ToLower(nameWithDashes),
		AuthorName:  contributor.AuthorName,
		AuthorEmail: contributor.AuthorEmail,
		Subject:     post.FrontMatter.Name,
	})
	if err != nil {
		if errors.Is(err, contributor.ErrBranchAlreadyExists) ||
			errors.Is(err, contributor.ErrFileAlreadyExists) {
			return ErrorResponse{Status: http.StatusUnprocessableEntity, Err: err}
		}
		return NewInternalServerError(fmt.Errorf("error generating pull request: %w", err))
	}

	resp := ContributeResponse{Link: url}
	s.writeData(w, http.StatusCreated, resp)
	return nil
}

const birthdateLayout = "2006-01-02"

// toBirthdate converts a date formatted as "YYYY-MM-DD" to a time.Time.
func toBirthdate(date string) (time.Time, error) {
	if date == "" {
		return time.Time{}, nil
	}

	return time.Parse(birthdateLayout, date)
}
