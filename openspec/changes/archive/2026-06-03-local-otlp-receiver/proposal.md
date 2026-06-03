## Why

ccusage reads Claude Code JSONL session logs to estimate cost, but those logs have fundamental token-count errors (`input_tokens` undercounts 100–174x, `output_tokens` 10–17x per ryoppippi/ccusage#866). Claude Code already emits accurate OTLP telemetry with a verbatim `cost_usd` field — routing it locally instead of to GCP gives a reliable, zero-cloud cost signal for statusline display.

## What Changes

- New Go binary `claude-otlp` with two subcommands: `serve` (OTLP HTTP daemon) and `status` (statusline query)
- `serve` listens on `localhost:4318`, receives OTLP HTTP/JSON logs, persists `api_request` events to SQLite at `~/.local/share/claude-otlp/telemetry.db`
- `status session` queries SQLite for current-session `SUM(cost_usd)` and total token count, prints a one-line statusline string
- `settings.json` env block redirects Claude Code's OTLP export to `localhost:4318`
- launchd plist keeps the daemon running as a user agent
- Replaces ccusage in the existing tmux/starship statusline setup

## Capabilities

### New Capabilities

- `otlp-receiver`: OTLP HTTP server that accepts Claude Code telemetry, filters `api_request` events, and persists them to SQLite
- `statusline-query`: CLI command that reads SQLite for current-session cost/tokens and emits a compact one-line string for statusline integrations

### Modified Capabilities

*(none — this is a net-new project)*

## Impact

- New binary installed to `~/.local/bin/claude-otlp`
- New SQLite database at `~/.local/share/claude-otlp/telemetry.db`
- `~/.claude/settings.json` gains OTLP env vars pointing to localhost
- Replaces `ccusage` calls in tmux `status-right` / starship config
- No cloud dependencies; no GCP auth required for local path
