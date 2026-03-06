# Configuration Reference

seedee pipelines are defined in a YAML file (`.seedee.yml` by default).

## File Structure

```yaml
pipeline:
  name: <string>               # required
  env:                          # optional
    KEY: "value"
  jobs:                         # required, at least one job
    <job-name>:
      image: <string>           # required
      depends_on: [<job-name>]  # optional
      env:                      # optional
        KEY: "value"
      steps:                    # required, at least one step
        - name: <string>        # optional
          run: <string>         # required
          env:                  # optional
            KEY: "value"
```

## Pipeline-Level Fields

| Field    | Type                | Required | Description                                    |
|----------|---------------------|----------|------------------------------------------------|
| `name`   | string              | yes      | Human-readable name for the pipeline.          |
| `env`    | map\[string\]string | no       | Environment variables inherited by all jobs.   |
| `jobs`   | map\[string\]JobDef | yes      | Jobs to execute, keyed by unique job name.     |

## Job-Level Fields

| Field        | Type                | Required | Description                                         |
|--------------|---------------------|----------|-----------------------------------------------------|
| `image`      | string              | yes      | Docker image used to run the job's steps.           |
| `depends_on` | list\[string\]      | no       | Names of jobs that must succeed before this one.    |
| `env`        | map\[string\]string | no       | Environment variables for this job.                 |
| `steps`      | list\[StepDef\]     | yes      | Steps to execute sequentially inside the container. |

## Step-Level Fields

| Field  | Type                | Required | Description                             |
|--------|---------------------|----------|-----------------------------------------|
| `name` | string              | no       | Human-readable label for the step.      |
| `run`  | string              | yes      | Shell command executed via `sh -c`.     |
| `env`  | map\[string\]string | no       | Environment variables for this step.    |

## Environment Variable Merging

Environment variables are merged at three levels. More specific levels override
less specific ones:

1. **Pipeline `env`** — base layer, available to every job and step.
2. **Job `env`** — merged on top of pipeline `env`. Job values win on conflict.
3. **Step `env`** — merged on top of job `env`. Step values win on conflict.

Example:

```yaml
pipeline:
  name: merge-demo
  env:
    GO_VERSION: "1.22"
    CGO_ENABLED: "1"
  jobs:
    build:
      image: golang:1.22
      env:
        CGO_ENABLED: "0"          # overrides pipeline-level value
      steps:
        - name: Check env
          run: echo $GO_VERSION $CGO_ENABLED $GOOS
          env:
            GOOS: linux            # only available in this step
```

In this example, the `Check env` step sees:
- `GO_VERSION=1.22` (from pipeline)
- `CGO_ENABLED=0` (job overrides pipeline)
- `GOOS=linux` (from step)

## Dependency Rules and Parallel Execution

- Jobs **without** `depends_on` (and whose dependencies are all satisfied) run
  **in parallel**.
- Jobs **with** `depends_on` wait until every listed job succeeds.
- If a dependency **fails**, all downstream jobs are **skipped**.
- Circular dependencies are detected at config validation time and produce a
  clear error message.

seedee uses Kahn's algorithm to sort jobs into execution groups. All jobs in
the same group run concurrently; groups execute sequentially.

## Examples

### Simple — single job

```yaml
pipeline:
  name: simple
  jobs:
    test:
      image: golang:1.22
      steps:
        - name: Run tests
          run: go test ./...
```

### Parallel — independent jobs

```yaml
pipeline:
  name: parallel
  jobs:
    lint:
      image: golangci/golangci-lint:latest
      steps:
        - name: Lint
          run: golangci-lint run ./...
    test:
      image: golang:1.22
      steps:
        - name: Test
          run: go test -race ./...
```

`lint` and `test` run at the same time because neither depends on the other.

### Diamond dependencies

```yaml
pipeline:
  name: diamond
  jobs:
    build:
      image: golang:1.22
      steps:
        - name: Build
          run: go build -o /workspace/bin/app ./cmd/app
    unit-test:
      image: golang:1.22
      depends_on: [build]
      steps:
        - name: Unit tests
          run: go test ./...
    integration-test:
      image: golang:1.22
      depends_on: [build]
      steps:
        - name: Integration tests
          run: go test -tags=integration ./...
    deploy:
      image: alpine:latest
      depends_on: [unit-test, integration-test]
      steps:
        - name: Deploy
          run: echo "Deploying..."
```

Execution order:
1. `build` runs first.
2. `unit-test` and `integration-test` run in parallel.
3. `deploy` runs after both test jobs succeed.

### Multi-step job

```yaml
pipeline:
  name: multi-step
  jobs:
    build-and-push:
      image: golang:1.22
      env:
        CGO_ENABLED: "0"
      steps:
        - name: Download dependencies
          run: go mod download
        - name: Build binary
          run: go build -o /workspace/bin/app ./cmd/app
        - name: Verify binary
          run: /workspace/bin/app --version
```

Steps within a job execute sequentially. If any step fails (non-zero exit code),
the remaining steps are skipped and the job is marked as failed.

## Error Messages

| Error | Cause | Fix |
|-------|-------|-----|
| `pipeline name is required` | Missing `name` field. | Add `name:` under `pipeline:`. |
| `must have at least one job` | Empty `jobs` map. | Add at least one job. |
| `job "X" has no image` | Job missing `image`. | Set `image:` on the job. |
| `job "X" has no steps` | Job has an empty `steps` list. | Add at least one step with a `run:` command. |
| `step N in job "X" has no run command` | Step missing `run`. | Add `run:` to the step. |
| `duplicate step name "Y" in job "X"` | Two steps share the same name. | Use unique step names within a job. |
| `job "X" depends on "Y" which does not exist` | Typo or missing job. | Check `depends_on` references. |
| `dependency cycle detected: A -> B -> A` | Circular dependency chain. | Remove one edge in the cycle. |
