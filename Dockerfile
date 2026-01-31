# Stage 1: Builder (Compiles the Go code)
FROM golang:1.24.5-alpine AS builder

WORKDIR /app

# Copy dependency files first (for better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the binary
# -o main: output file name
# cmd/server/main.go: input file
RUN go build -o rate-limiter cmd/server/main.go

# Stage 2: Runner (Tiny Alpine image)
FROM alpine:latest

WORKDIR /root/

# Copy only the compiled binary from the builder stage
COPY --from=builder /app/rate-limiter .

# Expose the gRPC port
EXPOSE 50051

# Default command: run the binary
# We can override flags in docker-compose
CMD ["./rate-limiter"]