## 1. Go module + project scaffold

- [x] 1.1 `go mod init github.com/moritzschmitz-oviva/claude-otlp-statusline`
- [x] 1.2 Add dependencies: `go.opentelemetry.io/proto/otlp` (OTLP protobuf), `modernc.org/sqlite` (CGo-free SQLite driver), `cobra` (CLI)
- [x] 1.3 Add `cmd/claude-otlp/main.go` with `serve` and `status` subcommands (stubs)

## 2. OTLP HTTP receiver (`serve`)

- [x] 2.1 Accept OTLP HTTP JSON at `POST /v1/logs` on `localhost:4318`
- [x] 2.2 Parse `ExportLogsServiceRequest`; extract `api_request` log records only (filter on `textPayload = "claude_code.api_request"` or resource job label)
- [x] 2.3 Parse `labels` struct from log record attributes → Go struct
- [x] 2.4 Return `200 OK` with empty `ExportLogsServiceResponse` for all other event types (don't error on `tool_result` etc.)

## 3. SQLite persistence

- [x] 3.1 Open/create `~/.local/share/claude-otlp/telemetry.db` on `serve` start
- [x] 3.2 Create `api_requests` table (schema: session_id, request_id, cost_usd, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, model, ts). Add UNIQUE constraint on `request_id` (idempotent inserts).
- [x] 3.3 INSERT on each received `api_request` event (INSERT OR IGNORE on duplicate `request_id`)

## 4. Statusline query (`status`)

- [x] 4.1 Detect current session ID: read latest `~/.claude/projects/*/` JSONL to find active `sessionId`, OR accept `--session` flag
- [x] 4.2 `SELECT SUM(cost_usd), SUM(input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens) FROM api_requests WHERE session_id = ?`
- [x] 4.3 Print `$<cost> ↑<tokens_compact>` (e.g., `$0.42 ↑1.2M`). Print nothing if no data or DB missing (silent fail for statusline).
- [x] 4.4 Add `--format json` flag for machine-readable output

## 5. Claude Code config

- [x] 5.1 Write `managed-settings.json` pointing `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318` and `OTEL_EXPORTER_OTLP_PROTOCOL=http/json`
- [x] 5.2 Verify Claude Code sends events to local receiver (check SQLite rows after a session)

## 6. Daemon management

- [x] 6.1 Write a launchd plist (`com.moritzschmitz-oviva.claude-otlp.plist`) to keep `serve` running as a user agent
- [x] 6.2 Document install steps in README: `go install`, copy plist, `launchctl load`

## 7. Statusline integration

- [x] 7.1 Document tmux status-right snippet: `#(claude-otlp status)`
- [x] 7.2 Document starship `custom.claude_cost` block
- [x] 7.3 Replace ccusage in current statusline setup

## 8. Validation

- [x] 8.1 Run a Claude Code session; confirm rows appear in SQLite
- [x] 8.2 Compare `SUM(cost_usd)` from SQLite against Claude Code's own statusbar cost — should match within rounding
- [x] 8.3 Compare against ccusage output to confirm the undercount gap from issue #866 is visible

<!-- 8.1-8.3 require a new Claude Code session after managed-settings.json takes effect -->

## 9. GCP forwarding auth

- [x] 9.1 Pass original request headers (especially `Authorization`, `x-goog-user-project`) through in `forward()` — Claude Code already attaches GCP Bearer token via `otelHeadersHelper`; the current bare `http.Post` drops them, causing 401 at `telemetry.googleapis.com`
