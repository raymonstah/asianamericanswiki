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

To ensure consistency between local development and CI, please use the following tool versions:
- `protoc`: [v29.3](https://github.com/protocolbuffers/protobuf/releases/tag/v29.3)
- `protoc-gen-go`: v1.36.11
- `protoc-gen-go-grpc`: v1.6.0
- `protoc-gen-grpc-gateway`: v2.27.7
- `protoc-gen-openapiv2`: v2.27.7

You can install the Go tools with:
```shell
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.0
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.27.7
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.27.7
```

Regenerate with:
```shell
 protoc -I functions/api/server --go_out ./functions/api/server --go_opt paths=source_relative \
  --go-grpc_out ./functions/api/server --go-grpc_opt paths=source_relative \
  --grpc-gateway_out ./functions/api/server --grpc-gateway_opt paths=source_relative \
  ./functions/api/server/api.proto
```

## Test GitHub actions workflows locally

```shell
act push -j validate-proto
```
