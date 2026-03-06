.PHONY: all build test lint lint-fix proto proto-frontend clean test-e2e test-e2e-remote test-e2e-local test-all \
	frontend-install frontend-dev frontend-build

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
	rm -rf gen/
	buf generate
	$(MAKE) proto-frontend

.PHONY: proto-frontend
proto-frontend: frontend-install
	rm -rf frontend/src/gen/
	buf generate proto --template buf.gen.frontend.yaml

.PHONY: proto-lint
proto-lint:
	buf lint

clean:
	rm -rf bin/
	find gen/ -type f ! -name '.gitkeep' -delete 2>/dev/null || true
	rm -rf frontend/src/gen/

.PHONY: test-e2e-remote
test-e2e-remote: build
	go test -tags=e2e -v -timeout=5m ./test/ -run TestE2E_Remote

.PHONY: test-e2e
test-e2e: test-e2e-remote

.PHONY: test-e2e-local
test-e2e-local: build
	go test -tags=e2e -v -timeout=5m ./test/ -run TestE2E_Local

.PHONY: test-all
test-all: test lint test-e2e-local

# Frontend targets
.PHONY: frontend-install
frontend-install:
	cd frontend && pnpm install

.PHONY: frontend-dev
frontend-dev:
	cd frontend && pnpm run dev

.PHONY: frontend-build
frontend-build:
	cd frontend && pnpm run build
