# CLI Reference

## Global Flags

These flags are available on all commands:

| Flag              | Short | Default        | Description                                      |
|-------------------|-------|----------------|--------------------------------------------------|
| `--server`        |       | *(empty)*      | Remote seedee server address (e.g., `host:8080`).|
| `--config`        | `-c`  | `.seedee.yml`  | Path to pipeline config file.                    |
| `--verbose`       | `-v`  | `false`        | Enable verbose output.                           |

---

## `seedee run`

Run the pipeline defined in `.seedee.yml` (or the file specified by `--config`).

**Usage:**

```bash
seedee run [flags]
```

**Behavior:**

- Without `--server`: runs the pipeline **locally** using Docker. Requires a
  running Docker daemon.
- With `--server`: sends the pipeline to the remote seedeed server and streams
  events back in real-time.

**Examples:**

```bash
# Run locally with default config
seedee run

# Run with a custom config file
seedee run --config ci/pipeline.yml

# Run against a remote server
seedee run --server ci.example.com:8080

# Run with verbose output
seedee run -v
```

**Output:**

The terminal displays a live view of pipeline execution:

```
▶ Pipeline "my-project" started

  ▶ lint
    [lint/Lint] Running golangci-lint...
    ✓ lint/Lint (2.3s)
  ✓ lint (2.5s)

  ▶ test
    [test/Test] ok  ./... 1.234s
    ✓ test/Test (1.5s)
  ✓ test (1.8s)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Pipeline: abc-123-def
Status:   ✓ success
Duration: 2.5s

  ✓ lint                 2.5s
  ✓ test                 1.8s
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

**Exit codes:**

| Code | Meaning                     |
|------|-----------------------------|
| 0    | Pipeline succeeded.         |
| 1    | Pipeline failed or canceled.|

---

## `seedee status [pipeline-id]`

Get the status of a pipeline run on the remote server.

**Usage:**

```bash
seedee status <pipeline-id> --server <addr>
```

Requires `--server`.

**Example:**

```bash
seedee status abc-123-def --server ci.example.com:8080
```

**Output:**

```
✓ Pipeline: my-project (abc-123-def)
  Status:   STATUS_SUCCESS
  Duration: 12.5s

  ✓ Job: build
      Duration: 5.2s
      ✓ Step: Download dependencies
          Duration: 2.1s
      ✓ Step: Build binary
          Duration: 3.1s
  ✓ Job: test
      Duration: 7.3s
      ✓ Step: Run unit tests
          Duration: 4.2s
      ✓ Step: Run integration tests
          Duration: 3.1s
```

**Status icons:**

| Icon | Meaning  |
|------|----------|
| ✓    | Success  |
| ✗    | Failed   |
| ●    | Running  |
| ○    | Pending  |
| ⊘    | Skipped  |
| ⊗    | Canceled |

---

## `seedee cancel [pipeline-id]`

Cancel a running pipeline on the remote server.

**Usage:**

```bash
seedee cancel <pipeline-id> --server <addr>
```

Requires `--server`.

**Example:**

```bash
seedee cancel abc-123-def --server ci.example.com:8080
# pipeline "abc-123-def" cancellation requested
```

---

## `seedee version`

Print version information.

**Usage:**

```bash
seedee version
```

**Output:**

```
seedee v0.1.0 (commit: abc1234, built: 2024-01-15T10:00:00Z)
```

The version, commit hash, and build date are injected at build time via
linker flags. See the `Makefile` for details.
