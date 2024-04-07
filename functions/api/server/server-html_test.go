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
	"github.com/tj/assert"
)

func Test_HTMLServer(t *testing.T) {
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: api.ProjectID})
	assert.NoError(t, err)
	fsClient, err := app.Firestore(ctx)
	assert.NoError(t, err)

	s := NewServer(Config{
		HumanDAO: humandao.NewDAO(fsClient),
	})

	tcs := map[string]struct {
		r                    func() *http.Request
		expectedStatus       int
		expectedContainsHTML string
	}{
		"home-page": {
			r: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			expectedStatus: http.StatusOK,
		},
		"home-page-with-extra-slash": {
			r: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "//", nil)
			},
			expectedStatus: http.StatusOK,
		},
		"humans-page": {
			r: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/humans", nil)
			},
			expectedStatus: http.StatusOK,
		},
		"humans-page-with-slash": {
			r: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/humans/", nil)
			},
			expectedStatus: http.StatusOK,
		},
		"humans-page-with-extra-slash": {
			r: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/humans//", nil)
			},
			expectedStatus: http.StatusOK,
		},
		"page-not-exist": {
			r: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/foo", nil)
			},
			expectedStatus:       http.StatusNotFound,
			expectedContainsHTML: "page does not exist",
		},
		"human-not-exist": {
			r: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/humans/foo", nil)
			},
			expectedStatus:       http.StatusNotFound,
			expectedContainsHTML: "human not found: foo",
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			req := tc.r()
			w := httptest.NewRecorder()
			s.ServeHTTP(w, req)
			assert.Equal(t, w.Result().StatusCode, tc.expectedStatus)
			if tc.expectedContainsHTML != "" {
				bytes, err := io.ReadAll(w.Result().Body)
				assert.NoError(t, err)
				assert.Contains(t, string(bytes), tc.expectedContainsHTML)
			}
		})
	}
}
