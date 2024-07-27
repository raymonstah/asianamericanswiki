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
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"
)

var opts struct {
	Name  string
	Debug bool
	Force bool
	URL   string
	// ID takes precedence over name
	ID string
}

func main() {
	app := &cli.App{
		Name: "A CLI tool to get an image from imdb for a human.",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Destination: &opts.Name},
			&cli.StringFlag{Name: "id", Destination: &opts.ID},
			&cli.StringFlag{Name: "url", Destination: &opts.URL},
			&cli.BoolFlag{Name: "force", Destination: &opts.Force},
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
			Limit: 500,
		})
		if err != nil {
			return err
		}
		group, ctx := errgroup.WithContext(ctx)
		group.SetLimit(8)
		for _, human := range humans {
			human := human
			if !slices.Contains(human.Tags, "actor") && !slices.Contains(human.Tags, "actress") {
				continue
			}
			group.Go(func() error {
				if err := h.search(ctx, human.Name); err != nil {
					return fmt.Errorf("unable to search imdb for %v: %w", human.Name, err)
				}

				return nil
			})
			if err := group.Wait(); err != nil {
				return err
			}
		}
	} else {
		if opts.URL != "" {
			if err := h.visitURL(opts.URL, opts.Name); err != nil {
				return fmt.Errorf("unable to download image: %w", err)
			}
		}
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
			log.Printf("Link found: %q -> %s\n", e.Text, "https://www.imdb.com"+link)
			if err := h.visitURL("https://www.imdb.com"+link, name); err != nil {
				log.Println("unable to download image:", err)
			}
		}
	})
	if err := col.Visit(url); err != nil {
		return fmt.Errorf("unable to visit url: %w", err)
	}

	return nil
}

func (h *Handler) visitURL(url, name string) error {
	col := colly.NewCollector()
	col.OnRequest(func(r *colly.Request) {
		r.Headers.Set("User-Agent", "1 Mozilla/5.0 (iPad; CPU OS 12_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148")
	})
	col.SetRequestTimeout(30 * time.Second)

	firstImageFound := false
	col.OnHTML("img.ipc-image", func(e *colly.HTMLElement) {
		if firstImageFound {
			return
		}
		if strings.Contains(e.Attr("alt"), name) {
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

	col.OnScraped(func(r *colly.Response) {
		if !firstImageFound {
			log.Println("No image found for", name)
			log.Printf("Creating job for %v IMDB link without image\n", name)
			h.jobs <- Job{
				Name: name,
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
	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(4)
	for job := range h.jobs {
		job := job
		group.Go(func() error {
			return h.processJob(ctx, job)
		})
	}

	if err := group.Wait(); err != nil {
		log.Println("error processing jobs:", err)
	}
}

func (h *Handler) processJob(ctx context.Context, job Job) error {
	log.Println("Processing job for", job.Name)
	defer log.Println("Processed job for", job.Name)
	url, name := job.Link, job.Name
	path := strings.ToLower(name)
	path = strings.ReplaceAll(path, " ", "-")
	input := humandao.HumanInput{Path: path}
	if opts.ID != "" {
		input.HumanID = opts.ID
	}
	human, err := h.humanDAO.Human(ctx, input)
	if err != nil {
		return fmt.Errorf("unable to get human: %w", err)
	}

	if url != "" && (human.FeaturedImage == "" || opts.Force) {
		fullSizeURL := modifyURL(url)
		log.Printf("Found image for %v: %v", name, fullSizeURL)

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
		raw, err := os.ReadFile(tempPath)
		if err != nil {
			return err
		}

		imgName := fmt.Sprintf("%v.jpg", id)
		obj := h.storageClient.Bucket(api.ImagesStorageBucket).Object(imgName)
		writer := obj.NewWriter(ctx)
		if _, err := writer.Write(raw); err != nil {
			return err
		}

		if err := writer.Close(); err != nil {
			return err
		}

		featuredImageURL := fmt.Sprintf("https://storage.googleapis.com/%v/%v", api.ImagesStorageBucket, imgName)
		human.FeaturedImage = featuredImageURL
		log.Printf("Setting image for %v to %v\n", human.Name, featuredImageURL)
	} else {
		log.Printf("%v already has an image: %v\n", human.Name, human.FeaturedImage)
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
