## Context

`claude-otlp status monthly` sums `cost_usd` from the `api_requests` SQLite table. If the daemon was offline for part of the month, or if requests were routed directly to GCP instead of localhost:4318, the SQLite total will be lower than actual Anthropic billing. The `statusline-command.sh` already queries `status monthly` live, so a correction mechanism inside SQLite propagates to the statusline without touching any shell scripts.

## Goals / Non-Goals

**Goals:**
- Single-shot CLI command to pin the current month's displayed spend to a user-supplied value
- Override persists across daemon restarts (stored in SQLite, not in-memory)
- `status monthly` output is self-describing: always emits both raw and corrected totals when an override is active
- `statusline-command.sh` picks up the corrected value transparently

**Non-Goals:**
- No retroactive editing of `api_requests` rows (override is additive metadata, not data surgery)
- No per-session overrides — monthly granularity only
- No UI or interactive prompts — CLI flag only

## Decisions

### Store override in a dedicated SQLite table, not by mutating `api_requests`

Alternatives: (a) add an `adjustment` row to `api_requests`, (b) write to a separate file.

Rationale: a dedicated `spend_overrides` table keeps raw telemetry immutable. Querying is a simple LEFT JOIN. No file I/O race with the daemon. The table can be cleared (`set-spent 0` or `set-spent --clear`) without touching collected data.

### Override semantics: snapshot + accumulate

`set-spent 96.37` means "as of now, total spend is $96.37; track new spend on top of that." `corrected_cost_usd = target_usd + SUM(cost_usd WHERE ts > set_at)`. OTLP rows recorded after the override's `set_at` timestamp accumulate normally; rows before it are replaced by the target figure.

This lets the statusline remain live after the correction — new API calls still register — while absorbing the historical gap that caused the drift.

### `status monthly` adds `corrected_cost_usd` field only when override is active

If no override exists for the month, `corrected_cost_usd` is absent from the JSON. This avoids breaking existing consumers that only read `cost_usd`. `statusline-command.sh` checks for `corrected_cost_usd` with a jq fallback to `cost_usd`.

### `set-spent 0` clears the override

Zero is not a meaningful spend target. Using it as a clear signal avoids adding a separate `--clear` flag and keeps the CLI simple. Alternatively a `--clear` flag could be added later.

## Risks / Trade-offs

- [Post-override OTLP catch-up] If old OTLP rows arrive late (daemon was offline and replays), they pre-date `set_at` and are excluded from post-override accumulation — the correction stays accurate. No special handling needed.
- [Raw accumulation past target] Once post-override OTLP accumulation pushes `corrected_cost_usd` well above target, the override is no longer doing useful work → `status monthly` emits `"warning":"override_stale"` when `corrected_cost_usd > target_usd * 1.05` so the operator knows to clear it.
- [Month boundary] The override is keyed by `YYYY-MM`. It expires naturally when the month rolls over — no cleanup needed.
- [Concurrent writes] The daemon is already writing `api_requests`; a human running `set-spent` concurrently is safe because SQLite serializes writes and the override table is only written by the CLI, never by the daemon.
