FROM golang:1.25 AS builder

WORKDIR /app

COPY shared/ /app/shared/
COPY tasks-service/ /app/tasks-service/

WORKDIR /app/tasks-service
RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/tasks-service ./main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/
COPY --from=builder /out/tasks-service ./tasks-service

EXPOSE 8082
CMD ["./tasks-service"]
