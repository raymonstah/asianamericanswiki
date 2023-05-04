package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	firebase "firebase.google.com/go/v4"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/rs/zerolog"
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
		HumansDAO: humanDAO,
		Logger:    zerolog.New(zerolog.NewTestWriter(t)),
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
