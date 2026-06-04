## ADDED Requirements

### Requirement: Log level control via environment variable
The `serve` daemon SHALL read `CLAUDE_OTLP_LOG_LEVEL` at startup and gate log output accordingly. Valid values are `debug`, `info`, `warn`. The default SHALL be `info`. Unknown values SHALL fall back to `info` and log a warning.

#### Scenario: Default log level suppresses debug lines
- **WHEN** `CLAUDE_OTLP_LOG_LEVEL` is unset
- **THEN** DEBUG-level log lines SHALL NOT appear in output

#### Scenario: Debug level enables per-event lines
- **WHEN** `CLAUDE_OTLP_LOG_LEVEL=debug`
- **THEN** all DEBUG log lines SHALL appear in output

#### Scenario: Unknown value falls back to info
- **WHEN** `CLAUDE_OTLP_LOG_LEVEL=verbose` (or any unrecognized string)
- **THEN** daemon SHALL start with `info` level and emit a warning: `warn: unknown log level "verbose", using info`

### Requirement: Log each incoming OTLP log request
At DEBUG level the daemon SHALL log each POST to `/v1/logs` with method and body size in bytes before processing.

#### Scenario: Request received at debug level
- **WHEN** Claude Code POSTs a payload to `/v1/logs` and `CLAUDE_OTLP_LOG_LEVEL=debug`
- **THEN** a line SHALL be logged: `DEBUG recv /v1/logs <N> bytes`

### Requirement: Log parse results
After parsing an OTLP log payload the daemon SHALL log at DEBUG level the count of extracted `api_request` events.

#### Scenario: Events extracted
- **WHEN** payload contains 3 `api_request` log records
- **THEN** log line: `DEBUG parsed <N> api_request events`

### Requirement: Log per-event fields at DEBUG
For each parsed `api_request` event the daemon SHALL log at DEBUG level: `session_id`, `request_id`, `cost_usd`, and `model`.

#### Scenario: Event fields logged
- **WHEN** an `api_request` event is parsed with `session_id=abc`, `request_id=req-1`, `cost_usd=0.0042`, `model=claude-sonnet-4-6`
- **THEN** log line: `DEBUG api_request session=abc request=req-1 cost=$0.0042 model=claude-sonnet-4-6`

### Requirement: Warn on empty session_id or request_id
If a parsed `api_request` event has an empty `session_id` or `request_id` the daemon SHALL log a WARN line. These fields are required for correct storage and BQ forwarding.

#### Scenario: Empty session_id
- **WHEN** an event is parsed with `session_id=""`
- **THEN** WARN log: `warn: api_request has empty session_id (request_id=<val>)`

#### Scenario: Empty request_id
- **WHEN** an event is parsed with `request_id=""`
- **THEN** WARN log: `warn: api_request has empty request_id (session_id=<val>)`

### Requirement: Log duplicate INSERT skips
`db.Insert` SHALL return a boolean indicating whether a row was written (`true`) or skipped as a duplicate (`false`). The caller SHALL log a DEBUG line when a row is skipped.

#### Scenario: Duplicate request_id skipped
- **WHEN** an event with an already-stored `request_id` is inserted and `CLAUDE_OTLP_LOG_LEVEL=debug`
- **THEN** `db.Insert` returns `(false, nil)` and the caller logs: `DEBUG skip duplicate request_id=<val>`

#### Scenario: New row inserted
- **WHEN** an event with a new `request_id` is inserted
- **THEN** `db.Insert` returns `(true, nil)`

### Requirement: Log GCP forward result
The `forward` function SHALL log at DEBUG level the HTTP status code and latency for every forwarded request. It SHALL log at WARN level when the response status is ≥ 400.

#### Scenario: Successful forward
- **WHEN** GCP returns HTTP 200 and `CLAUDE_OTLP_LOG_LEVEL=debug`
- **THEN** log: `DEBUG forward /v1/logs: 200 (42ms)`

#### Scenario: Failed forward
- **WHEN** GCP returns HTTP 403
- **THEN** log (at any log level): `warn: forward /v1/logs: 403 (38ms)`
