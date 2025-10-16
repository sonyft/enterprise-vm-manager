# Multi-stage Dockerfile for production deployment

# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache     git     ca-certificates     make     upx

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download
RUN go mod verify

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build     -ldflags="-w -s -extldflags '-static'"     -a -installsuffix cgo     -o vm-manager-server     ./cmd/server

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build     -ldflags="-w -s -extldflags '-static'"     -a -installsuffix cgo     -o vmctl     ./cmd/cli

# Compress binaries
RUN upx --best --lzma vm-manager-server vmctl

# Production stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk --no-cache add     ca-certificates     tzdata     curl     wget

# Create non-root user
RUN addgroup -g 1001 -S vmmanager &&     adduser -u 1001 -S vmmanager -G vmmanager

# Create necessary directories
RUN mkdir -p /app/configs /app/logs /app/data &&     chown -R vmmanager:vmmanager /app

# Set working directory
WORKDIR /app

# Copy binaries from builder stage
COPY --from=builder /app/vm-manager-server .
COPY --from=builder /app/vmctl .

# Copy configuration files
COPY --chown=vmmanager:vmmanager configs/ ./configs/
COPY --chown=vmmanager:vmmanager internal/database/migrations/ ./internal/database/migrations/

# Make binaries executable
RUN chmod +x vm-manager-server vmctl

# Switch to non-root user
USER vmmanager

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3     CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Default command
CMD ["./vm-manager-server"]

# Metadata
LABEL maintainer="STACKIT VM Manager Team"       version="1.0.0"       description="Enterprise VM Manager API"       org.opencontainers.image.title="VM Manager API"       org.opencontainers.image.description="REST API for managing virtual machines"       org.opencontainers.image.version="1.0.0"       org.opencontainers.image.created="2025-10-15T22:00:00Z"       org.opencontainers.image.source="https://github.com/stackit/enterprise-vm-manager"       org.opencontainers.image.licenses="MIT"
