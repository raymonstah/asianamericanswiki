package contributor

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matryer/is"
)

type mockPRService struct {
}

func (m mockPRService) createPRWithContent(ctx context.Context, input createPRWithContentInput) (string, error) {
	return "foo.com", nil
}

func TestHandler(t *testing.T) {
	is := is.New(t)

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
			"draft": false
		}	
		`)
		req := httptest.NewRequest(http.MethodPost, "/", requestBody)

		h := Handler{
			PullRequestService: mockPRService{},
		}

		h.Handle(w, req)
		result := w.Result()

		responseBody, err := ioutil.ReadAll(result.Body)
		is.NoErr(err)

		is.Equal(http.StatusCreated, result.StatusCode)
		trimmedResponse := strings.TrimSpace(string(responseBody))
		is.Equal(`{"link":"foo.com"}`, trimmedResponse)
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

		h := Handler{
			PullRequestService: mockPRService{},
		}

		h.Handle(w, req)
		result := w.Result()

		responseBody, err := ioutil.ReadAll(result.Body)
		is.NoErr(err)

		is.Equal(http.StatusBadRequest, result.StatusCode)
		trimmedResponse := strings.TrimSpace(string(responseBody))
		is.Equal(`{"error":"name is required"}`, trimmedResponse)
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
			"draft": false
		}	
		`)
		req := httptest.NewRequest(http.MethodPost, "/?test=ok", requestBody)

		h := Handler{
			PullRequestService: mockPRService{},
		}

		h.Handle(w, req)
		result := w.Result()

		responseBody, err := ioutil.ReadAll(result.Body)
		is.NoErr(err)

		is.Equal(http.StatusCreated, result.StatusCode)
		trimmedResponse := strings.TrimSpace(string(responseBody))
		is.Equal(`{"link":"https://github.com/raymonstah/asianamericanswiki/pulls/1"}`, trimmedResponse)
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
			"draft": false
		}	
		`)
		req := httptest.NewRequest(http.MethodPost, "/?test=dupe", requestBody)

		h := Handler{
			PullRequestService: mockPRService{},
		}

		h.Handle(w, req)
		result := w.Result()

		responseBody, err := ioutil.ReadAll(result.Body)
		is.NoErr(err)

		is.Equal(http.StatusUnprocessableEntity, result.StatusCode)
		trimmedResponse := strings.TrimSpace(string(responseBody))
		is.Equal(`{"error":"branch already exists"}`, trimmedResponse)
	})
}
