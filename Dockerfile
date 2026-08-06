# Build stage
FROM golang:1.25.4-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o openvpn-mng ./cmd/server

# The application is statically linked (CGO_ENABLED=0), so it can run in a
# minimal image without downloading Alpine packages during each build.
FROM scratch

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/openvpn-mng .

# Copy web templates and static files
COPY --from=builder /app/web ./web

# Copy Swagger docs if they exist
COPY --from=builder /app/docs ./docs

# Expose port
EXPOSE 8080

# Run the application
CMD ["./openvpn-mng"]
