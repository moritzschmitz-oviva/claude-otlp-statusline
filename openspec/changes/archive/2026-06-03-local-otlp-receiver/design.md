# Design: local-otlp-receiver

## Problem

ccusage reads Claude Code JSONL logs to estimate session cost. Those logs have fundamental data quality issues (ryoppippi/ccusage#866): `input_tokens` undercounts 100–174x, `output_tokens` 10–17x. Cache tokens are accurate. Real-world example: ccusage reported ~$50 for a day where actual cost was ~$446.

Claude Code already emits accurate OTLP telemetry (`api_request` events with verbatim `cost_usd`). The telemetry is normally routed to `telemetry.googleapis.com`. This project captures it locally instead.

## Approach

A small Go binary (`claude-otlp`) acts as both:
1. **Receiver daemon** — OTLP HTTP server (`/v1/logs`, `/v1/metrics`) on `localhost:4318`, persists `api_request` events to SQLite
2. **Statusline command** — queries SQLite for current-session cost/tokens, prints one line for tmux/starship/p10k

Single binary, two subcommands: `serve` and `status`.

## Data flow

```
Claude Code
  OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
    │  (OTLP HTTP protobuf or JSON)
    ▼
claude-otlp serve          (~/.local/share/claude-otlp/telemetry.db)
    │  INSERT INTO api_requests (session_id, cost_usd, input_tokens, ...)
    ▼
SQLite
    │  SELECT SUM(cost_usd) WHERE session_id = ?
    ▼
claude-otlp status         → "$0.42 | 1.2M tok"
    │
    ▼
statusline (tmux / starship / p10k)
```

## Claude Code config

`managed-settings.json` (or `~/.claude/settings.json`) sets:

```json
{
  "env": {
    "OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4318",
    "OTEL_EXPORTER_OTLP_PROTOCOL": "http/json"
  }
}
```

No auth helper needed (local HTTP). No GCP dependency.

## Schema: `api_requests` table

| Column | Source |
|---|---|
| `session_id` | `labels.session_id` |
| `request_id` | `labels.request_id` |
| `cost_usd` | `labels.cost_usd` |
| `input_tokens` | `labels.input_tokens` |
| `output_tokens` | `labels.output_tokens` |
| `cache_read_tokens` | `labels.cache_read_tokens` |
| `cache_creation_tokens` | `labels.cache_creation_tokens` |
| `model` | `labels.model` |
| `ts` | log record `timeUnixNano` |

## Statusline output format

Default: `$0.42 ↑1.2M` (cost + total tokens, compact).
If no active session or receiver not running: empty string (statusline renders nothing).

## Forwarding to GCP

`CLAUDE_OTLP_FORWARD_ENDPOINT` enables forwarding received payloads to a secondary OTLP endpoint (e.g. `telemetry.googleapis.com`).

`telemetry.googleapis.com` requires a GCP Bearer token. Claude Code handles this via `otelHeadersHelper` (`settings.json`): before each export, Claude Code calls the helper script which runs `gcloud auth print-access-token` and returns `{"Authorization":"Bearer <token>","x-goog-user-project":"<project>"}`. These headers are attached to requests sent to `localhost:4318`.

The `forward()` function must pass the original request headers through — not just `Content-Type`. The token is already present in the incoming request; no separate auth mechanism needed. Token expiry is not a concern for fire-and-forget forwarding since the token was freshly minted when Claude Code called the helper.

```
Claude Code
  │  POST /v1/logs
  │  Authorization: Bearer <token>   ← from otelHeadersHelper
  ▼
localhost:4318
  │  store to SQLite
  │  forward with original headers   ← must pass Authorization through
  ▼
telemetry.googleapis.com  ✓
```

## Non-goals

- No GCP, no cloud, no auth
- No metrics endpoint processing (logs only; cost is in `api_request` events)
- No UI beyond one-line statusline output
- No cross-user or multi-machine aggregation
