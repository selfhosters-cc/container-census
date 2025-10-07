.PHONY: build run clean docker-build docker-run docker-stop test

# Build the Go binary
build:
	CGO_ENABLED=1 go build -o census ./cmd/server

# Run locally
run:
	./census

# Build and run
dev: build run

# Clean build artifacts
clean:
	rm -f census
	rm -rf data/*.db

# Build Docker image with proper docker group permissions
docker-build:
	docker build --build-arg DOCKER_GID=$$(stat -c '%g' /var/run/docker.sock 2>/dev/null || echo 999) -t container-census:latest .

# Run Docker container
docker-run: docker-build
	docker run -d \
		--name container-census \
		-p 8080:8080 \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(PWD)/config/config.yaml:/app/config/config.yaml \
		-v $(PWD)/data:/app/data \
		container-census:latest

# Stop and remove container
docker-stop:
	docker stop container-census || true
	docker rm container-census || true

# Run with docker-compose
compose-up:
	DOCKER_GID=$$(stat -c '%g' /var/run/docker.sock 2>/dev/null || echo 999) docker-compose up -d --build

# Stop docker-compose
compose-down:
	docker-compose down

# View logs
compose-logs:
	docker-compose logs -f

# Run tests
test:
	go test -v ./...

# Download dependencies
deps:
	go mod download
	go mod verify

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	go vet ./...

# Create config from example
config:
	cp config/config.yaml.example config/config.yaml
	@echo "Config created at config/config.yaml - please edit to add your hosts"

# Full setup
setup: deps config
	@echo "Setup complete. Edit config/config.yaml then run 'make dev'"
