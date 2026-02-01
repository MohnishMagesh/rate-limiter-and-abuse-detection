FROM golang:1.24.5-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o rate-limiter cmd/server/main.go

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/rate-limiter .

EXPOSE 50051

CMD ["./rate-limiter"]