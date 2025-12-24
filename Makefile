# Makefile for LinkedIn Automation POC
# For Windows PowerShell - use with: make <target> or mingw32-make <target>

.PHONY: help build run clean test deps install

# Variables
BINARY_NAME=linkedin-automation.exe
MAIN_PATH=./cmd/main.go
BUILD_DIR=./build

# Default target
help:
	@echo "LinkedIn Automation POC - Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  deps     - Download Go dependencies"
	@echo "  build    - Build the application"
	@echo "  run      - Run the application"
	@echo "  clean    - Clean build artifacts"
	@echo "  install  - Install dependencies and build"
	@echo "  test     - Run tests (if any)"
	@echo ""

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod verify

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	set CGO_ENABLED=1
	go build -o $(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BINARY_NAME)"

# Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	if exist $(BINARY_NAME) del $(BINARY_NAME)
	if exist linkedin.db del linkedin.db
	if exist logs rmdir /s /q logs
	if exist sessions rmdir /s /q sessions
	@echo "Clean complete"

# Install and build
install: deps build
	@echo "Installation complete"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...
