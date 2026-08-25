# syntax=docker/dockerfile:1
ARG GO_VERSION=1.25
FROM golang:${GO_VERSION}-alpine AS builder
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ingest ./cmd/ingest && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/consumer ./cmd/consumer && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    if [ -d ./cmd/agent ]; then CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/agent ./cmd/agent; else printf '#!/bin/sh\nsleep infinity\n' > /out/agent && chmod +x /out/agent; fi

FROM alpine:3.20 AS runtime
RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=builder /out/ingest /out/consumer /out/api /usr/local/bin/
COPY migrations /app/migrations
RUN addgroup -S app && adduser -S app -G app && chown -R app:app /app
USER app
EXPOSE 8080 8081

FROM runtime AS agent
COPY --from=builder /out/agent /usr/local/bin/agent
EXPOSE 8080
