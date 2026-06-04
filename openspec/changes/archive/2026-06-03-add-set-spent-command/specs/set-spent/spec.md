## ADDED Requirements

### Requirement: Operator can pin monthly spend to a known-correct value
The binary SHALL expose a `set-spent` subcommand that records a target total spend for the current month in the SQLite store. Subsequent `status monthly` queries SHALL reflect the corrected value.

#### Scenario: Set spend to billing dashboard value
- **WHEN** operator runs `claude-otlp set-spent 96.37`
- **THEN** the command exits 0 and prints `Spend override set for 2026-06: $96.37`

#### Scenario: Set-spent value is negative or non-numeric
- **WHEN** operator runs `claude-otlp set-spent -1` or `claude-otlp set-spent abc`
- **THEN** the command exits non-zero and prints an error message without modifying the store

#### Scenario: Clear override by setting spend to zero
- **WHEN** operator runs `claude-otlp set-spent 0`
- **THEN** any existing override for the current month is deleted and the command prints `Spend override cleared for 2026-06`

#### Scenario: Override persists across daemon restart
- **WHEN** `set-spent 96.37` has been run and the daemon is restarted
- **THEN** `status monthly` still returns `corrected_cost_usd: 96.37`

### Requirement: set-spent accepts an optional month flag
The command SHALL accept `--month YYYY-MM` to target a month other than the current one.

#### Scenario: Override a past month
- **WHEN** operator runs `claude-otlp set-spent 45.00 --month 2026-05`
- **THEN** the override is stored for `2026-05` and `status monthly --month 2026-05` reflects the correction
