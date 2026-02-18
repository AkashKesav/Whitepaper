# Build stage for the workflow worker
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the workflow worker
RUN CGO_ENABLED=0 go build -o workflow-worker ./cmd/workflow

# Final stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates curl

# Copy the binary from builder
COPY --from=builder /app/workflow-worker .

# Health check
HEALTHCHECK --interval=10s --timeout=5s --retries=5 \
  CMD curl -f http://localhost:8081/health || exit 1

# Run the workflow worker
CMD ["./workflow-worker"]
