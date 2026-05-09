# ─────────────────────────────────────────────────────────────
# whatsgoapi Dockerfile — Multi-stage build
# Go 1.25 (whatsmeow backend)
# ─────────────────────────────────────────────────────────────

# Stage 1: Builder
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git make gcc musl-dev

WORKDIR /build

# Copy the whatsgoapi source (context is ./whatsgoapi)
COPY . .

# Initialize and tidy the Go module, then build
# go mod tidy requires the whatsmeow-lib submodule to be initialized
RUN if [ ! -f go.mod ]; then \
        go mod init whatsgo && \
        go mod edit -replace go.mau.fi/whatsmeow=./whatsmeow-lib; \
    fi && \
    go mod tidy && \
    mkdir -p build && \
    go build -v -o build/whatsgo cmd/whatsgo/main.go

# ─────────────────────────────────────────────────────────────
# Stage 2: Runtime image (minimal)
# ─────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl ffmpeg

WORKDIR /app

# Copy compiled binary from builder
COPY --from=builder /build/build/whatsgo /app/whatsgo

# Media files and SQLite databases are stored here
RUN mkdir -p /app/files /app/dbdata /app/logs

EXPOSE 8080

CMD ["/app/whatsgo"]
