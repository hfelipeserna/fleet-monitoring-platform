# syntax=docker/dockerfile:1
ARG GO_VERSION=1.25
FROM golang:${GO_VERSION}-alpine AS builder
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ingest ./cmd/ingest && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/consumer ./cmd/consumer

FROM alpine:3.20 AS runtime
RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=builder /out/ingest /out/consumer /usr/local/bin/
COPY migrations /app/migrations
EXPOSE 8080 8081
