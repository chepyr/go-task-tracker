# ./tasks-service/Dockerfile
FROM golang:1.25 AS builder

WORKDIR /app

# Копируем go.mod и go.sum из корня проекта
COPY ../go.mod ../go.sum ./
RUN go mod download

# Копируем код tasks-service и shared
COPY tasks-service/ ./tasks-service/
COPY shared/ ./shared/

# Сборка бинарника
RUN CGO_ENABLED=0 GOOS=linux go build -o tasks-service ./tasks-service/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/
COPY --from=builder /app/tasks-service .
COPY .env ./

EXPOSE 8082

CMD ["./tasks-service"]