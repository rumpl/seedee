.PHONY: all build test lint proto clean

all: proto build test lint

build:
	@mkdir -p bin
	go build -o bin/seedee ./cmd/seedee
	go build -o bin/seedeed ./cmd/seedeed

test:
	go test ./...

lint:
	@echo "golangci-lint not yet configured"

proto:
	@echo "buf generate not yet configured"

clean:
	rm -rf bin/
	find gen/ -type f ! -name '.gitkeep' -delete 2>/dev/null || true
