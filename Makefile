.PHONY: all build test lint lint-fix proto clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X github.com/rumpl/seedee/internal/cli.Version=$(VERSION) \
                      -X github.com/rumpl/seedee/internal/cli.GitCommit=$(COMMIT) \
                      -X github.com/rumpl/seedee/internal/cli.BuildDate=$(DATE)"

all: proto build test lint

build:
	@mkdir -p bin
	go build $(LDFLAGS) -o bin/seedee ./cmd/seedee
	go build -o bin/seedeed ./cmd/seedeed

test:
	go test ./...

lint:
	golangci-lint run ./...

.PHONY: lint-fix
lint-fix:
	golangci-lint run --fix ./...

proto:
	buf generate

.PHONY: proto-lint
proto-lint:
	buf lint

clean:
	rm -rf bin/
	find gen/ -type f ! -name '.gitkeep' -delete 2>/dev/null || true
