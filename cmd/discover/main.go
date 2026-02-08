package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"cloud.google.com/go/firestore"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/raymonstah/asianamericanswiki/internal/imageutil"
	"github.com/raymonstah/asianamericanswiki/internal/xai"
	"github.com/urfave/cli/v2"

	"cloud.google.com/go/storage"
)

var opts struct {
	Dry          bool
	XAIToken     string
	Enrich       bool
	MaxDiscovery int
	UseProd      bool
	GenImage     bool
}

func main() {
	app := &cli.App{
		Name:  "discover",
		Usage: "Discover new Asian Americans from various sources",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "dry", Value: false, Destination: &opts.Dry},
			&cli.StringFlag{Name: "xai-token", EnvVars: []string{"XAI_API_KEY"}, Destination: &opts.XAIToken},
			&cli.BoolFlag{Name: "enrich", Value: false, Usage: "Use XAI to enrich discovered humans", Destination: &opts.Enrich},
			&cli.IntFlag{Name: "max", Value: 10, Usage: "Maximum number of new humans to add per run", Destination: &opts.MaxDiscovery},
			&cli.BoolFlag{Name: "use-prod", Value: false, Usage: "Use production Firestore", Destination: &opts.UseProd},
			&cli.BoolFlag{Name: "gen-image", Value: false, Usage: "Generate images for discovered humans", Destination: &opts.GenImage},
		},
		Commands: []*cli.Command{
			{
				Name:   "wikipedia",
				Usage:  "Discover from Wikipedia categories",
				Action: discoverWikipedia,
			},
			{
				Name:   "brainstorm",
				Usage:  "Brainstorm new humans using XAI based on a query",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "query", Required: true, Usage: "The query to brainstorm for (e.g. 'Asian American tech founders')"},
				},
				Action: brainstorm,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type Discoverer struct {
	dao           *humandao.DAO
	existing      map[string]struct{}
	xaiClient     *xai.Client
	storageClient *storage.Client
	uploader      *imageutil.Uploader
}

func newDiscoverer(ctx context.Context) (*Discoverer, error) {
	if !opts.UseProd {
		if err := os.Setenv("FIRESTORE_EMULATOR_HOST", "localhost:8080"); err != nil {
			return nil, err
		}
	} else {
		if err := os.Unsetenv("FIRESTORE_EMULATOR_HOST"); err != nil {
			return nil, err
		}
	}

	client, err := firestore.NewClient(ctx, api.ProjectID)
	if err != nil {
		return nil, err
	}
	dao := humandao.NewDAO(client)
	humans, err := dao.ListHumans(ctx, humandao.ListHumansInput{
		Limit:         10000,
		IncludeDrafts: true,
	})
	if err != nil {
		return nil, err
	}

	existing := make(map[string]struct{})
	for _, h := range humans {
		existing[strings.ToLower(h.Name)] = struct{}{}
		existing[strings.ToLower(h.Path)] = struct{}{}
	}

	var xClient *xai.Client
	if opts.XAIToken != "" {
		xClient = xai.New(opts.XAIToken)
	}

	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	uploader := imageutil.NewUploader(storageClient, dao)

	return &Discoverer{
		dao:           dao,
		existing:      existing,
		xaiClient:     xClient,
		storageClient: storageClient,
		uploader:      uploader,
	}, nil
}

