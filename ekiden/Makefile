# Build configuration
# -------------------

BINARY_NAME = ekiden
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS = -ldflags "-s -w \
	-X github.com/rxtech-lab/ekiden-cli/cmd.Version=$(VERSION) \
	-X github.com/rxtech-lab/ekiden-cli/cmd.Commit=$(COMMIT) \
	-X github.com/rxtech-lab/ekiden-cli/cmd.BuildDate=$(BUILD_DATE)"

# Linter and formatter configuration
# ----------------------------------

PRETTIER_FILES_PATTERN = './*.md' './*/*.md'
SHFMT_FILES_PATTERN = ./**/*.sh

# Introspection targets
# ---------------------

.PHONY: help
help: targets

.PHONY: targets
targets:
	@echo "\033[34mTargets\033[0m"
	@echo "\033[34m---------------------------------------------------------------\033[0m"
	@perl -nle'print $& if m{^[a-zA-Z_-\d]+:.*?## .*$$}' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-22s\033[0m %s\n", $$1, $$2}'

# Check, lint and format targets
# ------------------------------

.PHONY: lint
lint: ## Lint files
	npx prettier --check $(PRETTIER_FILES_PATTERN)
	shfmt -d $(SHFMT_FILES_PATTERN)
	shellcheck $(SHFMT_FILES_PATTERN)

.PHONY: format
format: ## Format source files
	npx prettier --write $(PRETTIER_FILES_PATTERN)
	shfmt -w $(SHFMT_FILES_PATTERN)

# Go build targets
# ----------------

.PHONY: build
build: ## Build the CLI binary
	go build $(LDFLAGS) -o $(BINARY_NAME) .

.PHONY: build-all
build-all: ## Build for all platforms (macOS arm64/amd64)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 .
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 .

.PHONY: install
install: build ## Install to /usr/local/bin
	sudo cp $(BINARY_NAME) /usr/local/bin/

.PHONY: clean
clean: ## Remove build artifacts
	rm -f $(BINARY_NAME) $(BINARY_NAME)-darwin-*

.PHONY: deps
deps: ## Download dependencies
	go mod tidy
	go mod download

.PHONY: test
test: ## Run tests
	go test -v ./...

.PHONY: run
run: build ## Build and run with sample config
	./$(BINARY_NAME) --help
