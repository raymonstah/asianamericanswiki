package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	firebase "firebase.google.com/go/v4"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/tj/assert"
)

func Test_Server(t *testing.T) {
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: api.ProjectID})
	assert.NoError(t, err)
	fsClient, err := app.Firestore(ctx)
	assert.NoError(t, err)

	s := NewServer(Config{
		HumanDAO: humandao.NewDAO(fsClient),
	})
	server := httptest.NewServer(s)
	defer server.Close()

	humansURL := server.URL + "/api/v1/humans"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, humansURL, nil)
	assert.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var humansResponse struct {
		Humans []Human `json:"data"`
	}

	err = json.NewDecoder(resp.Body).Decode(&humansResponse)
	assert.NoError(t, err)
	assert.NotEmpty(t, humansResponse.Humans, "at least one human should exist.. did you seed the database?")
}
