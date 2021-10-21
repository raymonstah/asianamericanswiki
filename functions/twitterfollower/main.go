package main

import (
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
)

// this should be relative to the main project directory.
var humansDirectory = filepath.Join("content", "humans")

func main() {
	flags := flag.NewFlagSet("user-auth", flag.ExitOnError)
	consumerKey := flags.String("consumer-key", os.Getenv("TWITTER_CONSUMER_KEY"), "Twitter Consumer Key")
	consumerSecret := flags.String("consumer-secret", os.Getenv("TWITTER_CONSUMER_SECRET"), "Twitter Consumer Secret")
	accessToken := flags.String("access-token", os.Getenv("TWITTER_ACCESS_TOKEN"), "Twitter Access Token")
	accessSecret := flags.String("access-secret", os.Getenv("TWITTER_ACCESS_SECRET"), "Twitter Access Secret")
	if err := flags.Parse(os.Args[1:]); err != nil {
		log.Fatalf("error parsing flags: %v", err)
	}
	if *consumerKey == "" || *consumerSecret == "" || *accessToken == "" || *accessSecret == "" {
		log.Fatal("Consumer key/secret and Access token/secret required")
	}
	app := setupApp(*consumerKey, *consumerSecret, *accessToken, *accessSecret)
	if err := app.run(); err != nil {
		panic(err)
	}

}

func setupApp(consumerKey, consumerSecret, accessToken, accessSecret string) app {
	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessSecret)
	httpClient := config.Client(oauth1.NoContext, token)

	client := twitter.NewClient(httpClient)

	return app{
		client: client,
	}
}

type app struct {
	client *twitter.Client
}

func (app app) run() error {
	existingHandles, err := app.getExistingFollowingHandles()
	if err != nil {
		return fmt.Errorf("unable to get existing twitter followings: %w", err)
	}
	updatedHandles, err := app.getTwitterHandlesFromDir(humansDirectory)
	if err != nil {
		return fmt.Errorf("unable to get existing twitter handles: %w", err)
	}
	toFollow, toUnfollow := handleDiffs(existingHandles, updatedHandles)
	if err := app.followHandles(toFollow); err != nil {
		return fmt.Errorf("unable to follow: %w", err)
	}

	if err := app.unfollowHandles(toUnfollow); err != nil {
		return fmt.Errorf("unable to unfollow: %w", err)
	}

	return nil
}

func (app app) unfollowHandles(toUnfollow []string) error {
	for _, toUnfollow := range toUnfollow {
		fmt.Println("attempting to unfollow", toUnfollow)
		_, resp, err := app.client.Friendships.Destroy(&twitter.FriendshipDestroyParams{
			ScreenName: toUnfollow,
		})
		if err != nil {
			return err
		}
		if resp.StatusCode > 300 {
			body, _ := ioutil.ReadAll(resp.Body)
			bodyString := string(body)
			fmt.Println(resp.StatusCode, bodyString, resp.Header)
		}
	}

	return nil
}

func (app app) followHandles(toFollows []string) error {
	for _, toFollow := range toFollows {
		fmt.Println("attempting to follow", toFollow)
		_, resp, err := app.client.Friendships.Create(&twitter.FriendshipCreateParams{
			ScreenName: toFollow,
		})
		if err != nil {
			if !strings.Contains(err.Error(), "twitter: 160 You've already requested to follow") &&
				!strings.Contains(err.Error(), "twitter: 108 Cannot find specified user.") {
				return err
			}
		}
		if resp.StatusCode > 300 {
			err := fmt.Errorf("received %v when attempting to follow %v", resp.StatusCode, toFollow)
			fmt.Println(err)
		}
	}

	return nil
}

func handleDiffs(existingHandles, updatedHandles []string) (toFollow, toUnfollow []string) {
	toFollow = setDiff(updatedHandles, existingHandles)
	toUnfollow = setDiff(existingHandles, updatedHandles)
	return
}

func (app app) getExistingFollowingHandles() ([]string, error) {
	var existingFriends []string
	var cursor int64 = -1
	for {
		friends, _, err := app.client.Friends.List(&twitter.FriendListParams{
			ScreenName: "aapiwiki",
			Cursor:     cursor,
			Count:      200,
			SkipStatus: twitter.Bool(true),
		})
		if err != nil {
			return nil, err
		}

		cursor = friends.NextCursor
		for _, friend := range friends.Users {
			existingFriends = append(existingFriends, friend.ScreenName)
		}
		if cursor <= 0 {
			break
		}
	}

	return existingFriends, nil
}

var frontMatter = regexp.MustCompile(`twitter: (\S*)`)

func (app app) getTwitterHandlesFromDir(dir string) ([]string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	projectDirectory := filepath.Dir(filepath.Dir(wd))
	humansDir := filepath.Join(projectDirectory, dir)

	var usernames []string
	err = filepath.WalkDir(humansDir, func(path string, d fs.DirEntry, err error) error {
		if d.Name() != "index.md" {
			return nil
		}
		rawContent, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("unable to read file: %w", err)
		}
		subgroup := frontMatter.FindStringSubmatch(string(rawContent))
		if len(subgroup) < 2 {
			return nil
		}
		twitterUsername := strings.ReplaceAll(subgroup[1], `"`, "")
		twitterUsername = strings.TrimPrefix(twitterUsername, "https://twitter.com/")
		if twitterUsername == "" {
			return nil
		}
		usernames = append(usernames, twitterUsername)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("unable to walk humans directory: %v: %w", humansDir, err)
	}
	return usernames, nil
}

// setDiff computes a - b, ignoring case.
func setDiff(a, b []string) (diff []string) {
	m := make(map[string]struct{})
	for _, item := range b {
		item = strings.ToLower(item)
		m[item] = struct{}{}
	}

	for _, item := range a {
		item := strings.ToLower(item)
		if _, ok := m[item]; !ok {
			diff = append(diff, item)
		}
	}

	return diff
}
