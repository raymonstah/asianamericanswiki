package contributor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v40/github"
	"golang.org/x/oauth2"
)

var (
	ErrBranchAlreadyExists = errors.New("branch already exists")
	ErrFileAlreadyExists   = errors.New("file already exists")
)

type PullRequestService interface {
	createPRWithContent(ctx context.Context, input createPRWithContentInput) (string, error)
}

type DefaultPullRequestService struct {
	client *github.Client
}

func NewPullRequestService(ctx context.Context, token string) DefaultPullRequestService {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	return DefaultPullRequestService{client: client}
}

type createPRWithContentInput struct {
	Name        string
	Path        string
	Content     []byte
	Branch      string
	AuthorName  string
	AuthorEmail string
	Subject     string
}

func (prService DefaultPullRequestService) createPRWithContent(ctx context.Context, input createPRWithContentInput) (string, error) {
	var baseRef *github.Reference
	var err error
	if baseRef, _, err = prService.client.Git.GetRef(ctx, repoOwner, repo, "refs/heads/"+branchTo); err != nil {
		return "", fmt.Errorf("could not get reference to main branch: %w", err)
	}

	remoteBranch := fmt.Sprintf("refs/heads/%s", input.Branch)
	newRef := &github.Reference{Ref: &remoteBranch, Object: &github.GitObject{SHA: baseRef.Object.SHA}}
	_, resp, err := prService.client.Git.CreateRef(ctx, repoOwner, repo, newRef)
	if err != nil {
		if resp.StatusCode == http.StatusUnprocessableEntity {
			return "", ErrBranchAlreadyExists
		}
		return "", fmt.Errorf("error creating new branch: %w", err)
	}

	if err := prService.createFile(ctx, createFileInput{
		owner:         repoOwner,
		repository:    repo,
		path:          strings.ToLower(input.Path),
		content:       input.Content,
		commitMessage: fmt.Sprintf("system generated: adding %s", input.Name),
		branch:        input.Branch,
		authorName:    input.AuthorName,
		authorEmail:   input.AuthorEmail,
	}); err != nil {
		if err == ErrFileAlreadyExists {
			return "", err
		}
		return "", fmt.Errorf("unable to create file: %w", err)
	}

	url, err := prService.createPR(ctx, createPRInput{
		prSubject:     input.Subject,
		prRepoOwner:   repoOwner,
		prDescription: "",
		branchFrom:    input.Branch,
		branchTo:      branchTo,
		repo:          repo,
	})
	if err != nil {
		return "", fmt.Errorf("unable to create pr: %w", err)
	}

	return url, nil
}

type createFileInput struct {
	owner         string // owner of repo
	repository    string // repo name
	path          string // file path name
	content       []byte
	commitMessage string
	branch        string
	authorName    string
	authorEmail   string
}

func (prService DefaultPullRequestService) createFile(ctx context.Context, input createFileInput) error {

	fileContent, _, resp, err := prService.client.Repositories.GetContents(ctx, input.owner, input.repository, input.path, nil)
	if err != nil {
		if resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("error checking to see if file exists: %w", err)
		}
	}
	if fileContent != nil {
		return ErrFileAlreadyExists
	}

	// Note: the file needs to be absent from the repository as you are not
	// specifying a SHA reference here.
	opts := &github.RepositoryContentFileOptions{
		Message:   &input.commitMessage,
		Content:   input.content,
		Branch:    &input.branch,
		Committer: &github.CommitAuthor{Name: &input.authorName, Email: &input.authorEmail},
	}
	_, _, err = prService.client.Repositories.CreateFile(ctx, input.owner, input.repository, input.path, opts)
	if err != nil {
		return err

	}
	return nil
}

type createPRInput struct {
	prSubject     string
	prRepoOwner   string
	prDescription string
	branchFrom    string
	branchTo      string
	repo          string
}

// createPR creates a pull request. Based on: https://godoc.org/github.com/google/go-github/github#example-PullRequestsService-Create
func (prService DefaultPullRequestService) createPR(ctx context.Context, input createPRInput) (url string, err error) {
	if input.prSubject == "" {
		return "", errors.New("PR subject is missing")
	}

	newPR := &github.NewPullRequest{
		Title:               &input.prSubject,
		Head:                &input.branchFrom,
		Base:                &input.branchTo,
		Body:                &input.prDescription,
		MaintainerCanModify: github.Bool(true),
	}

	pr, _, err := prService.client.PullRequests.Create(ctx, input.prRepoOwner, input.repo, newPR)
	if err != nil {
		return "", err
	}

	return pr.GetHTMLURL(), nil
}
