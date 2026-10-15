# Build stage, on the platform of the build machine, cross-compiling for the
# platform of the image. The Go release is the toolchain line of go.mod
# (make check-go-version checks it).
FROM --platform=$BUILDPLATFORM golang:1.27.2-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary with the version stamp and flags the Makefile gives
# release builds
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags "-X github.com/AhaSend/ahasend-cli/internal/version.Version=${VERSION} \
    -X github.com/AhaSend/ahasend-cli/internal/version.BuildTime=${BUILD_TIME} \
    -X github.com/AhaSend/ahasend-cli/internal/version.GitCommit=${GIT_COMMIT}" \
    -o ahasend .

# Final stage
FROM alpine:3.22

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 -S ahasend && \
    adduser -u 1000 -S ahasend -G ahasend

# Set working directory
WORKDIR /home/ahasend

# Copy binary from builder, with the licence notices of the CLI and of the
# software it includes
COPY --from=builder /app/ahasend /usr/local/bin/ahasend
COPY LICENSE THIRD_PARTY_NOTICES /usr/share/doc/ahasend/

# Change ownership
RUN chown -R ahasend:ahasend /home/ahasend

# Switch to non-root user
USER ahasend

# Set entrypoint
ENTRYPOINT ["ahasend"]

# Default command (show help)
CMD ["--help"]