func (d *Discoverer) FindImageURL(ctx context.Context, name string) (string, error) {
	// 1. Try Wikipedia
	baseURL := "https://en.wikipedia.org/w/api.php"
	params := url.Values{}
	params.Set("action", "query")
	params.Set("titles", name)
	params.Set("prop", "pageimages")
	params.Set("pithumbsize", "1000")
	params.Set("format", "json")

	client := &http.Client{}
	req, err := http.NewRequest("GET", baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "AsianAmericansWiki/1.0 (https://asianamericans.wiki; contact@asianamericans.wiki)")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var wikiResp struct {
		Query struct {
			Pages map[string]struct {
				Thumbnail struct {
					Source string `json:"source"`
				} `json:"thumbnail"`
			} `json:"pages"`
		} `json:"query"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&wikiResp); err == nil {
		for _, page := range wikiResp.Query.Pages {
			if page.Thumbnail.Source != "" {
				return page.Thumbnail.Source, nil
			}
		}
	}

	// 2. If no Wikipedia image, just return empty and let GenerateAndUploadImage handle it
	// In a more advanced version, we could use a Search API here.
	return "", nil
}

func (d *Discoverer) GenerateAndUploadImage(ctx context.Context, human humandao.Human) error {
	if d.xaiClient == nil {
		return fmt.Errorf("XAI client is required for image generation")
	}

	sourceImageURL, _ := d.FindImageURL(ctx, human.Name)
	if sourceImageURL == "" {
		fmt.Printf("No source image found for %s, skipping generation (user requested no text-to-image)\n", human.Name)
		return nil
	}

	prompt := xai.DefaultImagePrompt(human.Name)

	fmt.Printf("Generating image for %s using source: %s...\n", human.Name, sourceImageURL)
	imageURLs, err := d.xaiClient.GenerateImage(ctx, xai.GenerateImageInput{
		Prompt: prompt,
		N:      1,
		Image:  sourceImageURL,
	})
	if err != nil {
		return fmt.Errorf("unable to generate image: %w", err)
	}

	if len(imageURLs) == 0 {
		return fmt.Errorf("no images generated")
	}

	resp, err := http.Get(imageURLs[0])
	if err != nil {
		return fmt.Errorf("unable to download generated image: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("unable to read generated image: %w", err)
	}

	if opts.Dry {
		fmt.Printf("DRY RUN: Would upload images for %s to GCS\n", human.Name)
		return nil
	}

	human.AIGenerated = true
	_, err = d.uploader.UploadHumanImages(ctx, human, raw)
	return err
}

func discoverWikipedia(c *cli.Context) error {
	ctx := c.Context
	d, err := newDiscoverer(ctx)
	if err != nil {
		return err
	}

	categories := []string{
		"Asian-American_businesspeople",
		"Asian-American_YouTube_personalities",
		"Asian-American_social_media_personalities",
		"American_chief_executives_of_Asian_descent",
		"American_founders_of_Asian_descent",
	}

	count := 0
	for _, cat := range categories {
		if count >= opts.MaxDiscovery {
			break
		}
		fmt.Printf("Searching category: %s\n", cat)
		members, err := d.fetchWikipediaCategoryMembers(cat)
		if err != nil {
			fmt.Printf("Error fetching category %s: %v\n", cat, err)
			continue
		}

		for _, member := range members {
			if count >= opts.MaxDiscovery {
				break
			}
			name := member
			if _, ok := d.existing[strings.ToLower(name)]; ok {
				continue
			}

			fmt.Printf("Found new person: %s\n", name)

			input := humandao.AddHumanInput{
				Name:   name,
				Draft:  true,
				Gender: humandao.GenderNonBinary,
			}

			if opts.Enrich && d.xaiClient != nil {
				fmt.Printf("Enriching %s with XAI...\n", name)
				enriched, err := d.xaiClient.GenerateHuman(ctx, xai.GenerateHumanRequest{
					Name: name,
					Tags: []string{cat},
				})
				if err == nil {
					fmt.Printf("Enriched data for %s: %+v\n", enriched.Name, enriched)
					input.Name = enriched.Name // Use the full name from AI
					input.Description = enriched.Description
					input.DOB = enriched.DOB
					input.DOD = enriched.DOD
					input.Ethnicity = enriched.Ethnicity
					input.Gender = humandao.Gender(enriched.Gender)
					if input.Gender == "" {
						input.Gender = humandao.GenderNonBinary
					}
					input.Website = enriched.Website
					input.Twitter = enriched.Twitter
					input.Location = enriched.Location
					for _, tag := range enriched.Tags {
						t := strings.ToLower(strings.TrimSpace(tag))
						if humandao.IsValidTag(t) {
							input.Tags = append(input.Tags, t)
						}
					}

					// Double check if name changed and if it's already in database
					if strings.ToLower(input.Name) != strings.ToLower(name) {
						if _, ok := d.existing[strings.ToLower(input.Name)]; ok {
							fmt.Printf("Skipping %s (renamed from %s) as it already exists in database\n", input.Name, name)
							continue
						}
					}
				} else {
					if strings.Contains(err.Error(), "not an individual human") {
						fmt.Printf("Skipping %s as it is not an individual human\n", name)
						continue
					}
					fmt.Printf("Error enriching %s: %v\n", name, err)
				}
			}

			if opts.Dry {
				count++
				continue
			}

			fmt.Printf("Saving %s as draft...\n", input.Name)
			human, err := d.dao.AddHuman(ctx, input)
			if err != nil {
				fmt.Printf("Error adding human %s: %v\n", name, err)
				continue
			}

			// update existing map to prevent duplicates in the same run
			d.existing[strings.ToLower(human.Name)] = struct{}{}
			d.existing[strings.ToLower(human.Path)] = struct{}{}

			if opts.GenImage {
				err := d.GenerateAndUploadImage(ctx, human)
				if err != nil {
					fmt.Printf("Error generating image for %s: %v\n", name, err)
				}
			}
			count++
		}
	}

	return nil
}

func (d *Discoverer) fetchWikipediaCategoryMembers(category string) ([]string, error) {
	baseURL := "https://en.wikipedia.org/w/api.php"
	params := url.Values{}
	params.Set("action", "query")
	params.Set("list", "categorymembers")
	params.Set("cmtitle", "Category:"+category)
	params.Set("cmlimit", "500")
	params.Set("format", "json")

	client := &http.Client{}
	req, err := http.NewRequest("GET", baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "AsianAmericansWiki/1.0 (https://asianamericans.wiki; contact@asianamericans.wiki)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Query struct {
			CategoryMembers []struct {
				Title string `json:"title"`
			} `json:"categorymembers"`
		} `json:"query"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unable to decode wikipedia response: %w (body: %s)", err, string(body))
	}

	var members []string
	for _, m := range result.Query.CategoryMembers {
		if strings.HasPrefix(m.Title, "Category:") {
			// Optional: recurse into subcategories
			continue
		}
		members = append(members, m.Title)
	}

	return members, nil
}

