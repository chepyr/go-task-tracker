# Use official Go 1.25 image as the base for building the application
FROM golang:1.25 AS builder

# Set working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum from the root directory (build context) to cache dependencies
COPY go.mod go.sum ./

# Download dependencies to leverage Docker layer caching
RUN go mod download

# Copy the auth and internal directories from the build context
COPY auth/ ./auth/
COPY internal/ ./internal/

# Build the Go binary
# Output binary to /app/auth-service
RUN CGO_ENABLED=0 GOOS=linux go build -o auth-service ./auth/main.go

# Use a minimal alpine image for the runtime to reduce image size
FROM alpine:latest

# Install ca-certificates for secure HTTP requests (e.g., if you add external APIs later)
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /root/

# Copy the built binary from the builder stage
COPY --from=builder /app/auth-service .

# Copy .env file from the build context for environment variables
COPY .env ./

# Expose port 8081 (matches SERVER_PORT in .env)
EXPOSE 8081

# Run the auth service binary
CMD ["./auth-service"]