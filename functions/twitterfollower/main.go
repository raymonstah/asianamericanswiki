package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

// this should be relative to the main project directory.
var humansDirectory = filepath.Join("content", "humans")

func run() error {
	existingHandles, err := getExistingFollowingHandles()
	if err != nil {
		return fmt.Errorf("unable to get existing twitter folowings: %w", err)
	}
	updatedHandles, err := getTwitterHandlesFromDir(humansDirectory)
	if err != nil {
		return fmt.Errorf("unable to get existing twitter handles: %w", err)
	}
	toFollow, toUnfollow := handleDiffs(existingHandles, updatedHandles)
	if err := followHandles(toFollow); err != nil {
		return fmt.Errorf("unable to follow: %w", err)
	}

	if err := unfollowHandles(toUnfollow); err != nil {
		return fmt.Errorf("unable to unfollow: %w", err)
	}

	return nil
}

func unfollowHandles(toUnfollow []string) error {
	return nil
}

func followHandles(toFollow []string) error {
	return nil
}

func handleDiffs(existingHandles, updatedHandles []string) (toFollow, toUnfollow []string) {
	toFollow = setDiff(updatedHandles, existingHandles)
	toUnfollow = setDiff(existingHandles, updatedHandles)
	return
}

func getExistingFollowingHandles() ([]string, error) {
	return nil, nil
}

func getTwitterHandlesFromDir(dir string) ([]string, error) {
	wd,err := os.Getwd()
	if err != nil {
		return nil, err
	}
	projectDirectory := filepath.Dir(filepath.Dir(wd))
	fmt.Println("parent:", projectDirectory)
	return nil, nil
}

// setDiff computes a - b
func setDiff(a, b []string) (diff []string) {
	m := make(map[string]struct{})
	for _, item := range b {
		m[item] = struct{}{}
	}

	for _, item := range a {
		if _, ok := m[item]; !ok {
			diff = append(diff, item)
		}
	}

	return diff
}
