#!/bin/bash
set -e

# Build script for RVMM CLI
# Usage: ./build.sh [command]
# Commands: deps, build, build-all, install, clean, test

# Use absolute path
PROJECT_DIR="/Users/qiweili/Desktop/github-self-hosted-vm-macos"

BINARY_NAME="rvmm"
VERSION="${VERSION:-$(git -C "$PROJECT_DIR" describe --tags --always --dirty 2>/dev/null || echo "dev")}"
COMMIT="${COMMIT:-$(git -C "$PROJECT_DIR" rev-parse --short HEAD 2>/dev/null || echo "none")}"
BUILD_DATE="${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"

LDFLAGS="-s -w \
    -X github.com/rxtech-lab/rvmm/cmd.Version=${VERSION} \
    -X github.com/rxtech-lab/rvmm/cmd.Commit=${COMMIT} \
    -X github.com/rxtech-lab/rvmm/cmd.BuildDate=${BUILD_DATE}"

deps() {
    echo "==> Fetching dependencies..."
    cd "$PROJECT_DIR" && go mod tidy && go mod download
    echo "==> Dependencies fetched"
}

build() {
    echo "==> Building ${BINARY_NAME}..."
    echo "    Version: ${VERSION}"
    echo "    Commit:  ${COMMIT}"
    echo "    Date:    ${BUILD_DATE}"
    cd "$PROJECT_DIR" && go build -ldflags "${LDFLAGS}" -o "${BINARY_NAME}" .
    echo "==> Built: ${PROJECT_DIR}/${BINARY_NAME}"
    "${PROJECT_DIR}/${BINARY_NAME}" version
}

build_all() {
    echo "==> Building for all platforms..."
    cd "$PROJECT_DIR"

    echo "    Building darwin/arm64..."
    GOOS=darwin GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o "${BINARY_NAME}-darwin-arm64" .

    echo "    Building darwin/amd64..."
    GOOS=darwin GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o "${BINARY_NAME}-darwin-amd64" .

    echo "==> Built binaries:"
    ls -la "${PROJECT_DIR}/${BINARY_NAME}"-darwin-*
}

install_binary() {
    build
    echo "==> Installing to /usr/local/bin..."
    sudo cp "${PROJECT_DIR}/${BINARY_NAME}" /usr/local/bin/
    echo "==> Installed: /usr/local/bin/${BINARY_NAME}"
}

clean() {
    echo "==> Cleaning..."
    rm -f "${PROJECT_DIR}/${BINARY_NAME}" "${PROJECT_DIR}/${BINARY_NAME}"-darwin-*
    echo "==> Cleaned"
}

run_tests() {
    echo "==> Running tests..."
    cd "$PROJECT_DIR" && go test -v ./...
}

case "${1:-build}" in
    deps)
        deps
        ;;
    build)
        deps
        build
        ;;
    build-all)
        deps
        build_all
        ;;
    install)
        deps
        install_binary
        ;;
    clean)
        clean
        ;;
    test)
        run_tests
        ;;
    *)
        echo "Usage: $0 {deps|build|build-all|install|clean|test}"
        exit 1
        ;;
esac
