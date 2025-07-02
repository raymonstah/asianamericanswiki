# syntax=docker/dockerfile:1

FROM golang:1.24.3-alpine as builder

WORKDIR /build
COPY . .
RUN go build -o app functions/api/cmd/*go

FROM gcr.io/distroless/static
COPY --from=builder /build/app /app
ENTRYPOINT ["/app"]
