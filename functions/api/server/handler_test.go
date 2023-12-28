package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if err := os.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "localhost:8081"); err != nil {
		log.Fatal("failed to set FIREBASE_AUTH_EMULATOR_HOST environment variable", err)
	}

	if err := os.Setenv("FIRESTORE_EMULATOR_HOST", "localhost:8080"); err != nil {
		log.Fatal("failed to set FIRESTORE_EMULATOR_HOST environment variable", err)
	}
	m.Run()
}

func signIn(email, password string) (string, error) {
	signInURL := "http://localhost:8081/identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=fake-api-key"
	body := fmt.Sprintf(`{"email": %q, "password": %q}`, email, password)
	r, err := http.NewRequest(http.MethodPost, signInURL, bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")

	if err != nil {
		return "", fmt.Errorf("unable to create sign in request: %w", err)
	}

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return "", fmt.Errorf("unable to make request: %w", err)
	}

	var results struct {
		IDToken string `json:"idToken"`
		Email   string `json:"email"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return "", fmt.Errorf("unable to decode response: %w", err)
	}

	return results.IDToken, nil
}
