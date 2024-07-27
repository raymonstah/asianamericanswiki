package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/httplog"
)

type Handler func(w http.ResponseWriter, r *http.Request) error

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	oplog := httplog.LogEntry(r.Context())
	w.Header().Set("Content-Type", "application/json")
	if err := h(w, r); err != nil {
		var errResponse ErrorResponse
		ok := errors.As(err, &errResponse)
		if !ok {
			oplog.Err(err).Msg("error in ServeHTTP handler")
			errResponse.Status = http.StatusInternalServerError
			errResponse.Err = err
		}
		w.WriteHeader(errResponse.Status)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{"error": errResponse.Error()}); err != nil {
			oplog.Err(err).Msg("unable to encode error response")
			return
		}

	}
}
