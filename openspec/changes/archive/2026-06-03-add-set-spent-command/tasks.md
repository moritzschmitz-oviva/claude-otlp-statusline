## 1. SQLite Store Changes

- [x] 1.1 Add `spend_overrides` table to `internal/store/db.go` (`month TEXT PRIMARY KEY, target_usd REAL, set_at DATETIME NOT NULL`)
- [x] 1.2 Add `SetSpendOverride(month string, targetUSD float64) error` method — upserts on month key; deletes row when targetUSD == 0
- [x] 1.3 Update `QueryMonthly` to LEFT JOIN `spend_overrides` on `month`; compute `post_override_cost = SUM(cost_usd WHERE ts > set_at)`; return `OverrideTarget float64`, `OverrideSetAt time.Time`, `PostOverrideCost float64`, `OverrideActive bool` on the result struct

## 2. CLI — set-spent Command

- [x] 2.1 Add `setSpentCmd()` cobra subcommand in `cmd/claude-otlp/main.go`
- [x] 2.2 Parse positional arg as float64; reject negative values and non-numeric input with non-zero exit
- [x] 2.3 Add `--month` flag defaulting to current `YYYY-MM` (override is always per-month — the flag selects which month, not the granularity)
- [x] 2.4 Call `db.SetSpendOverride(month, amount)`; print `"Spend override set for YYYY-MM: $X.XX"` or `"Spend override cleared for YYYY-MM"` on zero value

## 3. status monthly Output Changes

- [x] 3.1 When override is active, compute `corrected_cost_usd = target_usd + post_override_cost` and `offset_usd = target_usd - (raw_total - post_override_cost)`; include both in JSON response
- [x] 3.2 Add `"warning":"override_stale"` field when `corrected_cost_usd > target_usd * 1.05` (accumulated new spend has grown well past the correction point)

## 4. statusline-command.sh

- [x] 4.1 Change `jq -r '.cost_usd'` extraction to `jq -r '(.corrected_cost_usd // .cost_usd)'` so the corrected value is used when present

## 5. Verification

- [x] 5.1 Build binary (`go build -o ~/.local/bin/claude-otlp ./cmd/claude-otlp`)
- [x] 5.2 Smoke-test: `set-spent 96.37` → `status monthly` shows `corrected_cost_usd ≥ 96.37`; new OTLP rows with ts > set_at add to it; rows with ts < set_at do not
- [x] 5.3 `set-spent 0` clears override; `status monthly` returns only `cost_usd`
- [x] 5.4 Confirm statusline output shows corrected value when override active
