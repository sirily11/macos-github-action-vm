SHELL := /bin/bash

BINARY_NAME := rvmm

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -s -w \
	-X github.com/rxtech-lab/rvmm/cmd.Version=$(VERSION) \
	-X github.com/rxtech-lab/rvmm/cmd.Commit=$(COMMIT) \
	-X github.com/rxtech-lab/rvmm/cmd.BuildDate=$(BUILD_DATE)

.PHONY: deps build build-all install clean test

deps:
	@echo "==> Fetching dependencies..."
	go mod tidy
	go mod download
	@echo "==> Dependencies fetched"

build: deps
	@echo "==> Building $(BINARY_NAME)..."
	@echo "    Version: $(VERSION)"
	@echo "    Commit:  $(COMMIT)"
	@echo "    Date:    $(BUILD_DATE)"
	go build -ldflags "$(LDFLAGS)" -o "$(BINARY_NAME)" .
	@echo "==> Built: $(BINARY_NAME)"

build-all: deps
	@echo "==> Building for all platforms..."
	@echo "    Building darwin/arm64..."
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o "$(BINARY_NAME)-darwin-arm64" .
	@echo "    Building darwin/amd64..."
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o "$(BINARY_NAME)-darwin-amd64" .
	@echo "==> Built binaries:"
	ls -la "$(BINARY_NAME)"-darwin-*

install: deps
	$(MAKE) build
	@echo "==> Installing to /usr/local/bin..."
	sudo cp "$(BINARY_NAME)" /usr/local/bin/
	@echo "==> Installed: /usr/local/bin/$(BINARY_NAME)"

clean:
	@echo "==> Cleaning..."
	rm -f "$(BINARY_NAME)" "$(BINARY_NAME)"-darwin-*
	@echo "==> Cleaned"

test:
	@echo "==> Running tests..."
	go test -v ./...
