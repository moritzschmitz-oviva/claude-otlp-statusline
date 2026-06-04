## Why

The `serve` daemon ingests OTLP events silently — no per-request or per-event log lines — making it impossible to tell whether Claude Code is sending events, whether parsing succeeds, whether events have populated fields, or whether GCP forwarding is succeeding. When sessions don't appear in BigQuery the only signal is absence, with no log evidence to distinguish "nothing sent", "parse failed", "empty session_id", "INSERT OR IGNORE skipped duplicate", or "forward returned 4xx/5xx".

## What Changes

- `logsHandler` logs incoming request count and body size at DEBUG level.
- After parsing, logs count of extracted `api_request` events and — for each event — `session_id`, `request_id`, `cost_usd`, and `model` at DEBUG level.
- Logs a WARN when an event has an empty `session_id` or `request_id` (these silently produce orphaned or duplicate-safe-skipped rows today).
- `db.Insert` returns a boolean indicating whether the row was actually inserted (vs skipped by `INSERT OR IGNORE`); `logsHandler` logs a DEBUG line when a request_id is skipped as duplicate.
- `forward` logs the HTTP response status and latency at DEBUG level (currently only logs on error or 4xx+).
- Log verbosity controlled by `CLAUDE_OTLP_LOG_LEVEL` env var (`debug` / `info` / `warn`; default `info`). At `info`, only warn-level lines appear. At `debug`, every event is traced.

## Capabilities

### New Capabilities
- `structured-logging`: Structured, level-gated log output for the `serve` daemon covering ingest, parse, store, and forward paths.

### Modified Capabilities

## Impact

- `cmd/claude-otlp/main.go` — `logsHandler`, `forward` functions
- `internal/store/db.go` — `Insert` return signature (bool added)
- No schema changes, no new dependencies (use stdlib `log` or promote to `log/slog`)
