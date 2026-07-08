BINARY   := ghoss
MODULE   := github.com/ghoss
BUILD_DIR := ./bin
COMMAND  := ./cmd/ghoss

.DEFAULT_GOAL := build

.PHONY: build clean run fmt

build:
	@echo "Building $(BINARY)..."
	go build -o $(BUILD_DIR)/$(BINARY) $(COMMAND)/main.go

clean:
	@echo "Cleaning..."
	rm -f $(BUILD_DIR)/$(BINARY)
	go clean

run: build
	$(BUILD_DIR)/$(BINARY)

fmt:
	go fmt ./...
