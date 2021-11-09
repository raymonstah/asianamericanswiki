package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/go-github/v40/github"
	"golang.org/x/oauth2"
)

type PullRequestService struct {
	client *github.Client
}

func NewPullRequestService(ctx context.Context, token string) PullRequestService {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	return PullRequestService{client: client}
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

func (prService PullRequestService) createFile(ctx context.Context, input createFileInput) error {
	// Note: the file needs to be absent from the repository as you are not
	// specifying a SHA reference here.
	opts := &github.RepositoryContentFileOptions{
		Message:   &input.commitMessage,
		Content:   input.content,
		Branch:    &input.branch,
		Committer: &github.CommitAuthor{Name: &input.authorName, Email: &input.authorEmail},
	}
	_, _, err := prService.client.Repositories.CreateFile(ctx, input.owner, input.repository, input.path, opts)
	return err
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
func (prService PullRequestService) createPR(ctx context.Context, input createPRInput) (err error) {
	if input.prSubject == "" {
		return errors.New("PR subject is missing")
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
		return err
	}

	fmt.Printf("PR created: %s\n", pr.GetHTMLURL())
	return nil
}
