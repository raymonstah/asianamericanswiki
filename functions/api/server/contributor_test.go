package server

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tj/assert"

	"github.com/raymonstah/asianamericanswiki/internal/contributor"
)

type mockPRService struct {
}

func (m mockPRService) CreatePRWithContent(ctx context.Context, input contributor.CreatePRWithContentInput) (string, error) {
	return "foo.com", nil
}

func TestHandler(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		w := httptest.NewRecorder()
		requestBody := strings.NewReader(`{
			"name": "Bruce Lee",
			"aka": ["Young Dragon"],
			"dob": "2000-02-22",
			"tags": ["a", "b", "c"],
			"website": "https://brucelee.com",
			"ethnicity": ["Chinese"],
			"birthLocation": "San Francisco",
			"location": ["Oakland", "Seattle"],
			"twitter": "https://twitter.com/brucelee",
			"draft": false,
            "description": "foo bar"
		}	
		`)
		req := httptest.NewRequest(http.MethodPost, "/", requestBody)

		contributorClient := contributor.Client{
			PullRequestService: mockPRService{},
		}

		server := NewServer(Config{Contributor: contributorClient})
		Handler(server.Contribute).ServeHTTP(w, req)
		result := w.Result()

		responseBody, err := ioutil.ReadAll(result.Body)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusCreated, result.StatusCode)
		trimmedResponse := strings.TrimSpace(string(responseBody))
		assert.Equal(t, `{"data":{"link":"foo.com"}}`, trimmedResponse)
	})

	t.Run("missing-name", func(t *testing.T) {
		w := httptest.NewRecorder()
		requestBody := strings.NewReader(`{
			"aka": ["Young Dragon"],
			"dob": "2000-02-22",
			"tags": ["a", "b", "c"],
			"website": "https://brucelee.com",
			"ethnicity": ["Chinese"],
			"birthLocation": "San Francisco",
			"location": ["Oakland", "Seattle"],
			"twitter": "https://twitter.com/brucelee",
			"draft": false
		}	
		`)
		req := httptest.NewRequest(http.MethodPost, "/", requestBody)

		contributorClient := contributor.Client{
			PullRequestService: mockPRService{},
		}

		server := NewServer(Config{Contributor: contributorClient})
		Handler(server.Contribute).ServeHTTP(w, req)
		result := w.Result()

		responseBody, err := ioutil.ReadAll(result.Body)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, result.StatusCode)

		trimmedResponse := strings.TrimSpace(string(responseBody))
		assert.Equal(t, `{"error":"name is required"}`, trimmedResponse)
	})

	t.Run("test-flag-ok", func(t *testing.T) {
		w := httptest.NewRecorder()
		requestBody := strings.NewReader(`{
			"name": "Bruce Lee",
			"aka": ["Young Dragon"],
			"dob": "2000-02-22",
			"tags": ["a", "b", "c"],
			"website": "https://brucelee.com",
			"ethnicity": ["Chinese"],
			"birthLocation": "San Francisco",
			"location": ["Oakland", "Seattle"],
			"twitter": "https://twitter.com/brucelee",
			"draft": false,
			"description": "foo bar"
		}	
		`)
		req := httptest.NewRequest(http.MethodPost, "/?test=ok", requestBody)
		contributorClient := contributor.Client{
			PullRequestService: mockPRService{},
		}

		server := NewServer(Config{Contributor: contributorClient})
		Handler(server.Contribute).ServeHTTP(w, req)
		result := w.Result()

		responseBody, err := ioutil.ReadAll(result.Body)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusCreated, result.StatusCode)
		trimmedResponse := strings.TrimSpace(string(responseBody))
		assert.Equal(t, `{"link":"https://github.com/raymonstah/asianamericanswiki/pulls/1"}`, trimmedResponse)
	})

	t.Run("test-flag-dupe", func(t *testing.T) {
		w := httptest.NewRecorder()
		requestBody := strings.NewReader(`{
			"name": "Bruce Lee",
			"aka": ["Young Dragon"],
			"dob": "2000-02-22",
			"tags": ["a", "b", "c"],
			"website": "https://brucelee.com",
			"ethnicity": ["Chinese"],
			"birthLocation": "San Francisco",
			"location": ["Oakland", "Seattle"],
			"twitter": "https://twitter.com/brucelee",
			"draft": false,
			"description": "foo bar"
		}	
		`)
		req := httptest.NewRequest(http.MethodPost, "/?test=dupe", requestBody)

		contributorClient := contributor.Client{
			PullRequestService: mockPRService{},
		}

		server := NewServer(Config{Contributor: contributorClient})
		Handler(server.Contribute).ServeHTTP(w, req)
		result := w.Result()

		responseBody, err := ioutil.ReadAll(result.Body)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusUnprocessableEntity, result.StatusCode)
		trimmedResponse := strings.TrimSpace(string(responseBody))
		assert.Equal(t, `{"error":"branch already exists"}`, trimmedResponse)
	})
}
