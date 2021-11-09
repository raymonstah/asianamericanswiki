package main

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matryer/is"
)

func TestHandler(t *testing.T) {
	is := is.New(t)

	t.Run("missing-token", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		Handle(w, req)
		is.Equal(http.StatusUnauthorized, w.Result().StatusCode)
	})

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
		prService := mockPrService{url: "https://github.com/raymonstah/asianamericanswiki/pull/1"}
		contextWithMockService := context.WithValue(req.Context(), mockPrServiceKey, prService)
		req = req.WithContext(contextWithMockService)

		Handle(w, req)
		result := w.Result()

		responseBody, err := ioutil.ReadAll(result.Body)
		is.NoErr(err)

		is.Equal(http.StatusCreated, result.StatusCode)
		is.Equal("pull request created: https://github.com/raymonstah/asianamericanswiki/pull/1", string(responseBody))
	})
}

type mockPrService struct {
	url string
}

func (m mockPrService) createPRWithContent(ctx context.Context, input createPRWithContentInput) (string, error) {
	return m.url, nil
}
