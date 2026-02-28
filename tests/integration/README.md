# Integration tests

End-to-end tests that run the real `devx` binary against example projects.

## Requirements

- Docker running locally
- `devx` binary on `PATH` (or set `DEVX_BIN=/path/to/devx`)

## Running

```sh
# All integration tests
go test ./tests/integration/... -tags integration -v

# Single test
go test ./tests/integration/... -tags integration -v -run TestDevxUp_Basic

# Skip slow tests (those that actually start containers)
go test ./tests/integration/... -tags integration -v -short
```

## Build tag

All files in this package use `//go:build integration` so they are excluded
from the standard `go test ./...` run and only execute when explicitly opted in.
