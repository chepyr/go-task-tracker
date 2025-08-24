FROM golang:1.25 AS builder

WORKDIR /app

COPY shared/ /app/shared/
COPY auth-service/ /app/auth-service/

WORKDIR /app/auth-service
RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/auth-service ./main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/
COPY --from=builder /out/auth-service ./auth-service

EXPOSE 8081
CMD ["./auth-service"]
