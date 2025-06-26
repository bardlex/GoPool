# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN make build

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S gomp && \
    adduser -u 1001 -S gomp -G gomp

# Set working directory
WORKDIR /app

# Copy binaries from builder
COPY --from=builder /app/bin/ ./bin/

# Change ownership
RUN chown -R gomp:gomp /app

# Switch to non-root user
USER gomp

# Default command (can be overridden)
CMD ["./bin/stratumd"]