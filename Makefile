.PHONY: build run clean test deps

VERSION ?= 0.1.0
BINARY_NAME = yaktui
BUILD_DIR = bin

# Build the binary
build: deps
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "-X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME) .

# Run the application
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod tidy
	go mod download

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)

# Run tests
test:
	go test -v ./...

# Install to GOPATH/bin
install: deps
	go install -ldflags "-X main.version=$(VERSION)" .

# Build for multiple platforms
release:
	@echo "Building releases..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe .

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Build snapshot release locally (requires goreleaser)
snapshot:
	goreleaser release --snapshot --clean

# Show help
help:
	@echo "Available targets:"
	@echo "  build    - Build the binary"
	@echo "  run      - Build and run"
	@echo "  deps     - Download dependencies"
	@echo "  clean    - Clean build artifacts"
	@echo "  test     - Run tests"
	@echo "  install  - Install to GOPATH/bin"
	@echo "  release  - Build for multiple platforms"
	@echo "  snapshot - Build snapshot release locally (goreleaser)"
	@echo "  fmt      - Format code"
	@echo "  lint     - Lint code"

