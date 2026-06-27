.PHONY: help build test test-race test-cover lint fmt clean install

# Default target
help:
	@echo "Regent Development Commands"
	@echo ""
	@echo "  make build      - Build rgt binary"
	@echo "  make test       - Run all tests"
	@echo "  make test-race  - Run tests with race detector"
	@echo "  make test-cover - Run tests with coverage report"
	@echo "  make lint       - Run golangci-lint"
	@echo "  make fmt        - Format code with gofmt"
	@echo "  make clean      - Remove build artifacts"
	@echo "  make install    - Install rgt to GOPATH/bin"
	@echo ""

# Version stamping (mirrors .goreleaser.yaml ldflags). Falls back gracefully
# outside a git checkout.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/regent-vcs/regent/internal/cli.Version=$(VERSION) \
           -X github.com/regent-vcs/regent/internal/cli.Commit=$(COMMIT) \
           -X github.com/regent-vcs/regent/internal/cli.Date=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o rgt ./cmd/rgt

test:
	go test ./...

test-race:
	go test -race ./...

test-cover:
	go test -cover -coverprofile=coverage.txt ./...
	go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report: coverage.html"

lint:
	golangci-lint run

fmt:
	gofmt -w .

clean:
	rm -f rgt
	rm -f coverage.txt coverage.html
	rm -rf dist/

install:
	go install ./cmd/rgt
