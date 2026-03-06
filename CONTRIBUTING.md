# Contributing to seedee

## Development Setup

### Prerequisites

- **Go 1.22+** — [install](https://go.dev/dl/)
- **Docker** — required for running pipelines and integration tests
- **buf** — for protobuf code generation ([install](https://buf.build/docs/installation))
- **golangci-lint** — for linting ([install](https://golangci-lint.run/welcome/install/))

### Clone and build

```bash
git clone https://github.com/rumpl/seedee.git
cd seedee
make build
```

Binaries are placed in `bin/`.

## Building

```bash
make build
```

This builds two binaries:

- `bin/seedee` — the CLI client
- `bin/seedeed` — the server

Version information is injected via linker flags automatically.

## Testing

### Unit tests

```bash
make test
```

Runs all unit tests across the project (`go test ./...`).

### End-to-end tests (local mode)

```bash
make test-e2e-local
```

Requires a running Docker daemon. Runs the full pipeline locally.

### End-to-end tests (remote mode)

```bash
make test-e2e
```

Builds the server, starts it, and runs pipelines against it.

### All tests

```bash
make test-all
```

Runs unit tests, lint, and local end-to-end tests.

## Linting

```bash
make lint
```

Uses golangci-lint with the configuration in `.golangci.yml`. To auto-fix
issues where possible:

```bash
make lint-fix
```

## Protobuf Changes

The API is defined in `proto/seedee/v1/seedee.proto`. After editing:

```bash
make proto
```

This regenerates Go code in `gen/` using `buf generate`. To lint the proto
files:

```bash
make proto-lint
```

> **Do not edit files in `gen/` manually.** They are overwritten on each
> `make proto` run.

## Code Organization

```
cmd/seedee/        CLI entry point
cmd/seedeed/       Server entry point
internal/cli/      CLI commands, terminal UI, client-side logic
internal/core/     Pipeline model, config, scheduler, engine, events
internal/runner/   Runner interface and Docker implementation
internal/server/   ConnectRPC handlers, server, proto conversion
gen/               Generated protobuf + ConnectRPC code (do not edit)
proto/             Protobuf source files
test/              End-to-end tests
testdata/          Sample configs for tests
```

See [docs/architecture.md](docs/architecture.md) for a detailed design overview.

## PR Process

1. Fork the repository and create a feature branch from `main`.
2. Make your changes. Follow existing code style.
3. Add or update tests for any new behavior.
4. Run `make test-all` (or at minimum `make test` and `make lint`).
5. Open a pull request against `main`.
6. Describe what changed and why in the PR description.

### Commit messages

- Use clear, concise commit messages.
- Reference issue numbers where applicable (e.g., `Fixes #42`).
