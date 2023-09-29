package cartoonize

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"regexp"
)

type Client struct {
	Debug bool
}

var url = "https://cartoonize-lkqov62dia-de.a.run.app"

// Do submits an image to the cartoonize API and returns the cartoonized image
func (c *Client) Do(imgPath string) ([]byte, error) {
	rawHTML, err := c.submitImage(url+"/cartoonize", imgPath)
	if err != nil {
		return nil, err
	}

	img, err := c.extractDownloadHref(string(rawHTML))
	if err != nil {
		return nil, err
	}

	fullDownloadPath := fmt.Sprintf("%v/%v", url, img)
	raw, err := c.downloadImage(fullDownloadPath)
	if err != nil {
		return nil, err
	}

	return raw, nil
}

func (c *Client) downloadImage(path string) ([]byte, error) {
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

func (c *Client) submitImage(url string, imagePath string) ([]byte, error) {
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

func (c *Client) extractDownloadHref(htmlContent string) (downloadHref string, err error) {
	if c.Debug {
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
