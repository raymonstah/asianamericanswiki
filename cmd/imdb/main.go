package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/gocolly/colly/v2"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/cartoonize"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/urfave/cli/v2"
)

var bucketName = "asianamericanswiki-images"
var opts struct {
	Name  string
	Debug bool
}

func main() {
	app := &cli.App{
		Name: "A CLI tool to get an image from imdb for a human.",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Destination: &opts.Name},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type Handler struct {
	storageClient *storage.Client
	fsClient      *firestore.Client
	humanDAO      *humandao.DAO
	jobs          chan Job
}

type Job struct {
	Name string
	Link string
	IMDB string
}

func run(c *cli.Context) error {
	ctx := c.Context
	fsClient, err := firestore.NewClient(ctx, api.ProjectID)
	if err != nil {
		return fmt.Errorf("unable to create firestore client: %w", err)
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("unable to create storage client: %w", err)
	}

	humanDAO := humandao.NewDAO(fsClient)
	h := Handler{
		fsClient:      fsClient,
		storageClient: client,
		humanDAO:      humanDAO,
		jobs:          make(chan Job, 100),
	}

	if err := h.Do(ctx); err != nil {
		return err
	}

	return nil
}

func (h *Handler) Do(ctx context.Context) error {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.processJobs(ctx)
	}()
	if opts.Name == "" {
		log.Println("no name provided. scanning all humans..")
		humans, err := h.humanDAO.ListHumans(ctx, humandao.ListHumansInput{
			Limit: 20,
		})
		if err != nil {
			return err
		}
		for _, human := range humans {
			if !slices.Contains(human.Tags, "actor") && !slices.Contains(human.Tags, "actress") {
				continue
			}

			if err := h.search(ctx, human.Name); err != nil {
				return fmt.Errorf("unable to search imdb for %v: %w", human.Name, err)
			}
		}
	} else {
		if err := h.search(ctx, opts.Name); err != nil {
			return fmt.Errorf("unable to search imdb for %v: %w", opts.Name, err)
		}
	}

	close(h.jobs)
	wg.Wait()
	return nil
}

func (h *Handler) search(ctx context.Context, name string) error {
	// url encode name
	log.Println("Searching imdb for", name)
	urlEncodedName := url.QueryEscape(name)
	url := fmt.Sprintf("https://www.imdb.com/find/?q=%v&s=nm&exact=true", urlEncodedName)
	log.Println("Visiting", url)

	col := colly.NewCollector()
	col.OnRequest(func(r *colly.Request) {
		r.Headers.Set("User-Agent", "1 Mozilla/5.0 (iPad; CPU OS 12_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148")
	})
	col.SetRequestTimeout(30 * time.Second)
	found := false
	col.OnHTML(".ipc-metadata-list-summary-item__t", func(e *colly.HTMLElement) {
		if found {
			return
		}
		link := e.Attr("href")
		if name == e.Text {
			found = true
			log.Printf("Link found: %q -> %s\n", e.Text, link)
			if err := h.downloadImage("https://www.imdb.com"+link, name); err != nil {
				log.Println("unable to download image:", err)
			}
		}
	})
	if err := col.Visit(url); err != nil {
		return fmt.Errorf("unable to visit url: %w", err)
	}

	return nil
}

func (h *Handler) downloadImage(url, name string) error {
	col := colly.NewCollector()
	col.OnRequest(func(r *colly.Request) {
		r.Headers.Set("User-Agent", "1 Mozilla/5.0 (iPad; CPU OS 12_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148")
	})
	col.SetRequestTimeout(30 * time.Second)

	firstImageFound := false
	col.OnHTML(".ipc-image", func(e *colly.HTMLElement) {
		if firstImageFound {
			return
		}
		if e.Attr("alt") == name {
			log.Println("Found image for", name)
			firstImageFound = true
			imageLink := e.Attr("src")
			h.jobs <- Job{
				Name: name,
				Link: imageLink,
				IMDB: url,
			}

		}
	})

	if err := col.Visit(url); err != nil {
		return fmt.Errorf("unable to visit url: %w", err)
	}

	return nil
}

func (h *Handler) processJobs(ctx context.Context) {
	defer log.Println("done processing jobs")
	for job := range h.jobs {
		if err := h.processJob(ctx, job); err != nil {
			log.Fatal("unable to process job:", err)
		}
	}
}

func (h *Handler) processJob(ctx context.Context, job Job) error {
	log.Println("Processing job for", job.Name)
	defer log.Println("Processed job for", job.Name)
	url, name := job.Link, job.Name
	path := strings.ToLower(name)
	path = strings.ReplaceAll(path, " ", "-")
	human, err := h.humanDAO.Human(ctx, humandao.HumanInput{Path: path})
	if err != nil {
		return fmt.Errorf("unable to get human: %w", err)
	}

	if human.FeaturedImage == "" {
		log.Printf("Found image for %v: %v", name, url)
		fullSizeURL := modifyURL(url)
		log.Printf("New url: %v", fullSizeURL)

		resp, err := http.Get(fullSizeURL)
		if err != nil {
			log.Println("unable to download image:", err)
			return err
		}

		tempPath, err := writeResponseToTempFile(resp.Body)
		if err != nil {
			return err
		}

		resp.Body.Close()

		id := human.ID
		cartoonizeClient := cartoonize.Client{
			Debug: opts.Debug,
		}
		raw, err := cartoonizeClient.Do(tempPath)
		if err != nil {
			return fmt.Errorf("unable to cartoonize image: %w", err)
		}

		imgName := fmt.Sprintf("%v.jpg", id)
		obj := h.storageClient.Bucket(bucketName).Object(imgName)
		writer := obj.NewWriter(ctx)
		if _, err := writer.Write(raw); err != nil {
			return err
		}

		if err := writer.Close(); err != nil {
			return err
		}

		featuredImageURL := fmt.Sprintf("https://storage.googleapis.com/%v/%v", bucketName, imgName)
		human.FeaturedImage = featuredImageURL
	} else {
		log.Printf("human %v already has a featured image\n", human.Name)
	}

	human.Socials.IMDB = job.IMDB
	if err := h.humanDAO.UpdateHuman(ctx, human); err != nil {
		return fmt.Errorf("unable to update human: %w", err)
	}

	return nil
}
func modifyURL(inputURL string) string {
	// Find the position of the last period ('.')
	lastDotIndex := strings.LastIndex(inputURL, ".")

	// Check if a period was found
	if lastDotIndex != -1 {
		// Find the position of the second-to-last period ('.') before the last one
		secondToLastDotIndex := strings.LastIndex(inputURL[:lastDotIndex], ".")

		// Check if a second-to-last period was found
		if secondToLastDotIndex != -1 {
			// Create the new modified URL by replacing everything after the second-to-last period with "_V1_FMjpg_UX1000_.jpg"
			modifiedURL := inputURL[:secondToLastDotIndex] + "._V1_FMjpg_UX1000_" + inputURL[lastDotIndex:]
			return modifiedURL
		}
	}

	// Return the inputURL as is if no suitable periods were found
	return inputURL
}

func writeResponseToTempFile(body io.ReadCloser) (string, error) {
	tempFile, err := os.CreateTemp("", "response_")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	_, err = io.Copy(tempFile, body)
	if err != nil {
		return "", err
	}

	tempFilePath := tempFile.Name()

	return tempFilePath, nil
}
