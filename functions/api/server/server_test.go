package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	firebase "firebase.google.com/go/v4"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/segmentio/ksuid"
	"github.com/tj/assert"
	"google.golang.org/protobuf/encoding/protojson"
)

func Test_Server(t *testing.T) {
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: api.ProjectID})
	assert.NoError(t, err)
	fsClient, err := app.Firestore(ctx)
	assert.NoError(t, err)
	humanCollection := "humans-" + ksuid.New().String()
	s := NewServer(Config{
		HumanDAO: humandao.NewDAO(fsClient, humandao.WithHumanCollectionName(humanCollection)),
	})
	t.Cleanup(func() {
		ctx := context.Background()
		humanDocs, err := fsClient.Collection(humanCollection).DocumentRefs(ctx).GetAll()
		assert.NoError(t, err)
		for _, doc := range humanDocs {
			_, err := doc.Delete(ctx)
			assert.NoError(t, err)
		}
	})

	// seed database
	_, err = s.humanDAO.AddHuman(ctx, humandao.AddHumanInput{Name: "Bob", Gender: humandao.GenderMale})
	assert.NoError(t, err)

	server := httptest.NewServer(s)
	defer server.Close()

	humansURL := server.URL + "/api/v1/humans"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, humansURL, nil)
	assert.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var humansResponse HumansResponse
	raw, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	err = protojson.Unmarshal(raw, &humansResponse)
	assert.NoError(t, err)

	assert.NotEmpty(t, humansResponse.Humans, "at least one human should exist.. did you seed the database?")
}
