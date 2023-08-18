package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/raymonstah/asianamericanswiki/internal/userdao"
	"github.com/rs/zerolog"
	"github.com/segmentio/ksuid"
	"github.com/tj/assert"
)

func TestServer_User(t *testing.T) {
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: api.ProjectID})
	assert.NoError(t, err)
	client, err := app.Firestore(ctx)
	assert.NoError(t, err)

	userDAO := userdao.NewDAO(client, userdao.WithUserCollectionName("users"+ksuid.New().String()))
	humanDAO := humandao.NewDAO(client, humandao.WithHumanCollectionName("humans"+ksuid.New().String()))
	authClient, err := app.Auth(ctx)
	assert.NoError(t, err)

	humanID := ksuid.New().String()
	_, err = humanDAO.AddHuman(ctx, humandao.AddHumanInput{HumanID: humanID, Name: "Test Human"})
	assert.NoError(t, err)

	s := NewServer(Config{
		UsersDAO:   userDAO,
		HumansDAO:  humanDAO,
		AuthClient: authClient,
		Logger:     zerolog.New(zerolog.NewTestWriter(t)),
	})

	httpserver := httptest.NewServer(s)
	t.Cleanup(httpserver.Close)

	email := fmt.Sprintf("%v@test.com", ksuid.New().String())
	password := ksuid.New().String()
	userRecord, err := authClient.CreateUser(ctx, (&auth.UserToCreate{}).Email(email).Password(password))
	assert.NoError(t, err)
	defer func() {
		err := authClient.DeleteUser(ctx, userRecord.UID)
		assert.NoError(t, err)
	}()

	idToken, err := signIn(email, password)
	assert.NoError(t, err)

	// Save a human
	savePath := fmt.Sprintf("%s/humans/%s/save", httpserver.URL, humanID)
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, savePath, nil)
	assert.NoError(t, err)
	r.Header.Add("Authorization", "Bearer "+idToken)

	resp, err := http.DefaultClient.Do(r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// View a human
	viewPath := fmt.Sprintf("%s/humans/%s/view", httpserver.URL, humanID)
	r, err = http.NewRequestWithContext(ctx, http.MethodPost, viewPath, nil)
	assert.NoError(t, err)
	r.Header.Add("Authorization", "Bearer "+idToken)
	resp, err = http.DefaultClient.Do(r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Check user and validate the human was view and saved.
	userPath := fmt.Sprintf("%s/user", httpserver.URL)
	r, err = http.NewRequestWithContext(ctx, http.MethodGet, userPath, nil)
	assert.NoError(t, err)
	r.Header.Add("Authorization", "Bearer "+idToken)
	resp, err = http.DefaultClient.Do(r)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var response struct {
		User User `json:"data"`
	}

	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	userResponse := response.User

	assert.Equal(t, 1, len(userResponse.Saved))
	assert.Equal(t, humanID, userResponse.Saved[0].HumanID)
	assert.Equal(t, 1, len(userResponse.RecentlyViewed))
	assert.Equal(t, humanID, userResponse.RecentlyViewed[0].HumanID)

}
