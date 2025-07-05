package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	_ "image/jpeg"
	"io"
	"io/fs"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/webp"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	vision "cloud.google.com/go/vision/apiv1"
	"cloud.google.com/go/vision/v2/apiv1/visionpb"
	"github.com/davidbyttow/govips/v2/vips"
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
		Name: "A CLI tool for all image processing tasks.",
		Commands: []*cli.Command{
			{
				Name:  "upload",
				Usage: "upload a single image for a human",
				Flags: []cli.Flag{
					&cli.PathFlag{Name: "image", Destination: &opts.Image},
					&cli.StringFlag{Name: "name", Required: true, Destination: &opts.Name},
					&cli.BoolFlag{Name: "webp", Destination: &opts.Webp},
				},
				Action: run,
			},
			{
				Name:  "migrate-thumbnails",
				Usage: "one time tool to generate thumbnails for existing images",
				Flags: []cli.Flag{
					&cli.PathFlag{Name: "dir", Required: true, Usage: "input directory of images to generate thumbnails for"},
					&cli.PathFlag{Name: "cache", Usage: "a cache file of detected faces", Value: ".detected-faces.json"},
				},
				Action: migrateThumbnails,
			},
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "dry", Destination: &opts.Dry},
		},
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

// migrateThumbnails takes all images in a local directory and generates thumbnails for them using libvips.
// It writes to a new folder for inspection.
// If dry mode is enabled, it does not push the newly generated thumbnails upstream.
func migrateThumbnails(c *cli.Context) error {
	vips.LoggingSettings(nil, vips.LogLevelError)
	vips.Startup(nil)
	defer vips.Shutdown()
	ctx := c.Context
	if !opts.Dry {
		storageClient, err := storage.NewClient(ctx)
		if err != nil {
			return fmt.Errorf("unable to create storage client: %w", err)
		}
		bucket := storageClient.Bucket(api.ImagesStorageBucket)
		_ = bucket // todo: leave bucket unused for now, until we need to write to the bucket
	}

	dir := c.String("dir")
	log.Printf("migrating thumbnails dry=%v dir=%s", opts.Dry, dir)
	tempDir, err := os.MkdirTemp("", "thumbnails")
	if err != nil {
		return fmt.Errorf("unable to create directory to hold edited thumbnails: %w", err)
	}

	outputDir := dir
	if opts.Dry {
		outputDir = tempDir
	}
	log.Printf("output directory %s", outputDir)

	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return fmt.Errorf("unable to create new directory %v: %w", outputDir, err)
	}

	var imagePaths []string
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// skip hidden files
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		imagePaths = append(imagePaths, path)
		return nil
	})
	if err != nil {
		return fmt.Errorf("error walking directory: %w", err)
	}

	cacheFilePath := c.Path("cache")
	var cachedDetectedFaces CachedDetectedFaces
	raw, err := os.ReadFile(cacheFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("unable to open cache file: %w", err)
		}
		cachedDetectedFaces = make(CachedDetectedFaces, len(imagePaths))
	} else {
		if err := json.Unmarshal(raw, &cachedDetectedFaces); err != nil {
			return fmt.Errorf("unable to unmarshal to cachedDetectedFaces: %w", err)
		}
	}

	images := make([][]byte, 0, len(imagePaths))
	for _, path := range imagePaths {
		log.Printf("processing image %v", path)
		imgBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("unable to read file: %w", err)
		}
		images = append(images, imgBytes)

	}

	log.Printf("found %v images", len(images))

	// validate that the images are in the cache
	var missingInCache [][]byte
	for i, ip := range imagePaths {
		if _, ok := cachedDetectedFaces[ip]; !ok {
			log.Printf("%v missing from cache", ip)
			missingInCache = append(missingInCache, images[i])
		}

	}

	// todo: tweak this so that we only detect the missing faces
	if len(missingInCache) > 0 {
		log.Printf("%v images missing from cache", len(missingInCache))
		log.Printf("detecting faces..")
		detectedFaces, err := detectFaceGoogleVision(ctx, DetectFaceInput{
			Images: images,
		})
		if err != nil {
			return fmt.Errorf("unable to detect faces: %w", err)
		}
		if len(detectedFaces.Faces) != len(missingInCache) {
			return fmt.Errorf("length mismatch, got %v expected %v", len(detectedFaces.Faces), len(imagePaths))
		}
		for i, face := range detectedFaces.Faces {
			cachedDetectedFaces[imagePaths[i]] = face
		}
		raw, err := json.Marshal(cachedDetectedFaces)
		if err != nil {
			return fmt.Errorf("unable to marshal detected faces: %w", err)
		}
		if err := os.WriteFile(cacheFilePath, raw, 0644); err != nil {
			return fmt.Errorf("unable to write cached file: %w", err)
		}
		log.Printf("wrote cache file %v", cacheFilePath)
	} else {
		log.Printf("using %v face detections from cache", len(cachedDetectedFaces))
	}

	for _, path := range imagePaths {
		face, ok := cachedDetectedFaces[path]
		if !ok {
			face.NoFace = true
		}

		outputDir := outputDir
		baseName := filepath.Base(path)
		thumbNailFile := "thumbnail.webp"
		highlightFile := "highlighted.webp"
		if opts.Dry {
			thumbNailFile = fmt.Sprintf("%v-%v", stripExtension(baseName), thumbNailFile)
			highlightFile = fmt.Sprintf("%v-%v", stripExtension(baseName), highlightFile)
		} else {
			outputDir = filepath.Dir(path)
		}

		newThumbnailPath := filepath.Join(outputDir, thumbNailFile)
		newHighlightPath := filepath.Join(outputDir, highlightFile)

		if opts.Dry {
			// highlight is useful for debugging, since we can see the red box on the original image.
			if err := highlight(path, face, newHighlightPath); err != nil {
				return fmt.Errorf("unable to highlight image: %w", err)
			}
		}
		if err := createThumbnail(path, newThumbnailPath, face); err != nil {
			return fmt.Errorf("unable to create thumbnail: %w", err)
		}
	}
	log.Println("done.")
	log.Printf("open %s", outputDir)
	return nil
}

