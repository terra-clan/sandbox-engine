# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=${VERSION:-dev}" \
    -o /sandbox-engine \
    ./cmd/sandbox-engine

# Final stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata docker-cli

# Create non-root user
RUN addgroup -g 1000 sandbox && \
    adduser -u 1000 -G sandbox -D sandbox

WORKDIR /app

# Copy binary
COPY --from=builder /sandbox-engine /app/sandbox-engine

# Copy templates directory
COPY templates /app/templates

# Set ownership
RUN chown -R sandbox:sandbox /app

# Switch to non-root user
USER sandbox

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run
ENTRYPOINT ["/app/sandbox-engine"]
