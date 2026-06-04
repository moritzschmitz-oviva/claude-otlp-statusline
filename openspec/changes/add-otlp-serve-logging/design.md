## Context

`claude-otlp serve` receives OTLP log payloads from Claude Code, parses `api_request` events, inserts them into SQLite, and optionally forwards the raw body to GCP. The ingest path (`logsHandler` → `otlp.ParseApiRequests` → `db.Insert` → `forward`) runs without emitting any per-event log lines. Parse failures, empty-field events, `INSERT OR IGNORE` duplicate skips, and GCP forward errors all happen silently. The only observable signal is the startup line.

`log.Printf` (stdlib) is already used throughout `main.go`.

## Goals / Non-Goals

**Goals:**
- Emit DEBUG-level lines covering: request received (method, size), events parsed (count), per-event fields (`session_id`, `request_id`, `cost_usd`, `model`), WARN on empty `session_id`/`request_id`, INSERT skip on duplicate, GCP forward HTTP status + latency.
- Gate verbosity with `CLAUDE_OTLP_LOG_LEVEL` env var (`debug`/`info`/`warn`, default `info`).
- Expose whether `db.Insert` actually wrote a row (vs skipped) so the caller can log it.

**Non-Goals:**
- Structured JSON logging (slog) — too much churn for a local debugging tool; stdlib `log` is fine.
- Log rotation or file-sink configuration.
- Metrics or traces.
- Changes to `status` or `set-spent` commands.

## Decisions

### Use stdlib `log` with a level wrapper, not `log/slog`

`log/slog` requires Go 1.21+ and a non-trivial API change across the file. This is a local daemon tool; human-readable flat lines are sufficient for debugging. A thin `logDebug` / `logWarn` helper that checks a package-level `logLevel` var is enough and keeps the diff minimal.

Alternatives considered:
- `log/slog` — cleaner structured output but more invasive; out of scope.
- Third-party (`zap`, `zerolog`) — no new deps wanted.

### `db.Insert` returns `(bool, error)` — `true` means row was written

`INSERT OR IGNORE` currently returns no row-count signal. Changing the return to `(inserted bool, err error)` — derived from `sql.Result.RowsAffected()` — lets the caller log duplicates. No other callers of `Insert` exist so the signature change is safe.

### `forward` receives and logs HTTP status + latency

Currently `forward` closes the response body and only logs on error. Change: capture `time.Since(start)` and log `DEBUG forward %s: %d %s (%dms)` unconditionally, `WARN` when status ≥ 400.

### `CLAUDE_OTLP_LOG_LEVEL` env var, parsed at startup

Parsed once in `serveCmd.RunE` before starting the server. Values: `debug`, `info` (default), `warn`. Stored as an `int` constant for O(1) level checks. No config file changes — this is a runtime-only setting.

## Risks / Trade-offs

- **Log volume at DEBUG** — every API call from Claude Code emits ~5 lines. At heavy usage (many tool calls/min) this grows the `/tmp/claude-otlp.log` file quickly.
  → Mitigation: default is `info`, so DEBUG lines are opt-in. Document that `CLAUDE_OTLP_LOG_LEVEL=debug` is for transient diagnosis only.

- **`RowsAffected` on SQLite** — `modernc.org/sqlite` returns correct `RowsAffected` for `INSERT OR IGNORE` (0 on skip, 1 on insert). This is a known behavior; tested against the same driver already in use.
  → Mitigation: none needed; behavior is specified.

## Migration Plan

1. Build and install: `go build -o ~/.local/bin/claude-otlp ./cmd/claude-otlp`.
2. Restart daemon: `kill $(pgrep claude-otlp); ~/.local/bin/claude-otlp serve`.
3. To enable DEBUG: `kill $(pgrep claude-otlp); CLAUDE_OTLP_LOG_LEVEL=debug ~/.local/bin/claude-otlp serve`.
4. No SQLite schema changes; no data migration needed.
5. Rollback: revert binary to previous build.