func expandBoundingBox(face BoundingBox, imgWidth, imgHeight int) BoundingBox {
	if face.NoFace {
		return face
	}
	// Desired thumbnail size (square)
	const thumbSize = 256

	// The face vertical position inside the crop: 1/3 from the top
	const faceTopRatio = 1.0 / 3.0

	// Compute the center of the face box
	faceCenterX := float64(face.X) + float64(face.Width)/2

	// We want the crop size so that the face height corresponds to ~1/3 of crop height
	// i.e. cropSize * faceHeightRatio = face.Height
	// faceHeightRatio = 1/3, so cropSize = face.Height * 3
	cropSizeF := float64(face.Height) * 3.0

	// Ensure cropSize does not exceed image boundaries
	cropSizeF = math.Min(cropSizeF, float64(imgWidth))
	cropSizeF = math.Min(cropSizeF, float64(imgHeight))

	// Calculate cropX, cropY such that face is 1/3 from top inside crop
	cropXf := faceCenterX - cropSizeF/2

	// Vertically, place face so its top is at cropY + cropSize * faceTopRatio
	// So cropY = face.Y - cropSize * faceTopRatio
	cropYf := float64(face.Y) - cropSizeF*faceTopRatio

	// Clamp cropX and cropY so crop is fully inside the image
	if cropXf < 0 {
		cropXf = 0
	} else if cropXf+cropSizeF > float64(imgWidth) {
		cropXf = float64(imgWidth) - cropSizeF
	}

	if cropYf < 0 {
		cropYf = 0
	} else if cropYf+cropSizeF > float64(imgHeight) {
		cropYf = float64(imgHeight) - cropSizeF
	}

	return BoundingBox{
		X:      int(math.Round(cropXf)),
		Y:      int(math.Round(cropYf)),
		Width:  int(math.Round(cropSizeF)),
		Height: int(math.Round(cropSizeF)),
	}
}

func highlight(imgPath string, box BoundingBox, newPath string) error {
	if box.NoFace {
	}
	log.Printf("highlighting box for %v", imgPath)

	// Load the image
	image, err := vips.NewImageFromFile(imgPath)
	if err != nil {
		return fmt.Errorf("failed to load image: %v", err)
	}
	defer image.Close()

	// if no face, then just highlight the entire image
	if box.NoFace {
		box.Width = image.Width()
		box.Height = image.Height()
	} else {
		// comment this out if you want just the face highlighted.
		box = expandBoundingBox(box, image.Width(), image.Height())
	}

	color := vips.ColorRGBA{R: 255, G: 0, B: 0, A: 255}

	if err := image.DrawRect(color, box.X, box.Y, box.Width, box.Height, false); err != nil {
		return fmt.Errorf("failed to draw rect: %v", err)
	}

	params := vips.NewWebpExportParams()
	params.Quality = 100        // highest quality available
	params.StripMetadata = true // strip metadata for a smaller image
	params.ReductionEffort = 6  // max effort, slower encoding
	raw, _, err := image.ExportWebp(params)
	if err != nil {
		return fmt.Errorf("unable to export image %s as webp: %w", imgPath, err)
	}

	if err := os.WriteFile(newPath, raw, 0644); err != nil {
		return fmt.Errorf("unable to create file %v: %w", newPath, err)
	}

	return nil
}

