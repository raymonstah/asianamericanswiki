package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"regexp"
	"strings"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/urfave/cli/v2"
)

var url = "https://cartoonize-lkqov62dia-de.a.run.app"
var bucketName = "asianamericanswiki-images"
var opts struct {
	Image string
	Name  string
	Debug bool
}

func main() {
	app := &cli.App{
		Name: "A CLI tool to cartoonize an image.",
		Flags: []cli.Flag{
			&cli.PathFlag{Name: "image", Required: true, Destination: &opts.Image},
			&cli.StringFlag{Name: "name", Required: true, Destination: &opts.Name},
			&cli.BoolFlag{Name: "debug", Destination: &opts.Debug},
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
	}

	if err := h.Do(ctx); err != nil {
		return err
	}

	return nil
}

func (h *Handler) Do(ctx context.Context) error {
	path := strings.ToLower(opts.Name)
	path = strings.ReplaceAll(path, " ", "-")
	human, err := h.humanDAO.Human(ctx, humandao.HumanInput{Path: path})
	if err != nil {
		return fmt.Errorf("unable to get human: %w", err)
	}

	id := human.ID

	rawHTML, err := submitImage(url+"/cartoonize", opts.Image)
	if err != nil {
		return err
	}

	imgPath, err := extractDownloadHref(string(rawHTML))
	if err != nil {
		return err
	}

	fullDownloadPath := fmt.Sprintf("%v/%v", url, imgPath)
	raw, err := downloadImage(fullDownloadPath)
	if err != nil {
		return err
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
	if err := h.humanDAO.UpdateHuman(ctx, human); err != nil {
		return fmt.Errorf("unable to update human: %w", err)
	}

	log.Println("done.")
	return nil
}

func downloadImage(path string) ([]byte, error) {
	log.Printf("downloading image from %v\n", path)

	// Create a new GET request to the URL
	request, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	// Send the GET request
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// Check the response status code
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func submitImage(url string, imagePath string) ([]byte, error) {
	log.Printf("submitting image from %v to cartoonize\n", imagePath)
	// Create a new POST request to the URL
	requestBody := &bytes.Buffer{}
	writer := multipart.NewWriter(requestBody)

	// Open the image file
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Create a form field for the image
	part, err := writer.CreateFormFile("image", imagePath)
	if err != nil {
		return nil, err
	}

	// Copy the image file into the form field
	_, err = io.Copy(part, file)
	if err != nil {
		return nil, err
	}

	// Close the multipart writer
	writer.Close()

	// Create a POST request with the form data
	request, err := http.NewRequest("POST", url, requestBody)
	if err != nil {
		return nil, err
	}

	// Set the Content-Type header for the request
	request.Header.Set("Content-Type", writer.FormDataContentType())

	// Send the POST request
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// Check the response status code
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func extractDownloadHref(htmlContent string) (downloadHref string, err error) {
	log.Printf("extracting download href\n")
	if opts.Debug {
		fmt.Println(htmlContent)
		fmt.Println()
		fmt.Println()
	}

	re := regexp.MustCompile(`static/cartoonized_images/[^"']+`)

	// Find all matches
	matches := re.FindAllString(htmlContent, -1)

	if len(matches) == 0 {
		return "", fmt.Errorf("no matching href found")
	}

	return matches[0], nil
}
