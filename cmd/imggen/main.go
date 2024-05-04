package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/urfave/cli/v2"
)

var opts struct {
	Image string
	Name  string
	Webp  bool
	Dry   bool
}

func main() {
	app := &cli.App{
		Name: "A CLI tool to upload an image.",
		Flags: []cli.Flag{
			&cli.PathFlag{Name: "image", Destination: &opts.Image},
			&cli.StringFlag{Name: "name", Required: true, Destination: &opts.Name},
			&cli.BoolFlag{Name: "webp", Destination: &opts.Webp},
			&cli.BoolFlag{Name: "dry", Destination: &opts.Dry},
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
	pathToImage := opts.Image

	if opts.Webp {
		tempDir, err := os.MkdirTemp(os.TempDir(), "webp")
		if err != nil {
			return fmt.Errorf("unable to create temp dir: %w", err)
		}

		// no image provided, use the image from Cloud Storage
		if pathToImage == "" {
			cloudStoragePath := filepath.Base(human.FeaturedImage)
			fmt.Println("cloudStoragePath:", cloudStoragePath)
			object := h.storageClient.Bucket(api.ImagesStorageBucket).Object(cloudStoragePath)
			reader, err := object.NewReader(ctx)
			if err != nil {
				return fmt.Errorf("unable to read image from cloud storage path %v: %w", cloudStoragePath, err)
			}
			parts := strings.Split(cloudStoragePath, ".")
			extension := ""
			if len(parts) == 1 {
				attrs, err := object.Attrs(ctx)
				if err != nil {
					return fmt.Errorf("unable to get image attrs: %w", err)
				}
				if strings.Contains(attrs.ContentType, "jpeg") {
					extension = ".jpeg"
				} else if strings.Contains(attrs.ContentType, "png") {
					extension = ".png"
				} else {
					return fmt.Errorf("unsupported file type: %v", attrs.ContentType)
				}
			}
			pathToImage = filepath.Join(tempDir, cloudStoragePath+extension)
			dest, err := os.Create(pathToImage)
			if err != nil {
				return fmt.Errorf("unable to write image from cloud storage: %w", err)
			}
			fmt.Println("wrote image from cloud storage to temp dir", pathToImage)
			if _, err := io.Copy(dest, reader); err != nil {
				return fmt.Errorf("unable to copy image from cloud storage: %w", err)
			}
			defer func() {
				if len(parts) == 1 {
					fmt.Println("overwrite happened -- skipping delete")
					return
				}
				fmt.Printf("deleting old image %v", cloudStoragePath)
				if err := object.Delete(ctx); err != nil {
					log.Default().Panic("error deleting image", err)
				}
			}()

		}

		fileName := filepath.Base(pathToImage)
		fileNameParts := strings.Split(fileName, ".")
		fileNameWithoutExtension := fileNameParts[0]
		webpImage := fileNameWithoutExtension + ".webp"
		sourceImagePath := filepath.Join(tempDir, fileName)
		pathToWebp := filepath.Join(tempDir, webpImage)
		args := []string{"-path", tempDir, "-format", "webp", "-quality", "10", sourceImagePath}
		cmd := exec.Command("mogrify", args...)
		if err := cmd.Run(); err != nil {
			fmt.Printf("mogrify %v\n", strings.Join(args, " "))
			return fmt.Errorf("unable to convert image to webp: %w", err)
		}
		fmt.Printf("wrote webp image to %v\n", pathToWebp)
		pathToImage = pathToWebp
	}

	raw, err := os.ReadFile(pathToImage)
	if err != nil {
		return err
	}

	if opts.Dry {
		fmt.Println("dry mode detected -- exiting.")
		return nil
	}

	imgName := fmt.Sprintf("%v", id)
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
	if err := h.humanDAO.UpdateHuman(ctx, human); err != nil {
		return fmt.Errorf("unable to update human: %w", err)
	}

	log.Println("done: ", featuredImageURL)
	return nil
}
