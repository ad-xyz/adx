# Multi-stage build for ADX
FROM golang:1.21-alpine AS builder

# Install build dependencies including FoundationDB client
RUN apk add --no-cache git make gcc g++ musl-dev wget && \
    wget -q https://github.com/apple/foundationdb/releases/download/7.3.27/foundationdb-clients-7.3.27.linux.x86_64.tar.gz && \
    tar xzf foundationdb-clients-7.3.27.linux.x86_64.tar.gz && \
    cp foundationdb-clients-7.3.27.linux.x86_64/lib/libfdb_c.so /usr/local/lib/ && \
    ldconfig /usr/local/lib || true

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binaries
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o adx-exchange ./cmd/adx-exchange
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o adx-miner ./cmd/adx-miner

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates curl libc6-compat && \
    mkdir -p /usr/local/lib

# Install FoundationDB client library
COPY --from=builder /usr/local/lib/libfdb_c.so /usr/local/lib/

# Create non-root user
RUN addgroup -g 1000 -S adx && \
    adduser -u 1000 -S adx -G adx

# Set working directory
WORKDIR /app

# Copy binaries from builder
COPY --from=builder /app/adx-exchange /app/adx-exchange
COPY --from=builder /app/adx-miner /app/adx-miner

# Create data directory
RUN mkdir -p /app/data && chown -R adx:adx /app

# Switch to non-root user
USER adx

# Expose ports
EXPOSE 8080 8081

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

# Default command
CMD ["/app/adx-exchange"]