## 1. Log level infrastructure

- [x] 1.1 Add `logLevel` package-level int constant (`logDebug=0`, `logInfo=1`, `logWarn=2`) and `currentLogLevel` var in `main.go`
- [x] 1.2 Add `logDebug(format, args)` and `logWarn(format, args)` helpers that gate on `currentLogLevel`
- [x] 1.3 Parse `CLAUDE_OTLP_LOG_LEVEL` env var in `serveCmd.RunE` before starting the server; set `currentLogLevel`; warn on unknown value

## 2. Store: Insert return signature

- [x] 2.1 Change `db.Insert` signature from `error` to `(bool, error)` — derive bool from `sql.Result.RowsAffected() > 0`

## 3. Ingest path logging (`logsHandler`)

- [x] 3.1 Log DEBUG after reading body: `recv /v1/logs <N> bytes`
- [x] 3.2 Log DEBUG after parsing: `parsed <N> api_request events`
- [x] 3.3 For each parsed event log DEBUG: `api_request session=<> request=<> cost=$<> model=<>`
- [x] 3.4 Log WARN when `session_id` is empty: `api_request has empty session_id (request_id=<>)`
- [x] 3.5 Log WARN when `request_id` is empty: `api_request has empty request_id (session_id=<>)`
- [x] 3.6 Use updated `db.Insert` bool return; log DEBUG `skip duplicate request_id=<>` when `inserted=false`

## 4. Forward path logging (`forward`)

- [x] 4.1 Capture `start := time.Now()` before sending HTTP request
- [x] 4.2 Log DEBUG on any response: `forward <path>: <status> (<ms>ms)`
- [x] 4.3 Log WARN (unconditional, overriding level gate) when status ≥ 400: `forward <path>: <status> (<ms>ms)`
- [x] 4.4 Keep existing error log for transport failures (no change needed)

## 5. Build and verify

- [x] 5.1 Build binary: `go build -o ~/.local/bin/claude-otlp ./cmd/claude-otlp`
- [x] 5.2 Restart daemon and tail log to confirm startup line still present
- [x] 5.3 Run `CLAUDE_OTLP_LOG_LEVEL=debug ~/.local/bin/claude-otlp serve --foreground` and trigger a Claude Code API call; verify DEBUG lines appear with populated `session_id` and `cost_usd`
