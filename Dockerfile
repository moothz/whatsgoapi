# ─────────────────────────────────────────────────────────────
# whatsgoapi Dockerfile — Multi-stage build
# Go 1.25 (whatsmeow backend)
# ─────────────────────────────────────────────────────────────

# Stage 1: Builder
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git make gcc musl-dev

WORKDIR /build

# Copy dependency files first to cache module downloads
COPY go.mod go.sum ./
COPY whatsmeow-lib/ ./whatsmeow-lib/

# Download and cache Go dependencies
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the binary
RUN mkdir -p build && \
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
