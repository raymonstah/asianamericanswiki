package contributor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
)

const (
	repo        = "asianamericanswiki"
	repoOwner   = "raymonstah"
	authorName  = "asianamericanswiki-bot"
	authorEmail = "dne@asianamericans.wiki"
	branchTo    = "main"
)

func init() {
	var (
		token = os.Getenv("GITHUB_AUTH_TOKEN")
		ctx   = context.Background()
		h     = Handler{
			PullRequestService: NewPullRequestService(ctx, token),
		}
	)

	functions.HTTP("Handle", h.Handle)
}

type Handler struct {
	PullRequestService PullRequestService
}

// Handle is the signature required for GCP Cloud function.
func (h Handler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, fmt.Errorf("http method must be POST"))
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

		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		errorResponse(w, http.StatusBadRequest, err)

		return
	}

	if input.Name == "" {
		errorResponse(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}

	asBirthdate, err := toBirthdate(input.Dob)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, fmt.Errorf("birthday invalid: %w", err))
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
		errorResponse(w, http.StatusInternalServerError, fmt.Errorf("unable to generate markdown: %w", err))
		return
	}

	nameWithDashes := strings.ReplaceAll(input.Name, " ", "-")
	path := fmt.Sprintf("content/humans/%s/index.md", nameWithDashes)
	url, err := h.PullRequestService.createPRWithContent(ctx, createPRWithContentInput{
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
			errorResponse(w, http.StatusUnprocessableEntity, err)
			return
		}
		errorResponse(w, http.StatusInternalServerError,
			fmt.Errorf("error generating pull request: %w", err))
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	resp := response{Link: url}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		errorResponse(w, http.StatusInternalServerError, err)
		return
	}
}

// toBirthdate converts a date formatted as "YYYY-MM-DD" to a time.Time.
func toBirthdate(date string) (time.Time, error) {
	if date == "" {
		return time.Time{}, nil
	}

	return time.Parse(birthdateLayout, date)
}

type response struct {
	Link string `json:"link,omitempty"`
}

func errorResponse(w http.ResponseWriter, statusCode int, err error) {
	log.Printf("error: %v\n", err.Error())
	w.Header().Set("Content-Type", "application/json charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(statusCode)
	response := struct {
		Error string `json:"error"`
	}{
		Error: err.Error(),
	}

	_ = json.NewEncoder(w).Encode(response)
}
