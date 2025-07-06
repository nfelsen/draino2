# Multi-stage build for draino2
# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o draino2 ./cmd/draino2

# Final stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S draino2 && \
    adduser -u 1001 -S draino2 -G draino2

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/draino2 .

# Copy configuration
COPY --from=builder /app/config ./config

# Change ownership to non-root user
RUN chown -R draino2:draino2 /app

# Switch to non-root user
USER draino2

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

# Run the binary
ENTRYPOINT ["./draino2"]
CMD ["--config-file=/app/config/draino2.yaml"] 