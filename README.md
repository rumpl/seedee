# seedee

A CI system in Go. Run pipelines locally or on a remote server.

## Quick Start

### Install

```bash
go install github.com/rumpl/seedee/cmd/seedee@latest
```

### Define a pipeline

Create `.seedee.yml` in your project root:

```yaml
pipeline:
  name: my-project
  jobs:
    build:
      image: golang:1.22
      steps:
        - name: Build
          run: go build ./...
    test:
      image: golang:1.22
      depends_on: [build]
      steps:
        - name: Test
          run: go test ./...
```

### Run locally

```bash
seedee run
```

### Run on a remote server

```bash
# Start the server
seedeed --addr :8080

# Run from another machine
seedee run --server myserver:8080
```

## Features

- **Parallel job execution** — jobs without dependencies run concurrently
- **Docker-based isolation** — each job runs in its own container
- **Dependency DAG** — define job dependencies, seedee handles ordering
- **Local & remote modes** — run locally for development, remotely for CI
- **Real-time streaming** — logs stream to your terminal as they happen
- **ConnectRPC API** — modern gRPC-compatible API

## Documentation

- [Configuration Reference](docs/config.md)
- [Architecture](docs/architecture.md)
- [Deployment Guide](docs/deployment.md)
- [CLI Reference](docs/cli.md)
- [Contributing](CONTRIBUTING.md)
