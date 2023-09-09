package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"cloud.google.com/go/firestore"
	"github.com/segmentio/ksuid"
	"github.com/urfave/cli/v2"

	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
)

func main() {
	app := &cli.App{
		Name: "cli app to add affiliate links for existing Asian Americans",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "link"},
			&cli.StringFlag{Name: "name"},
			&cli.StringFlag{Name: "human-name"},
			&cli.BoolFlag{Name: "scan"},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type Handler struct {
	FSClient  *firestore.Client
	HumanDAO  *humandao.DAO
	HumanName string
	Name      string
	Link      string // raw amazon link to be converted
	Scan      bool
}

func run(c *cli.Context) error {
	ctx := c.Context
	fsClient, err := firestore.NewClient(ctx, api.ProjectID)
	if err != nil {
		return fmt.Errorf("unable to create firestore client: %w", err)
	}

	humanDAO := humandao.NewDAO(fsClient)
	h := Handler{
		HumanDAO:  humanDAO,
		FSClient:  fsClient,
		HumanName: c.String("human-name"),
		Name:      c.String("name"),
		Link:      c.String("link"),
		Scan:      c.Bool("scan"),
	}

	if err := h.do(ctx); err != nil {
		return err
	}

	return nil
}

func (h *Handler) do(ctx context.Context) error {
	if h.HumanName != "" {
		if h.Link == "" {
			return fmt.Errorf("link is required")
		}

		path := strings.ReplaceAll(h.HumanName, " ", "-")
		path = strings.ToLower(path)
		human, err := h.HumanDAO.Human(ctx, humandao.HumanInput{Path: path})
		if err != nil {
			return err
		}
		log.Printf("Adding affiliate link for %v (%v)\n", human.Name, human.ID)

		if h.Name == "" {
			return fmt.Errorf("name is required")
		}

		url, err := createAmazonAffiliateLink("asianameri0dc-20", h.Link)
		if err != nil {
			return err
		}

		log.Println("Generated affiliate link:", url)
		human.Affiliates = append(human.Affiliates, humandao.Affiliate{
			ID:   ksuid.New().String(),
			URL:  url,
			Name: h.Name,
		})

		if err := h.HumanDAO.UpdateHuman(ctx, human); err != nil {
			return err
		}

		log.Printf("Updated %v (%v)\n", human.Name, human.ID)
		return nil
	}

	if h.Scan {
		panic("not implemented")
	}

	log.Println("Done.")
	return nil
}

func createAmazonAffiliateLink(referralID, productURL string) (string, error) {
	// Ensure the referral ID is not empty
	if referralID == "" {
		return "", fmt.Errorf("Referral ID cannot be empty")
	}

	// Ensure the product URL is not empty
	if productURL == "" {
		return "", fmt.Errorf("Product URL cannot be empty")
	}

	productURL, err := simplifyAmazonURL(productURL)
	if err != nil {
		return "", fmt.Errorf("could not simplify amazon url: %w", err)
	}

	// Parse the product URL
	parsedURL, err := url.Parse(productURL)
	if err != nil {
		return "", err
	}

	query := parsedURL.Query()

	// Remove any existing "tag" parameter in the query
	for key := range query {
		if strings.ToLower(key) == "tag" {
			delete(query, key)
		}
	}

	// Construct the Amazon affiliate link
	query.Set("tag", referralID)

	parsedURL.RawQuery = query.Encode()

	return parsedURL.String(), nil
}

func simplifyAmazonURL(amazonURL string) (string, error) {
	// Parse the input URL
	parsedURL, err := url.Parse(amazonURL)
	if err != nil {
		return "", err
	}

	// Extract the "dp" identifier from the path
	pathParts := strings.Split(parsedURL.Path, "/")
	var dpIdentifier string
	for i, part := range pathParts {
		if part == "dp" && i+1 < len(pathParts) {
			dpIdentifier = pathParts[i+1]
			break
		}
	}

	if dpIdentifier == "" {
		return "", fmt.Errorf("No 'dp' identifier found in the URL")
	}

	// Construct the shortest, most basic Amazon URL using the "dp" identifier
	smallestURL := fmt.Sprintf("https://www.amazon.com/dp/%s", dpIdentifier)

	return smallestURL, nil
}
