# Safety Boundaries And Runbook

## Architecture

Every synthetic action crosses flags, deterministic policy, budget checks, append-only audit, and
only then an internal adapter. The model is deliberately outside this authority chain: an LLM may
propose an action or an optional rubric may produce an evaluation signal, but neither can override
a feature halt, policy `BLOCK`, `REQUIRE_APPROVAL`, or `HALT_RUN` decision.

The audit record contains only role, tool, risk, verified-evidence status, decision metadata, and
correlation identifiers. It intentionally omits resource strings and raw citations.

## Threat Model

Untrusted instructions are blocked before tool authorization. Missing flag data, missing policy
context, policy compilation/evaluation failure, ledger unavailability, duplicate decision IDs, and
expired or malformed emergency controls fail closed for side effects. Shadow decisions are audited
but never invoke a tool adapter. This local lab has no production credentials, real service calls,
or model provider integration.

## Incident Demo

Run `make demo`. It emits JSON proving that an evidence-less restart is `BLOCK` and that a global
halt applied mid-run becomes `HALT_RUN` at the next boundary. Run `make benchmark` to measure
local package benchmarks; emergency propagation target is 500ms.

## Operator Runbook

Start `go run ./cmd/abcp serve`, then use `abcp control set`, `list`, and `clear`. Every set needs
an owner, reason, and expiry. Prefer the narrowest tool halt first, confirm the next boundary
observes it, and retain the returned control ID for explicit clearance. Review the sanitized SQLite
decision record with `abcp ledger get`.
