# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o vault-plugin-host .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 vault && \
    adduser -D -u 1000 -G vault vault

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/vault-plugin-host .

# Change ownership
RUN chown -R vault:vault /app

# Switch to non-root user
USER vault

# Expose default port
EXPOSE 8200

# Set entrypoint
ENTRYPOINT ["/app/vault-plugin-host"]

# Default command (can be overridden)
CMD ["--help"]
