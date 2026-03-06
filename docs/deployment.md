# Deployment Guide

This guide covers running seedeed (the seedee server) in production.

## Building

```bash
# Build both binaries
make build

# Binaries are placed in bin/
ls bin/
# seedee   seedeed
```

Or install directly:

```bash
go install github.com/rumpl/seedee/cmd/seedeed@latest
```

## Running seedeed as a systemd Service

Create `/etc/systemd/system/seedeed.service`:

```ini
[Unit]
Description=seedee CI server
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/seedeed --addr :8080
Restart=on-failure
RestartSec=5
Environment=SEEDEE_LOG_LEVEL=info

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now seedeed
sudo journalctl -u seedeed -f
```

## Docker Deployment

```bash
# Build the image (example Dockerfile)
docker build -t seedeed .

# Run with access to the host Docker socket
docker run -d \
  --name seedeed \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  seedeed --addr :8080
```

> **Note:** seedeed needs access to a Docker daemon to run pipeline jobs.
> Mounting the host Docker socket is the simplest approach. In production,
> consider a dedicated Docker-in-Docker sidecar or a remote Docker host.

## Environment Variables

| Variable            | Default  | Description                                     |
|---------------------|----------|-------------------------------------------------|
| `SEEDEE_ADDR`       | `:8080`  | Listen address (overrides `--addr` flag).       |
| `SEEDEE_LOG_LEVEL`  | `info`   | Log level: `debug`, `info`, `warn`, `error`.   |

Environment variables override command-line flags.

## Command-Line Flags

| Flag               | Default    | Description                                        |
|--------------------|------------|----------------------------------------------------|
| `--addr`           | `:8080`    | Listen address (e.g., `:8080`, `0.0.0.0:9090`).  |
| `--log-level`      | `info`     | Minimum log level.                                 |
| `--prune-interval` | `5m`       | How often to prune old completed pipeline runs.    |
| `--prune-max-age`  | `1h`       | Maximum age of completed runs before pruning.      |

## Reverse Proxy Setup

### nginx

```nginx
upstream seedeed {
    server 127.0.0.1:8080;
}

server {
    listen 443 ssl http2;
    server_name ci.example.com;

    ssl_certificate     /etc/ssl/certs/ci.example.com.pem;
    ssl_certificate_key /etc/ssl/private/ci.example.com.key;

    location / {
        grpc_pass grpc://seedeed;

        # Required for streaming
        proxy_buffering off;
        proxy_request_buffering off;
    }
}
```

### Caddy

```
ci.example.com {
    reverse_proxy h2c://127.0.0.1:8080
}
```

Caddy automatically handles TLS via Let's Encrypt and supports HTTP/2 upstream.

## Health Check

seedeed exposes a health check endpoint:

```bash
curl http://localhost:8080/healthz
# ok
```

Use this in Docker health checks, load balancer probes, or Kubernetes
readiness/liveness checks.

Example Docker health check:

```dockerfile
HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD curl -f http://localhost:8080/healthz || exit 1
```

## Monitoring and Logging

seedeed logs to stderr using Go's `slog` structured logger. Each log line
includes key-value pairs:

```
time=2024-01-15T10:30:00.000Z level=INFO msg="starting server" addr=:8080
time=2024-01-15T10:30:05.123Z level=INFO msg="pipeline completed" id=abc-123 status=success duration=12.5s
```

Set `--log-level debug` (or `SEEDEE_LOG_LEVEL=debug`) for verbose output during
troubleshooting.

To integrate with external systems, pipe stderr to a log collector:

```bash
seedeed --addr :8080 2>&1 | your-log-collector
```

### Pipeline Run Pruning

Completed pipeline runs are kept in memory for status queries. The server
automatically prunes runs older than `--prune-max-age` (default: 1 hour) every
`--prune-interval` (default: 5 minutes). Adjust these values based on your
query patterns and memory constraints.
