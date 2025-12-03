# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s' \
    -o /proxy \
    ./main.go

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS support and wget for healthcheck
RUN apk --no-cache add ca-certificates wget

# Create non-root user
RUN addgroup -S proxy && adduser -S proxy -G proxy

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /proxy /app/proxy

# Create directory for database with proper permissions
RUN mkdir -p /app/data && chown -R proxy:proxy /app

# Switch to non-root user
USER proxy

# # Health check (uses default port 80, override PORT env var if needed)
# HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
#     CMD wget --no-verbose --tries=1 --spider http://localhost/health || exit 1

# Run the application
ENTRYPOINT ["/app/proxy"]
EXPOSE 80

