.PHONY: help build run-deps run-migrate run-services clean test

# Default target
help:
	@echo "Available targets:"
	@echo "  run-deps     - Start PostgreSQL, Redis, and Elasticsearch"
	@echo "  run-migrate  - Run database migrations and seed data"
	@echo "  run-services - Start all microservices"
	@echo "  build        - Build all services"
	@echo "  clean        - Clean build artifacts"
	@echo "  test         - Run tests"

# Start dependencies
run-deps:
	docker-compose up -d
	@echo "Waiting for services to be ready..."
	@sleep 10

# Run migrations and seed data
run-migrate:
	go run cmd/migrate/main.go

# Start all services
run-services:
	@echo "Starting all services..."
	@echo "API Gateway: http://localhost:8080"
	@echo "Search Service: http://localhost:8081"
	@echo "Event Service: http://localhost:8082"
	@echo "Booking Service: http://localhost:8083"
	@echo "CDC Service: http://localhost:8084"
	@echo ""
	@echo "Press Ctrl+C to stop all services"
	@echo ""
	@trap 'kill %1 %2 %3 %4 %5' INT; \
	go run cmd/api-gateway/main.go & \
	go run cmd/search-service/main.go & \
	go run cmd/event-service/main.go & \
	go run cmd/booking-service/main.go & \
	go run cmd/cdc-service/main.go & \
	wait

# Build all services
build:
	go build -o bin/api-gateway cmd/api-gateway/main.go
	go build -o bin/search-service cmd/search-service/main.go
	go build -o bin/event-service cmd/event-service/main.go
	go build -o bin/booking-service cmd/booking-service/main.go
	go build -o bin/cdc-service cmd/cdc-service/main.go
	go build -o bin/migrate cmd/migrate/main.go

# Clean build artifacts
clean:
	rm -rf bin/

# Run tests
test:
	go test ./...

# Full setup
setup: run-deps run-migrate
	@echo "Setup complete! Run 'make run-services' to start all services."
