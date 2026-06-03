## ADDED Requirements

### Requirement: Accept OTLP HTTP/JSON log exports
The receiver SHALL listen on `localhost:4318` and accept `POST /v1/logs` requests with `Content-Type: application/json` containing an `ExportLogsServiceRequest` payload.

#### Scenario: Valid OTLP logs request
- **WHEN** Claude Code POSTs an `ExportLogsServiceRequest` to `POST /v1/logs`
- **THEN** the receiver returns `200 OK` with an empty `ExportLogsServiceResponse`

#### Scenario: Non-api_request event types
- **WHEN** the payload contains log records with body other than `claude_code.api_request` (e.g. `tool_result`, `tool_decision`)
- **THEN** the receiver returns `200 OK` and does not persist any rows

### Requirement: Persist api_request events to SQLite
The receiver SHALL extract `api_request` log records from incoming payloads and INSERT each into the `api_requests` table in `~/.local/share/claude-otlp/telemetry.db`.

#### Scenario: New api_request event
- **WHEN** a log record with body `claude_code.api_request` is received
- **THEN** the receiver inserts a row with `session_id`, `request_id`, `cost_usd`, `input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_creation_tokens`, `model`, and `ts` populated from the log record attributes and `timeUnixNano`

#### Scenario: Duplicate request_id
- **WHEN** a log record arrives with a `request_id` already present in the table
- **THEN** the receiver silently ignores the duplicate (INSERT OR IGNORE) and returns `200 OK`

### Requirement: Auto-create database and schema on startup
The receiver SHALL create `~/.local/share/claude-otlp/telemetry.db` and the `api_requests` table if they do not exist when `serve` starts.

#### Scenario: First run on a machine
- **WHEN** `serve` starts and no database file exists
- **THEN** the database file and table are created before the HTTP listener begins accepting connections

### Requirement: Forward received payloads to a secondary OTLP endpoint
When `CLAUDE_OTLP_FORWARD_ENDPOINT` is set, the receiver SHALL forward the raw request body and all original request headers to that endpoint after persisting to SQLite.

#### Scenario: Forwarding enabled with GCP endpoint
- **WHEN** `CLAUDE_OTLP_FORWARD_ENDPOINT=https://telemetry.googleapis.com/v1/logs` and the incoming request carries an `Authorization: Bearer <token>` header
- **THEN** the receiver POSTs the same body with the same headers to the forward endpoint; a non-2xx response is logged but does not affect the `200 OK` returned to Claude Code

#### Scenario: Forwarding not configured
- **WHEN** `CLAUDE_OTLP_FORWARD_ENDPOINT` is unset
- **THEN** no forwarding occurs; the receiver only persists to SQLite

### Requirement: Run as a persistent user daemon
The receiver SHALL be packaged with a launchd plist so it can run as a macOS user agent that restarts on exit.

#### Scenario: Daemon already running on start
- **WHEN** `serve` is invoked and port 4318 is already bound
- **THEN** the new process exits cleanly without error
