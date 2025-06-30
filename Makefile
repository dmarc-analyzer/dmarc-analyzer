# DMARC Analyzer Makefile

.PHONY: build-consumer build-backfill build-all run-consumer run-backfill clean help docker-build docker-run docker-stop

# Build the SQS consumer
build-consumer:
	@echo "Building SQS consumer..."
	@cd backend && go build -o ../bin/consumer ./cmd/consumer

# Build the backfill tool
build-backfill:
	@echo "Building backfill tool..."
	@cd backend && go build -o ../bin/backfill ./cmd/backfill

# Build all tools
build-all: build-consumer build-backfill
	@echo "All tools built successfully"

# Run the SQS consumer
run-consumer: build-consumer
	@echo "Running SQS consumer..."
	@./bin/consumer

# Run the backfill tool
run-backfill: build-backfill
	@echo "Running backfill tool..."
	@./bin/backfill

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	@docker build -t dmarc-analyzer .

# Run with Docker Compose
docker-run:
	@echo "Starting services with Docker Compose..."
	@docker-compose up -d

# Stop Docker Compose services
docker-stop:
	@echo "Stopping Docker Compose services..."
	@docker-compose down

# View logs
docker-logs:
	@docker-compose logs -f

# View consumer logs
docker-logs-consumer:
	@docker-compose logs -f dmarc-consumer

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/

# Show help
help:
	@echo "Available targets:"
	@echo "  build-consumer      - Build the SQS consumer"
	@echo "  build-backfill      - Build the backfill tool"
	@echo "  build-all          - Build all tools"
	@echo "  run-consumer       - Build and run the SQS consumer"
	@echo "  run-backfill       - Build and run the backfill tool"
	@echo "  docker-build       - Build Docker image"
	@echo "  docker-run         - Start services with Docker Compose"
	@echo "  docker-stop        - Stop Docker Compose services"
	@echo "  docker-logs        - View all service logs"
	@echo "  docker-logs-consumer - View consumer logs only"
	@echo "  clean              - Clean build artifacts"
	@echo "  help               - Show this help message" 