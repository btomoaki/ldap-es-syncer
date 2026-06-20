# Stage 1: Build binary
FROM golang:1.26-alpine AS builder

# Set timezone and ca-certificates
RUN apk add --no-cache tzdata ca-certificates

WORKDIR /app

# Copy dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the Go application
# CGO_ENABLED=0 for a pure static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o ldap-es-syncer ./cmd/main.go

# Stage 2: Minimal runtime image
FROM alpine:3.19 AS runner

# Copy certificates and timezone database from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

WORKDIR /app

# Copy compiled binary from builder
COPY --from=builder /app/ldap-es-syncer .

# Default configuration via environment variables
ENV TZ=Asia/Tokyo

# Execute the application
ENTRYPOINT ["./ldap-es-syncer"]
