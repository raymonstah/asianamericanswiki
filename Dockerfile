# syntax=docker/dockerfile:1

FROM golang:1.23-alpine as builder

WORKDIR /build
COPY . .
RUN go build -o app functions/api/cmd/*go

FROM alpine:latest
COPY --from=builder /build/app /app
ENTRYPOINT ["/app"]
