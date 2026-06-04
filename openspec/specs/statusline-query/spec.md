## MODIFIED Requirements

### Requirement: Monthly status query returns spend with override applied
`status monthly` SHALL return a JSON object. When a spend override exists for the queried month, the response SHALL include both the raw OTLP total and the corrected total. The corrected total is `target_usd + SUM(cost_usd WHERE ts > set_at)` — the override target plus any OTLP rows recorded after the override was set. When no override is active, the response is unchanged from the existing behavior.

#### Scenario: No override active
- **WHEN** `claude-otlp status monthly` is called and no override exists
- **THEN** response is `{"month":"YYYY-MM","cost_usd":<raw>,"total_tokens":<n>}`

#### Scenario: Override active, new spend accumulated after set_at
- **WHEN** an override of 96.37 was set at T, and 2.50 of OTLP rows exist with ts > T
- **THEN** response includes `"cost_usd":<raw_total>,"corrected_cost_usd":98.87,"offset_usd":<target - pre_override_raw>`

#### Scenario: OTLP rows older than set_at are excluded from post-override accumulation
- **WHEN** an override is active and a late-arriving OTLP row has ts < set_at
- **THEN** that row is not counted in post-override accumulation; `corrected_cost_usd` is unaffected

#### Scenario: Corrected cost has grown well past target
- **WHEN** `corrected_cost_usd > target_usd * 1.05`
- **THEN** response includes `"warning":"override_stale"` so the operator knows to clear it

#### Scenario: statusline-command.sh uses corrected value
- **WHEN** `statusline-command.sh` reads monthly output and `corrected_cost_usd` is present
- **THEN** the statusline displays the corrected value, not `cost_usd`
