# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Download dependencies first for layer caching.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o mcp-radarr \
    ./cmd/server

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.21

# CA certificates are required for HTTPS connections to Radarr.
RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /app/mcp-radarr .

ENTRYPOINT ["./mcp-radarr"]
