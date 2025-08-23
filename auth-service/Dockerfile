FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY auth-service/ ./auth-service/
COPY shared/ ./shared/

# Build the Go binary
# Output binary to /app/auth-service
RUN CGO_ENABLED=0 GOOS=linux go build -o auth-service ./auth-service/main.go

# Use a minimal alpine image for the runtime to reduce image size
FROM alpine:latest

# Install ca-certificates for secure HTTP requests
RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/auth-service .
COPY .env ./

EXPOSE 8081

CMD ["./auth-service"]