func createThumbnail(path string, newPath string, box BoundingBox) error {
	// Load the image
	image, err := vips.NewImageFromFile(path)
	if err != nil {
		return fmt.Errorf("failed to load image: %v", err)
	}
	defer image.Close()

	box = expandBoundingBox(box, image.Width(), image.Height())
	if !box.NoFace {
		log.Printf("extracting area for %v", path)
		err = image.ExtractArea(box.X, box.Y, box.Width, box.Height)
		if err != nil {
			return fmt.Errorf("unable to extract area: %w", err)
		}
	}

	if err := image.Thumbnail(256, 256, vips.InterestingNone); err != nil {
		return fmt.Errorf("unable to resize image to thumbnail: %w", err)
	}

	params := vips.NewWebpExportParams()
	params.Quality = 95         // higher quality
	params.StripMetadata = true // strip metadata for a smaller image
	params.ReductionEffort = 6  // max effort, slower encoding
	raw, _, err := image.ExportWebp(params)
	if err != nil {
		return fmt.Errorf("unable to export image %s as webp: %w", path, err)
	}

	if err := os.WriteFile(newPath, raw, 0644); err != nil {
		return fmt.Errorf("unable to create file %v: %w", newPath, err)
	}

	return nil
}

func stripExtension(name string) string {
	ext := filepath.Ext(name)
	return strings.TrimSuffix(name, ext)
}

//go:embed facefinder
var cascadeFaceFinder []byte

var ErrNoFaceDetected = errors.New("no face detected")

type DetectFaceInput struct {
	// a slice of images
	Images [][]byte
}

type BoundingBox struct {
	X, Y          int
	Width, Height int
	NoFace        bool
}

type DetectedFaceResults struct {
	Faces []BoundingBox
}

type CachedDetectedFaces map[string]BoundingBox

func detectFaceGoogleVision(ctx context.Context, input DetectFaceInput) (DetectedFaceResults, error) {
	client, err := vision.NewImageAnnotatorClient(ctx)
	if err != nil {
		return DetectedFaceResults{}, err
	}
	defer client.Close()

	var detectedFaces []BoundingBox

	const batchSize = 10
	for i := 0; i < len(input.Images); i += batchSize {
		end := i + batchSize
		end = min(end, len(input.Images))
		log.Printf("detecting faces for images [%v:%v]", i, end)
		batch := input.Images[i:end]
		req := make([]*visionpb.AnnotateImageRequest, 0, len(batch))
		for _, image := range batch {
			req = append(req, &visionpb.AnnotateImageRequest{
				Image: &visionpb.Image{
					Content: image,
				},
				Features: []*visionpb.Feature{
					{
						Type: visionpb.Feature_FACE_DETECTION,
					},
				},
			})
		}

		resp, err := client.BatchAnnotateImages(ctx, &visionpb.BatchAnnotateImagesRequest{
			Requests: req,
		})
		if err != nil {
			return DetectedFaceResults{}, fmt.Errorf("unable to annotate images: %w", err)
		}

		if len(resp.Responses) != len(req) {
			return DetectedFaceResults{}, fmt.Errorf("mismatch response length, expected %v, got %v", len(req), len(resp.Responses))
		}

		for _, item := range resp.Responses {
			if len(item.FaceAnnotations) == 0 {
				detectedFaces = append(detectedFaces, BoundingBox{NoFace: true})
				continue
			}

			annotation := item.FaceAnnotations[0]
			for _, a := range item.FaceAnnotations[1:] {
				if a.DetectionConfidence > annotation.DetectionConfidence {
					annotation = a
				}
			}

			vertices := annotation.GetBoundingPoly().GetVertices()
			if len(vertices) != 4 {
				return DetectedFaceResults{}, fmt.Errorf("expected 4 vertices for rectangle, got %d", len(vertices))
			}

			minX, minY := vertices[0].X, vertices[0].Y
			maxX, maxY := vertices[0].X, vertices[0].Y
			for _, v := range vertices {
				if v.X < minX {
					minX = v.X
				}
				if v.Y < minY {
					minY = v.Y
				}
				if v.X > maxX {
					maxX = v.X
				}
				if v.Y > maxY {
					maxY = v.Y
				}
			}
			width := maxX - minX
			height := maxY - minY
			detectedFaces = append(detectedFaces, BoundingBox{
				X:      int(minX),
				Y:      int(minY),
				Width:  int(width),
				Height: int(height),
			})
		}
	}

	return DetectedFaceResults{Faces: detectedFaces}, nil
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
