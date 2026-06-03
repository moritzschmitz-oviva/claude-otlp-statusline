package otlp

import (
	"encoding/json"
	"strconv"
)

// OTLP HTTP JSON types — minimal subset for parsing api_request events.

type ExportLogsServiceRequest struct {
	ResourceLogs []ResourceLogs `json:"resourceLogs"`
}

type ResourceLogs struct {
	Resource  Resource    `json:"resource"`
	ScopeLogs []ScopeLogs `json:"scopeLogs"`
}

type Resource struct {
	Attributes []KeyValue `json:"attributes"`
}

type ScopeLogs struct {
	LogRecords []LogRecord `json:"logRecords"`
}

type LogRecord struct {
	TimeUnixNano string     `json:"timeUnixNano"`
	Body         AnyValue   `json:"body"`
	Attributes   []KeyValue `json:"attributes"`
}

type KeyValue struct {
	Key   string   `json:"key"`
	Value AnyValue `json:"value"`
}

// AnyValue handles OTLP proto3 JSON encoding where int64 fields are strings.
type AnyValue struct {
	StringValue *string      `json:"stringValue,omitempty"`
	BoolValue   *bool        `json:"boolValue,omitempty"`
	IntValue    *int64JSON   `json:"intValue,omitempty"`
	DoubleValue *float64JSON `json:"doubleValue,omitempty"`
}

func (a AnyValue) AsString() string {
	if a.StringValue != nil {
		return *a.StringValue
	}
	return ""
}

func (a AnyValue) AsFloat64() float64 {
	if a.DoubleValue != nil {
		return float64(*a.DoubleValue)
	}
	if a.IntValue != nil {
		return float64(*a.IntValue)
	}
	return 0
}

func (a AnyValue) AsInt64() int64 {
	if a.IntValue != nil {
		return int64(*a.IntValue)
	}
	if a.DoubleValue != nil {
		return int64(*a.DoubleValue)
	}
	return 0
}

// int64JSON handles proto3 JSON encoding: int64 may arrive as a JSON string or number.
type int64JSON int64

func (v *int64JSON) UnmarshalJSON(b []byte) error {
	// Try number first
	var n int64
	if err := json.Unmarshal(b, &n); err == nil {
		*v = int64JSON(n)
		return nil
	}
	// Try string
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*v = int64JSON(n)
	return nil
}

// float64JSON handles proto3 JSON encoding: double may arrive as number or string.
type float64JSON float64

func (v *float64JSON) UnmarshalJSON(b []byte) error {
	var f float64
	if err := json.Unmarshal(b, &f); err == nil {
		*v = float64JSON(f)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*v = float64JSON(f)
	return nil
}

// ApiRequest holds the parsed fields from a claude_code.api_request log record.
type ApiRequest struct {
	SessionID           string
	RequestID           string
	CostUSD             float64
	InputTokens         int64
	OutputTokens        int64
	CacheReadTokens     int64
	CacheCreationTokens int64
	Model               string
	TimeUnixNano        uint64
}

// ParseApiRequests extracts all api_request events from an export request.
func ParseApiRequests(req *ExportLogsServiceRequest) []ApiRequest {
	var results []ApiRequest
	for _, rl := range req.ResourceLogs {
		for _, sl := range rl.ScopeLogs {
			for _, lr := range sl.LogRecords {
				if lr.Body.AsString() != "claude_code.api_request" {
					continue
				}
				results = append(results, parseRecord(lr))
			}
		}
	}
	return results
}

func parseRecord(lr LogRecord) ApiRequest {
	attrs := make(map[string]AnyValue, len(lr.Attributes))
	for _, kv := range lr.Attributes {
		attrs[kv.Key] = kv.Value
	}

	var ts uint64
	if lr.TimeUnixNano != "" {
		n, _ := strconv.ParseUint(lr.TimeUnixNano, 10, 64)
		ts = n
	}

	return ApiRequest{
		SessionID:           attrs["session.id"].AsString(),
		RequestID:           attrs["request_id"].AsString(),
		CostUSD:             attrs["cost_usd"].AsFloat64(),
		InputTokens:         attrs["input_tokens"].AsInt64(),
		OutputTokens:        attrs["output_tokens"].AsInt64(),
		CacheReadTokens:     attrs["cache_read_tokens"].AsInt64(),
		CacheCreationTokens: attrs["cache_creation_tokens"].AsInt64(),
		Model:               attrs["model"].AsString(),
		TimeUnixNano:        ts,
	}
}
