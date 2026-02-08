# AsianAmericans.wiki

## About

Source code for AsianAmericans.wiki.

## Running Locally

To run tailwind watcher:

```shell
npx @tailwindcss/cli -i functions/api/server/public/static/input.css -o ./functions/api/server/public/static/output.css --watch
```

To run the emulators:

```shell
firebase emulators:start --only "auth,firestore"
```

To run the backend locally (emulators required):

```shell
go run functions/api/cmd/main.go --local
```

Or use air (for hot reload)

```shell
air
```

## Deploying manually to Cloud Run

```shell
export IMAGE_NAME=us-central1-docker.pkg.dev/asianamericans-wiki/asianamericanswiki-api/api
docker build -t $IMAGE_NAME . --platform linux/amd64
docker push $IMAGE_NAME
gcloud run deploy apiv2 --max-instances 1 --timeout 10 --region us-central1 --memory 128Mi --image ${IMAGE_NAME}:latest --allow-unauthenticated
```


## Search

There is a Firestore -> Algolia extension used for the search index.

## Protobufs

To ensure consistency between local development and CI, the tool versions are pinned in `go.mod` via `tools.go`.

You can install the Go tools with:
```shell
go install google.golang.org/protobuf/cmd/protoc-gen-go
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2
```

Regenerate with:
```shell
 cd functions/api/server && protoc -I . \
  --go_out . --go_opt paths=source_relative \
  --go-grpc_out . --go-grpc_opt paths=source_relative \
  --grpc-gateway_out . --grpc-gateway_opt paths=source_relative \
  api.proto
```

## Linting

This project uses `golangci-lint` for linting.

To install:
```shell
brew install golangci-lint
```

To run:
```shell
golangci-lint run
```

There is a pre-commit hook that runs the linter automatically. To set it up:
```shell
ln -s ../../scripts/pre-commit .git/hooks/pre-commit
```

## Test GitHub actions workflows locally

```shell
act push -j validate-proto
```
