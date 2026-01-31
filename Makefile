# Makefile

.PHONY: run stop test proto clean

# Generate Go code from Proto files
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/limiter.proto

# Start the Docker environment
run:
	docker-compose up --build

# Stop the Docker environment
stop:
	docker-compose down

# Run the test client
test:
	go run cmd/client/main.go

# Clean up binaries
clean:
	rm -f rate-limiter