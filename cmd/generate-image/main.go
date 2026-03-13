package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/raymonstah/asianamericanswiki/internal/imageutil"
	"github.com/raymonstah/asianamericanswiki/internal/xai"
	"github.com/urfave/cli/v2"
)

func main() {
	var opts struct {
		HumanID   string
		UseProd   bool
		XAIToken  string
		SourceURL string
	}

	app := &cli.App{
		Name:  "generate-image",
		Usage: "Generate an image for a human using xAI and upload it",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "id", Required: true, Destination: &opts.HumanID, Usage: "The ID of the human"},
			&cli.BoolFlag{Name: "use-prod", Value: false, Destination: &opts.UseProd},
			&cli.StringFlag{Name: "xai-api-key", EnvVars: []string{"XAI_API_KEY"}, Destination: &opts.XAIToken, Required: true},
			&cli.StringFlag{Name: "source-url", Destination: &opts.SourceURL, Usage: "URL of a source image to base the generation on", Required: true},
		},
		Action: func(c *cli.Context) error {
			ctx := c.Context

			if !opts.UseProd {
				if err := os.Setenv("FIRESTORE_EMULATOR_HOST", "127.0.0.1:8080"); err != nil {
					return err
				}
				if err := os.Setenv("STORAGE_EMULATOR_HOST", "127.0.0.1:9199"); err != nil {
					return err
				}
			} else {
				if err := os.Unsetenv("FIRESTORE_EMULATOR_HOST"); err != nil {
					return err
				}
				if err := os.Unsetenv("STORAGE_EMULATOR_HOST"); err != nil {
					return err
				}
			}

			xClient := xai.New(opts.XAIToken)

			fsClient, err := firestore.NewClient(ctx, api.ProjectID)
			if err != nil {
				return fmt.Errorf("unable to create firestore client: %w", err)
			}
			dao := humandao.NewDAO(fsClient)

			storageClient, err := storage.NewClient(ctx)
			if err != nil {
				return fmt.Errorf("unable to create storage client: %w", err)
			}

			storageURL := "https://storage.googleapis.com"
			if !opts.UseProd {
				storageURL = "http://127.0.0.1:9199"
			}
			uploader := imageutil.NewUploader(storageClient, dao, storageURL)

			human, err := dao.Human(ctx, humandao.HumanInput{HumanID: opts.HumanID})
			if err != nil {
				return fmt.Errorf("unable to fetch human %q: %w", opts.HumanID, err)
			}

			log.Printf("Generating image for %s (%s)...", human.Name, human.ID)
			prompt := xai.DefaultImagePrompt(human.Name)
			log.Printf("Using prompt: %q", prompt)

			log.Printf("Downloading source image from %s", opts.SourceURL)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, opts.SourceURL, nil)
			if err != nil {
				return fmt.Errorf("unable to create request for source image: %w", err)
			}
			// Set a user agent to avoid being blocked by simple scrapers
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("unable to download source image: %w", err)
			}
			defer func() {
				_ = resp.Body.Close()
			}()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("unexpected status code downloading source image: %d", resp.StatusCode)
			}
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read source image: %w", err)
			}

			base64Data := base64.StdEncoding.EncodeToString(data)
			mimeType := http.DetectContentType(data)
			baseImage := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)
			log.Printf("Successfully prepared source image base64 (type: %s)", mimeType)

			imageURLs, err := xClient.GenerateImage(ctx, xai.GenerateImageInput{
				Prompt: prompt,
				N:      1,
				Image:  baseImage,
			})
			if err != nil {
				return fmt.Errorf("unable to generate image: %w", err)
			}
			if len(imageURLs) == 0 {
				return fmt.Errorf("no image URLs returned from xAI")
			}

			imageURL := imageURLs[0]
			log.Printf("Image generated at: %s", imageURL)

			log.Println("Downloading image...")
			resp, err = http.Get(imageURL)
			if err != nil {
				return fmt.Errorf("unable to download image: %w", err)
			}
			defer func() {
				_ = resp.Body.Close()
			}()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("unexpected status code downloading image: %d", resp.StatusCode)
			}

			rawImage, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("unable to read downloaded image: %w", err)
			}

			log.Println("Uploading image to storage...")
			human, err = uploader.UploadHumanImages(ctx, human, rawImage)
			if err != nil {
				return fmt.Errorf("unable to upload image: %w", err)
			}

			log.Printf("Successfully updated human %s with new image.", human.Name)
			log.Printf("Featured Image: %s", human.Images.Featured)
			
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
