// Copyright 2018 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The commitpr command utilizes go-github as a CLI tool for
// pushing files to a branch and creating a pull request from it.
// It takes an auth token as an environment variable and creates
// the commit and the PR under the account affiliated with that token.
//
// The purpose of this example is to show how to use refs, trees and commits to
// create commits and pull requests.
//
// Note, if you want to push a single file, you probably prefer to use the
// content API. An example is available here:
// https://godoc.org/github.com/google/go-github/github#example-RepositoriesService-CreateFile
//
// Note, for this to work at least 1 commit is needed, so you if you use this
// after creating a repository you might want to make sure you set `AutoInit` to
// `true`.
package main

import (
	"context"
	"log"
	"os"
)

func main() {
	token := os.Getenv("GITHUB_AUTH_TOKEN")
	if token == "" {
		log.Fatal("Unauthorized: No token present")
	}

	ctx := context.Background()
	prService := NewPullRequestService(ctx, token)

	if err := prService.createFile(ctx, createFileInput{
		owner:         "",
		repository:    "",
		path:          "",
		content:       []byte(""),
		commitMessage: "",
		branch:        "",
		authorName:    "",
		authorEmail:   "",
	}); err != nil {
		log.Fatalf("Unable to create file: %s\n", err)
	}

	if err := prService.createPR(ctx, createPRInput{
		prSubject:     "",
		prRepoOwner:   "",
		prDescription: "",
		branchFrom:    "",
		branchTo:      "",
		repo:          "",
	}); err != nil {
		log.Fatalf("Unable to create pull request: %s", err)
	}
}
