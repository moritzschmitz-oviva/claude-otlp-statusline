package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/moritzschmitz-oviva/claude-otlp-statusline/internal/otlp"
	"github.com/moritzschmitz-oviva/claude-otlp-statusline/internal/session"
	"github.com/moritzschmitz-oviva/claude-otlp-statusline/internal/store"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "claude-otlp",
		Short: "Local OTLP receiver and statusline query for Claude Code telemetry",
	}
	root.AddCommand(serveCmd(), statusCmd())
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// serve -----------------------------------------------------------------------

func serveCmd() *cobra.Command {
	var addr string
	var foreground bool
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the OTLP HTTP receiver (persists api_request events to SQLite)",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Daemon mode (default): daemonize on first call, run in background.
			// Skip if already running or if we are the daemon child.
			if !foreground && os.Getenv("_CLAUDE_OTLP_DAEMON") == "" {
				if portOpen(addr) {
					return nil // already running — exit 0 silently
				}
				return daemonize(addr)
			}

			db, err := store.Open()
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer db.Close()

			forwardBase := strings.TrimRight(os.Getenv("CLAUDE_OTLP_FORWARD_ENDPOINT"), "/")
			dbPath, _ := store.DBPath()
			if forwardBase != "" {
				log.Printf("claude-otlp serve: listening on %s, db %s, forwarding to %s", addr, dbPath, forwardBase)
			} else {
				log.Printf("claude-otlp serve: listening on %s, db %s", addr, dbPath)
			}

			mux := http.NewServeMux()
			mux.HandleFunc("/v1/logs", logsHandler(db, forwardBase))
			mux.HandleFunc("/v1/metrics", forwardHandler(forwardBase))
			mux.HandleFunc("/v1/traces", forwardHandler(forwardBase))

			return http.ListenAndServe(addr, mux)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "localhost:4318", "Listen address")
	cmd.Flags().BoolVar(&foreground, "foreground", false, "Run in foreground (skip daemonize)")
	return cmd
}

// portOpen returns true if something is already listening on addr.
func portOpen(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// daemonize re-execs the current binary as a detached daemon and exits the parent.
func daemonize(addr string) error {
	logFile, err := os.OpenFile("/tmp/claude-otlp.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logFile, _ = os.Open(os.DevNull)
	}

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("executable: %w", err)
	}

	child := exec.Command(self, "serve", "--addr", addr)
	child.Env = append(os.Environ(), "_CLAUDE_OTLP_DAEMON=1")
	child.Stdin = nil
	child.Stdout = logFile
	child.Stderr = logFile
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := child.Start(); err != nil {
		return fmt.Errorf("daemonize: %w", err)
	}
	os.Exit(0)
	return nil
}

func logsHandler(db *store.DB, forwardBase string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}

		var req otlp.ExportLogsServiceRequest
		if err := json.Unmarshal(body, &req); err != nil {
			// Return 200 even on parse error — don't break Claude Code on bad payloads.
			log.Printf("warn: parse OTLP logs: %v", err)
		} else {
			events := otlp.ParseApiRequests(&req)
			for _, e := range events {
				if err := db.Insert(e); err != nil {
					log.Printf("warn: insert api_request %s: %v", e.RequestID, err)
				}
			}
		}

		if forwardBase != "" {
			go forward(forwardBase+"/v1/logs", r.Header, body)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}
}

// forwardHandler returns 200 and optionally forwards the raw body to the remote endpoint.
func forwardHandler(forwardBase string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if forwardBase != "" && r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err == nil {
				go forward(forwardBase+r.URL.Path, r.Header, body)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}
}

// forward sends body to url asynchronously, passing the original request headers.
// This preserves GCP auth headers (Authorization, x-goog-user-project) that Claude Code
// attaches via otelHeadersHelper, required for forwarding to telemetry.googleapis.com.
func forward(url string, origHeaders http.Header, body []byte) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body)) //nolint:gosec
	if err != nil {
		log.Printf("forward %s: build request: %v", url, err)
		return
	}
	for key, vals := range origHeaders {
		for _, v := range vals {
			req.Header.Add(key, v)
		}
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("forward %s: %v", url, err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		log.Printf("forward %s: remote returned %d", url, resp.StatusCode)
	}
}

// status ----------------------------------------------------------------------

func statusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Query cost and token usage from SQLite",
	}
	cmd.AddCommand(statusSessionCmd(), statusMonthlyCmd())
	return cmd
}

// sessionFromStdin reads a JSON object from stdin (when piped) and returns the session_id field.
// Falls back to the CLAUDE_CODE_SESSION_ID env var that Claude Code always sets.
func sessionFromStdin() string {
	fi, err := os.Stdin.Stat()
	if err == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
		var payload struct {
			SessionID string `json:"session_id"`
		}
		if err := json.NewDecoder(os.Stdin).Decode(&payload); err == nil && payload.SessionID != "" {
			return payload.SessionID
		}
	}
	return os.Getenv("CLAUDE_CODE_SESSION_ID")
}

func statusSessionCmd() *cobra.Command {
	var sessionFlag string
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Current-session cost and tokens (JSON)",
		RunE: func(cmd *cobra.Command, args []string) error {
			sid := sessionFlag
			if sid == "" {
				sid = sessionFromStdin()
			}
			if sid == "" {
				var err error
				sid, err = session.Detect()
				if err != nil {
					return jsonOut(map[string]any{})
				}
			}
			db, err := store.Open()
			if err != nil {
				return jsonOut(map[string]any{})
			}
			defer db.Close()

			stats, err := db.QuerySession(sid)
			if err != nil {
				return jsonOut(map[string]any{})
			}
			return jsonOut(map[string]any{
				"session_id":   sid,
				"cost_usd":     stats.CostUSD,
				"total_tokens": stats.TotalTokens,
			})
		},
	}
	cmd.Flags().StringVar(&sessionFlag, "session", "", "Session ID (auto-detected if omitted)")
	return cmd
}

func statusMonthlyCmd() *cobra.Command {
	var monthFlag string
	cmd := &cobra.Command{
		Use:   "monthly",
		Short: "Current-month cost and tokens (JSON)",
		RunE: func(cmd *cobra.Command, args []string) error {
			month := monthFlag
			if month == "" {
				month = time.Now().Format("2006-01")
			}
			db, err := store.Open()
			if err != nil {
				return jsonOut(map[string]any{})
			}
			defer db.Close()

			stats, err := db.QueryMonthly(month)
			if err != nil {
				return jsonOut(map[string]any{})
			}
			return jsonOut(map[string]any{
				"month":        month,
				"cost_usd":     stats.CostUSD,
				"total_tokens": stats.TotalTokens,
			})
		},
	}
	cmd.Flags().StringVar(&monthFlag, "month", "", "Month YYYY-MM (default: current)")
	return cmd
}

func jsonOut(v any) error {
	return json.NewEncoder(os.Stdout).Encode(v)
}
