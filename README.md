# Agent Behavior Control Plane

A local, synthetic Go lab for enforcing feature flags, policy decisions, kill switches, rollout
cohorts, and audit records at agent tool-call boundaries. It does not connect to production tools,
model providers, or external services.

## Development

Requires Go 1.25 or newer.

```sh
go test ./...
go vet ./...
go build ./cmd/abcp
go run ./cmd/abcp --help
go run ./cmd/abcp serve --addr 127.0.0.1:8080
```

The initial health endpoint is available at `GET /healthz`.

## Direction

The completed lab will route every synthetic incident-response tool proposal through one
enforcement interface. Flag evaluation, policy decision, budget checks, and append-only audit
records will be introduced in subsequent tasks. High-risk actions will fail closed when a required
control is unavailable.
