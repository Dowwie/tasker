.PHONY: build test lint clean cross-compile help

BINARY_NAME := tasker
BUILD_DIR := ./bin
CMD_PATH := ./cmd/tasker

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_DATE)"

PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64

help:
	@echo "Available targets:"
	@echo "  build         - Build binary for current platform"
	@echo "  test          - Run tests with coverage"
	@echo "  lint          - Run golangci-lint"
	@echo "  cross-compile - Build for darwin/amd64, darwin/arm64, linux/amd64"
	@echo "  clean         - Remove build artifacts"

build:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)

test:
	go test -json -race -cover -coverprofile=coverage.out ./... | tparse -progress

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not found, installing..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	}
	golangci-lint run ./...

cross-compile:
	@mkdir -p $(BUILD_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d'/' -f1); \
		GOARCH=$$(echo $$platform | cut -d'/' -f2); \
		output="$(BUILD_DIR)/$(BINARY_NAME)-$$GOOS-$$GOARCH"; \
		echo "Building $$output..."; \
		GOOS=$$GOOS GOARCH=$$GOARCH go build $(LDFLAGS) -o $$output $(CMD_PATH); \
	done

clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out
