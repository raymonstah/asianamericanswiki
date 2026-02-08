package server

import (
	"log"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if err := os.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "127.0.0.1:8081"); err != nil {
		log.Fatal("failed to set FIREBASE_AUTH_EMULATOR_HOST environment variable", err)
	}

	if err := os.Setenv("FIRESTORE_EMULATOR_HOST", "127.0.0.1:8080"); err != nil {
		log.Fatal("failed to set FIRESTORE_EMULATOR_HOST environment variable", err)
	}
	m.Run()
}
