# claude-otlp-statusline

Local OTLP receiver that captures Claude Code telemetry and exposes it for statusline display.

## Problem

ccusage reads Claude Code JSONL session logs. Those logs have severe data quality issues (GitHub issue ryoppippi/ccusage#866): `input_tokens` undercounts by 100–174x, `output_tokens` by 10–17x. Only cache tokens are accurate. Cost estimates from ccusage can be 9x lower than actual.

## Solution

Claude Code emits accurate OTLP telemetry to `telemetry.googleapis.com`. The `api_request` events include a verbatim `cost_usd` field (same source as Claude Code's own statusbar). Instead of routing to GCP, receive OTLP locally, persist to SQLite, expose a fast statusline query command.

## Architecture

```
Claude Code
  └─ OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
       └─ local receiver (Go binary, daemon)
            └─ SQLite (~/.local/share/claude-otlp/telemetry.db)
                 └─ statusline binary  →  tmux / starship / p10k
```

## Key OTLP fields (from telemetry-platform-poc POC)

`api_request` event labels:
- `cost_usd` — direct cost per request (no recalculation needed)
- `input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_creation_tokens`
- `session_id` — aggregate per session
- `model`, `user_email`, `organization_id`

Event types observed: `api_request`, `tool_result`, `tool_decision`, `hook_execution_start`, `hook_execution_complete`

## Prior art

- `telemetry-platform-poc` — POC validating RFC-179 Option G (local Claude Code → GCP). Validated that `cost_usd` is accurate in OTLP stream. FINDINGS.md has full schema notes.
- ccusage — reads JSONL, severely undercounts. ryoppippi/ccusage#866 is the upstream bug report.

## GCP / infra

None. Fully local. SQLite only. No cloud dependency.

## Config

Claude Code uses `OTEL_EXPORTER_OTLP_ENDPOINT` + `managed-settings.json` (or env) to point at a custom OTLP endpoint. Receiver must listen on OTLP HTTP (`/v1/logs`, `/v1/metrics`).
