.PHONY: all build test lint lint-fix proto clean

all: proto build test lint

build:
	@mkdir -p bin
	go build -o bin/seedee ./cmd/seedee
	go build -o bin/seedeed ./cmd/seedeed

test:
	go test ./...

lint:
	golangci-lint run ./...

.PHONY: lint-fix
lint-fix:
	golangci-lint run --fix ./...

proto:
	@echo "buf generate not yet configured"

clean:
	rm -rf bin/
	find gen/ -type f ! -name '.gitkeep' -delete 2>/dev/null || true
