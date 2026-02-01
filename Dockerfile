# syntax=docker/dockerfile:1

ARG GO_VERSION=1.25.6
FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o app functions/api/cmd/*go

FROM gcr.io/distroless/static
COPY --from=builder /build/app /app
ENTRYPOINT ["/app"]