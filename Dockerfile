# Build stage
FROM golang:1.25.4-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH \
	go build -ldflags '-w -s' -o checkify ./cmd

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/checkify .

# Copy config template as reference (users should mount their own config.yaml)
COPY config.template.yaml .

# Set environment variables with sensible defaults
ENV LOG_LEVEL=INFO

# Default command is evaluator, but can be overridden
CMD ["./checkify", "evaluator"]