func brainstorm(c *cli.Context) error {
	ctx := c.Context
	d, err := newDiscoverer(ctx)
	if err != nil {
		return err
	}

	if d.xaiClient == nil {
		return fmt.Errorf("XAI client is required for brainstorming. Provide --xai-token or set XAI_API_KEY.")
	}

	query := c.String("query")
	fmt.Printf("Brainstorming for: %s\n", query)

	names, err := d.xaiClient.Brainstorm(ctx, xai.BrainstormInput{Query: query})
	if err != nil {
		return err
	}

	count := 0
	for _, name := range names {
		if count >= opts.MaxDiscovery {
			break
		}
		if _, ok := d.existing[strings.ToLower(name)]; ok {
			continue
		}

		fmt.Printf("Found new person: %s\n", name)

		input := humandao.AddHumanInput{
			Name:   name,
			Draft:  true,
			Gender: humandao.GenderNonBinary,
		}

		if opts.Enrich {
			fmt.Printf("Enriching %s with XAI...\n", name)
			enriched, err := d.xaiClient.GenerateHuman(ctx, xai.GenerateHumanRequest{
				Name: name,
				Tags: []string{query},
			})
			if err == nil {
				fmt.Printf("Enriched data for %s: %+v\n", enriched.Name, enriched)
				input.Name = enriched.Name // Use full name
				input.Description = enriched.Description
				input.DOB = enriched.DOB
				input.DOD = enriched.DOD
				input.Ethnicity = enriched.Ethnicity
				input.Gender = humandao.Gender(enriched.Gender)
				if input.Gender == "" {
					input.Gender = humandao.GenderNonBinary
				}
				input.Website = enriched.Website
				input.Twitter = enriched.Twitter
				input.Location = enriched.Location
				for _, tag := range enriched.Tags {
					t := strings.ToLower(strings.TrimSpace(tag))
					if humandao.IsValidTag(t) {
						input.Tags = append(input.Tags, t)
					}
				}

				// Check if renamed person already exists
				if strings.ToLower(input.Name) != strings.ToLower(name) {
					if _, ok := d.existing[strings.ToLower(input.Name)]; ok {
						fmt.Printf("Skipping %s (renamed from %s) as it already exists in database\n", input.Name, name)
						continue
					}
				}
			} else {
				if strings.Contains(err.Error(), "not an individual human") {
					fmt.Printf("Skipping %s as it is not an individual human\n", name)
					continue
				}
				fmt.Printf("Error enriching %s: %v\n", name, err)
			}
		}

		if opts.Dry {
			count++
			continue
		}

		fmt.Printf("Saving %s as draft...\n", input.Name)
		human, err := d.dao.AddHuman(ctx, input)
		if err != nil {
			fmt.Printf("Error adding human %s: %v\n", name, err)
			continue
		}

		if opts.GenImage {
			err := d.GenerateAndUploadImage(ctx, human)
			if err != nil {
				fmt.Printf("Error generating image for %s: %v\n", name, err)
			}
		}
		count++
	}

	return nil
}