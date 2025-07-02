# syntax=docker/dockerfile:1

FROM golang:1.24.3-alpine AS builder

WORKDIR /build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o app functions/api/cmd/*go

FROM gcr.io/distroless/static
COPY --from=builder /build/app /app
ENTRYPOINT ["/app"]
