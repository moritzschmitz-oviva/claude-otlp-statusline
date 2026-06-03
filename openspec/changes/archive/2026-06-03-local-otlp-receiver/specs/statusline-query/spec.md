## ADDED Requirements

### Requirement: Query current-session cost and tokens
The `status session` command SHALL query SQLite for the session identified by `CLAUDE_CODE_SESSION_ID` (or `--session` flag) and return the aggregated cost and total token count.

#### Scenario: Active session with data
- **WHEN** `status session` is run and the current session has rows in `api_requests`
- **THEN** the command prints `$<cost> ↑<tokens_compact>` (e.g. `$0.42 ↑1.2M`) to stdout and exits 0

#### Scenario: No data for session
- **WHEN** `status session` is run and no rows match the current session ID
- **THEN** the command prints nothing and exits 0

#### Scenario: Database file missing
- **WHEN** the SQLite file does not exist
- **THEN** the command prints nothing and exits 0 (silent fail for statusline)

### Requirement: Detect session ID automatically
The `status session` command SHALL resolve the current session ID from `CLAUDE_CODE_SESSION_ID` env var if set, otherwise by reading the most recent Claude Code JSONL file under `~/.claude/projects/`.

#### Scenario: Env var present
- **WHEN** `CLAUDE_CODE_SESSION_ID` is set in the environment
- **THEN** that value is used as the session ID without reading any JSONL files

#### Scenario: Env var absent
- **WHEN** `CLAUDE_CODE_SESSION_ID` is not set
- **THEN** the command walks `~/.claude/projects/*/` JSONL files, sorted by modification time descending, and extracts the most recent `sessionId` field

### Requirement: Accept --session flag override
The `status session` command SHALL accept a `--session <id>` flag to query a specific session ID instead of the auto-detected one.

#### Scenario: Explicit session flag
- **WHEN** `status session --session <uuid>` is invoked
- **THEN** the specified UUID is used and env var / JSONL detection is skipped

### Requirement: Support JSON output format
The `status session` command SHALL support a `--format json` flag (or subcommand `status session`) that emits `{"session_id":"...","cost_usd":0.42,"total_tokens":420000}`.

#### Scenario: JSON format requested
- **WHEN** `status session` is invoked (as a subcommand, JSON output is default)
- **THEN** stdout contains a valid JSON object with `session_id`, `cost_usd`, and `total_tokens` fields

### Requirement: Compact token notation
The default text output SHALL format total tokens using SI suffixes: `K` (thousands), `M` (millions).

#### Scenario: Token count over one million
- **WHEN** total tokens = 1,234,567
- **THEN** output is `↑1.2M`

#### Scenario: Token count under one thousand
- **WHEN** total tokens = 847
- **THEN** output is `↑847`
