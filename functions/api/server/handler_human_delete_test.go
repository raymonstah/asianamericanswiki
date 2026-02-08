package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"cloud.google.com/go/firestore"
	"github.com/go-chi/chi/v5"
	"github.com/raymonstah/asianamericanswiki/functions/api"
	"github.com/raymonstah/asianamericanswiki/internal/humandao"
	"github.com/segmentio/ksuid"
	"github.com/tj/assert"
)

func TestHandlerHumanDelete(t *testing.T) {
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, api.ProjectID)
	assert.NoError(t, err)

	humanCollection := "humans-" + ksuid.New().String()
	dao := humandao.NewDAO(client, humandao.WithHumanCollectionName(humanCollection))
	t.Cleanup(func() {
		ctx := context.Background()
		humanDocs, err := client.Collection(humanCollection).DocumentRefs(ctx).GetAll()
		assert.NoError(t, err)
		for _, doc := range humanDocs {
			_, err := doc.Delete(ctx)
			assert.NoError(t, err)
		}
	})

	s := NewServerHTML(ServerHTMLConfig{
		HumanDAO:   dao,
		AuthClient: NoOpAuthorizer{},
		FirebaseConfig: FirebaseConfig{
			APIKey: "fake-api-key",
		},
	})

	// Seed a human
	human, err := dao.AddHuman(ctx, humandao.AddHumanInput{
		Name:   "Test Human",
		Gender: humandao.GenderMale,
	})
	assert.NoError(t, err)

	router := chi.NewRouter()
	router.Delete("/humans/{id}", HttpHandler(s.HandlerHumanDelete).Serve(s.HandlerError))

	t.Run("successful deletion", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/humans/"+human.ID, nil)
		// Mock session cookie for parseToken
		req.AddCookie(&http.Cookie{Name: "session", Value: "fake-session"})
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusSeeOther, w.Code)
		assert.Equal(t, "/admin", w.Header().Get("Location"))

		// Verify deletion in DAO
		_, err = dao.Human(ctx, humandao.HumanInput{HumanID: human.ID})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), humandao.ErrHumanNotFound.Error())
	})

	t.Run("successful deletion with HTMX", func(t *testing.T) {
		// Seed another human
		human2, err := dao.AddHuman(ctx, humandao.AddHumanInput{
			Name:   "Test Human 2",
			Gender: humandao.GenderMale,
		})
		assert.NoError(t, err)

		req := httptest.NewRequest(http.MethodDelete, "/humans/"+human2.ID, nil)
		req.AddCookie(&http.Cookie{Name: "session", Value: "fake-session"})
		req.Header.Set("HX-Request", "true")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "/admin", w.Header().Get("HX-Redirect"))

		// Verify deletion in DAO
		_, err = dao.Human(ctx, humandao.HumanInput{HumanID: human2.ID})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), humandao.ErrHumanNotFound.Error())
	})
}
