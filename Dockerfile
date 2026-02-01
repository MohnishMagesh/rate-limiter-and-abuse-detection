# Stage 1: Builder
FROM golang:1.24.5-alpine AS builder

WORKDIR /app

# Copy dependency files first
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the binary
# 1. CGO_ENABLED=0: Disables C bindings (Faster build, purely static binary)
# 2. -ldflags="-w -s": Strips debug information (Makes binary smaller)
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o rate-limiter cmd/server/main.go

# Stage 2: Runner
FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/rate-limiter .

EXPOSE 50051

CMD ["./rate-limiter"]