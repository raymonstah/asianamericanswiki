package contributer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	// gcp requires this when vendoring.
	_ "github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
)

const (
	repo        = "asianamericanswiki"
	repoOwner   = "raymonstah"
	authorName  = "asianamericanswiki-bot"
	authorEmail = "dne@asianamericans.wiki"
	branchTo    = "main"
)

type PullRequester interface {
	createPRWithContent(ctx context.Context, input createPRWithContentInput) (string, error)
}

// used for testing
var mockPrServiceKey struct{}

// Handle is the signature required for GCP Cloud function.
func Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := os.Getenv("GITHUB_AUTH_TOKEN")

	var pullRequester PullRequester
	if value, ok := ctx.Value(mockPrServiceKey).(PullRequester); ok {
		pullRequester = value
		token = "blah"
	} else {
		pullRequester = NewPullRequestService(ctx, token)
	}

	if token == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var input struct {
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

		Description string `json:"description`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, err)
		return
	}

	if input.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "name is required")
		return
	}

	asBirthdate, err := toBirthdate(input.Dob)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "birthdate invalid: %s", err)
		return
	}

	content, err := generateMarkdown(frontMatterInput{
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
	}, input.Description)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "unable to generate markdown: %s", err)
		return
	}

	nameWithDashes := strings.ReplaceAll(input.Name, " ", "-")
	path := fmt.Sprintf("content/humans/%s/index.md", nameWithDashes)
	url, err := pullRequester.createPRWithContent(ctx, createPRWithContentInput{
		Name:        input.Name,
		Path:        path,
		Content:     content,
		Branch:      strings.ToLower(nameWithDashes),
		AuthorName:  authorName,
		AuthorEmail: authorEmail,
		Subject:     input.Name,
	})
	if err != nil {
		if errors.Is(err, ErrBranchAlreadyExists) ||
			errors.Is(err, ErrFileAlreadyExists) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprintln(w, err.Error())
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error generating pull request: %s", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "pull request created: %s", url)
}

// toBirthdate converts a date formatted as "YYYY-MM-DD" to a time.Time.
func toBirthdate(date string) (time.Time, error) {
	if date == "" {
		return time.Time{}, nil
	}

	return time.Parse(birthdateLayout, date)
}
