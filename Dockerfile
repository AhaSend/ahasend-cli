# Build stage
FROM golang:1.23-alpine AS builder

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

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ahasend .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 -S ahasend && \
    adduser -u 1000 -S ahasend -G ahasend

# Set working directory
WORKDIR /home/ahasend

# Copy binary from builder
COPY --from=builder /app/ahasend /usr/local/bin/ahasend

# Change ownership
RUN chown -R ahasend:ahasend /home/ahasend

# Switch to non-root user
USER ahasend

# Set entrypoint
ENTRYPOINT ["ahasend"]

# Default command (show help)
CMD ["--help"]