package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	firebase "firebase.google.com/go/v4"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/raymonstah/asianamericanswiki/internal/userdao"
	"github.com/rs/zerolog"
	"github.com/segmentio/ksuid"
	"github.com/tj/assert"
)

func TestServer_HumansList(t *testing.T) {
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: api.ProjectID})
	assert.NoError(t, err)
	client, err := app.Firestore(ctx)
	assert.NoError(t, err)

	humanDAO := humandao.NewDAO(client)
	s := NewServer(Config{
		HumanDAO: humanDAO,
		Logger:   zerolog.New(zerolog.NewTestWriter(t)),
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	err = s.HumansList(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)

	// check for cache hit in logs.
	err = s.HumansList(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
}

func TestServer_HumansList_OrderByMostViewed(t *testing.T) {
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: api.ProjectID})
	assert.NoError(t, err)
	client, err := app.Firestore(ctx)
	assert.NoError(t, err)

	humanDAO := humandao.NewDAO(client, humandao.WithHumanCollectionName("humans-"+ksuid.New().String()))
	n := 15
	for i := 0; i < n; i++ {
		human, err := humanDAO.AddHuman(ctx, humandao.AddHumanInput{Name: fmt.Sprintf("Human %v", i), Gender: humandao.GenderFemale})
		assert.NoError(t, err)
		human.Views = int64(i + 1)

		err = humanDAO.UpdateHuman(ctx, human)
		assert.NoError(t, err)
	}

	s := NewServer(Config{
		HumanDAO: humanDAO,
		Logger:   zerolog.New(zerolog.NewTestWriter(t)),
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/?orderBy=views&direction=desc", nil)

	err = s.HumansList(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
	var respBody struct {
		Humans []Human `json:"data"`
	}
	err = json.NewDecoder(w.Result().Body).Decode(&respBody)
	assert.NoError(t, err)

	assert.Len(t, respBody.Humans, 10) // default limit is 10
	for i, human := range respBody.Humans {
		assert.Equal(t, fmt.Sprintf("Human %v", n-i-1), human.Name)
	}
}

func TestServer_HumansByID(t *testing.T) {
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: api.ProjectID})
	assert.NoError(t, err)
	client, err := app.Firestore(ctx)
	assert.NoError(t, err)

	userDAO := userdao.NewDAO(client, userdao.WithUserCollectionName("users-"+ksuid.New().String()))
	humanDAO := humandao.NewDAO(client, humandao.WithHumanCollectionName("humans-"+ksuid.New().String()))

	var humans []humandao.Human
	for i := 0; i < 10; i++ {
		human, err := humanDAO.AddHuman(ctx, humandao.AddHumanInput{Name: fmt.Sprintf("Human %v", i), Gender: humandao.GenderFemale})
		assert.NoError(t, err)
		humans = append(humans, human)
	}

	humanIDs := make([]string, 0, len(humans))
	for _, human := range humans {
		humanIDs = append(humanIDs, human.ID)
	}

	s := NewServer(Config{
		UserDAO:    userDAO,
		HumanDAO:   humanDAO,
		AuthClient: NoOpAuthorizer{},
		Logger:     zerolog.New(zerolog.NewTestWriter(t)),
	})

	httpserver := httptest.NewServer(s)
	t.Cleanup(httpserver.Close)

	raw, err := json.Marshal(humanIDs)
	assert.NoError(t, err)
	body := bytes.NewReader(raw)
	req, err := http.NewRequest(http.MethodPost, httpserver.URL+"/api/v1/humans/search", body)
	req.Header.Set("Authorization", "Bearer XXXXXXXXXXXX")
	assert.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// check response body
	var respBody struct {
		Humans []humandao.Human `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	assert.NoError(t, err)
	assert.Equal(t, len(humanIDs), len(respBody.Humans))
	for i, human := range respBody.Humans {
		assert.Equal(t, humanIDs[i], human.ID)
		assert.Equal(t, humans[i].Name, human.Name)
	}
}

func TestServer_HumanWithAffiliateLinks(t *testing.T) {
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: api.ProjectID})
	assert.NoError(t, err)
	client, err := app.Firestore(ctx)
	assert.NoError(t, err)

	userDAO := userdao.NewDAO(client, userdao.WithUserCollectionName("users-"+ksuid.New().String()))
	humanDAO := humandao.NewDAO(client, humandao.WithHumanCollectionName("humans-"+ksuid.New().String()))

	n := 10
	affiliates := make([]humandao.Affiliate, 0, n)
	for i := 0; i < n; i++ {
		affiliates = append(affiliates, humandao.Affiliate{
			URL:   fmt.Sprintf("https://affiliate-link-%v.com", i),
			Name:  fmt.Sprintf("Affiliate Link %v", i),
			Image: fmt.Sprintf("https://affiliate-link-image-%v.com", i),
		})
	}

	human, err := humanDAO.AddHuman(ctx, humandao.AddHumanInput{Name: "Human Affiliate", Affiliates: affiliates, Gender: humandao.GenderFemale})
	assert.NoError(t, err)

	s := NewServer(Config{
		UserDAO:    userDAO,
		HumanDAO:   humanDAO,
		AuthClient: NoOpAuthorizer{},
		Logger:     zerolog.New(zerolog.NewTestWriter(t)),
	})

	httpserver := httptest.NewServer(s)
	t.Cleanup(httpserver.Close)

	req, err := http.NewRequest(http.MethodGet, httpserver.URL+fmt.Sprintf("/api/v1/humans/%v", human.Path), nil)
	assert.NoError(t, err)
	req.Header.Set("Authorization", "Bearer XXXXXXXXXXXX")

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// check response body
	var respBody struct {
		Human Human `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	assert.NoError(t, err)

	assert.Len(t, respBody.Human.Affiliates, n)
	for i := 0; i < n; i++ {
		assert.NotEmpty(t, respBody.Human.Affiliates[i].ID)
		assert.Equal(t, human.Affiliates[i].URL, respBody.Human.Affiliates[i].URL)
		assert.Equal(t, human.Affiliates[i].Name, respBody.Human.Affiliates[i].Name)
		assert.Equal(t, human.Affiliates[i].Image, respBody.Human.Affiliates[i].Image)
	}
}
