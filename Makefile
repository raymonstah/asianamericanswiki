.PHONY: build test dev tailwind emulators lint proto help

# Default target
help:
	@echo "Available commands:"
	@echo "  make build      - Build the project"
	@echo "  make test       - Run all tests"
	@echo "  make dev        - Run with hot reload (air)"
	@echo "  make tailwind   - Build tailwind CSS"
	@echo "  make emulators  - Start firebase emulators"
	@echo "  make lint       - Run golangci-lint"
	@echo "  make proto      - Regenerate protobufs"

build:
	go build -o bin/api functions/api/cmd/main.go

test:
	go test ./...

dev:
	air

tailwind:
	npx @tailwindcss/cli -i functions/api/server/public/static/input.css -o ./functions/api/server/public/static/output.css

emulators:
	firebase emulators:start --only "auth,firestore,storage"

lint:
	golangci-lint run

proto:
	cd functions/api/server && protoc -I . 
		--go_out . --go_opt paths=source_relative 
		--go-grpc_out . --go-grpc_opt paths=source_relative 
		--grpc-gateway_out . --grpc-gateway_opt paths=source_relative 
		api.proto
