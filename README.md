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

## Flag Controls

`config/flags.json` is a restricted, flagd-compatible static configuration for the local lab.
The OpenFeature-backed evaluator reads it at every synthetic action boundary and records a
deterministic `sha256:` snapshot of the evaluated values. It controls the global halt, individual
tool halts, memory writes, sub-agent spawning, maximum autonomy, and the model route.

Targeting is stable for each `tenant:run` key: 5% of keys enter the canary cohort, the next 10%
the shadow cohort, and the remainder the control cohort. The local provider is deliberately
provider-neutral so a flagd provider can replace it without changing the enforcement boundary.

## Policy Controls

`internal/policy/incident.rego` and its version file are embedded in the binary and evaluated with
OPA. The policy permits only the synthetic incident tools, requires verified citations for writes,
requires approval for `service.restart`, enforces autonomy and sub-agent limits, and blocks any
untrusted instruction from authorizing a tool call. Every policy decision carries the embedded
policy's `sha256:` digest, version, and evaluated flag snapshot. A missing or invalid policy blocks
the action rather than yielding an allow decision.

## Enforcement And Audit

`internal/enforce` is the only exported route to the synthetic `metrics.read`, `ticket.update`,
`service.restart`, and `subagent.spawn` adapters. It re-evaluates flags and policy, applies budget
limits, appends a sanitized decision to SQLite, and only then invokes an adapter for `ALLOW`.
`SHADOW`, `BLOCK`, `REQUIRE_APPROVAL`, and `HALT_RUN` decisions are recorded without a side effect.
Duplicate decision IDs are rejected before a second adapter invocation.

Read a persisted event with:

```sh
go run ./cmd/abcp ledger get --db abcp-audit.db DECISION_ID
```

## Direction

The completed lab will route every synthetic incident-response tool proposal through one
enforcement interface. Emergency controls and rollout evaluation will be introduced in subsequent
tasks. High-risk actions fail closed when a required control is unavailable.
