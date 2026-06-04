## Why

OTLP telemetry can drift from actual Anthropic spend — gaps occur when the daemon isn't running, when requests bypass the local receiver, or after a month boundary reset. A manual correction command lets the operator pin the spend to the known-accurate figure from the Anthropic billing dashboard without resetting all collected data.

## What Changes

- Add `claude-otlp set-spent <amount>` subcommand to the binary
- Command writes a spend-offset record to SQLite so that subsequent `status monthly` queries return the corrected total
- The offset record is visible in output: `status monthly` reports both raw OTLP total and the corrected total when an offset is active

## Capabilities

### New Capabilities
- `set-spent`: CLI command that records a monthly spend override into the SQLite store; `status monthly` applies the override when present

### Modified Capabilities
- `statusline-query`: `status monthly` output gains an `offset_usd` field and uses `corrected_cost_usd` when an offset is set

## Impact

- `internal/store/db.go`: new table `spend_overrides (month TEXT PRIMARY KEY, target_usd REAL, set_at DATETIME)`
- `cmd/claude-otlp/main.go`: new `setSpentCmd()` cobra subcommand
- `status monthly` JSON output: adds `offset_usd` and `corrected_cost_usd` fields when override active
- `statusline-command.sh`: reads `corrected_cost_usd` if present, otherwise falls back to `cost_usd`
