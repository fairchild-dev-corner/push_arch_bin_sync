package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

var (
	lokiURL      string
	lokiWarnOnce sync.Once
	lokiClient   = &http.Client{Timeout: 2 * time.Second}
)

// InitLokiLogger sets the Loki push endpoint used by LogError/LogSync to
// stream log lines into Grafana's Logs panel. An empty url disables Loki
// push entirely - like Prometheus/Grafana, Loki is observability tooling,
// not a dependency the daemon needs to run.
func InitLokiLogger(url string) {
	lokiURL = url
}

// pushToLoki ships a single log line to Loki, best-effort. It runs in its
// own goroutine so a slow or unreachable Loki never blocks the caller, and
// repeated failures are logged once (not per line) to avoid spamming
// stdout/err_logs when Loki simply isn't running.
func pushToLoki(level, line string) {
	if lokiURL == "" {
		return
	}
	go func() {
		body, err := json.Marshal(map[string]interface{}{
			"streams": []map[string]interface{}{
				{
					"stream": map[string]string{
						"job":   "push_arch_bin_sync",
						"level": level,
					},
					"values": [][]string{
						{strconv.FormatInt(time.Now().UnixNano(), 10), line},
					},
				},
			},
		})
		if err != nil {
			return
		}

		resp, err := lokiClient.Post(lokiURL+"/loki/api/v1/push", "application/json", bytes.NewReader(body))
		if err != nil {
			lokiWarnOnce.Do(func() {
				log.Printf("⚠️ Loki unreachable at %s - live log panel won't have data (further push failures suppressed): %v", lokiURL, err)
			})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			lokiWarnOnce.Do(func() {
				log.Printf("⚠️ Loki push rejected with HTTP %d (further push failures suppressed)", resp.StatusCode)
			})
		}
	}()
}

// PushEvent records one detected row change (INSERT/UPDATE/DELETE) in two
// independent places: the local transaction_logs/ file (durable, always-on,
// unaffected by whether Loki is running or configured) and, best-effort,
// Loki as its own "event" stream for the Grafana Change Feed table. The line
// is logfmt-encoded (action=... table=... info=...) so Grafana's Loki table
// panel can parse it into ACTION/TABLE/INFO columns via `| logfmt`, with
// TIMESTAMP coming from Loki's own per-line ingest time - no extra column
// needs to be sent. table and action are low-cardinality (15 tables, 3
// operations) so this is safe as a query-time parse; info (the record's PK
// values) stays out of labels entirely to avoid unbounded label cardinality.
func PushEvent(action, table, info string) {
	line := fmt.Sprintf("action=%s table=%s info=%q", action, table, info)
	logTransaction(line)
	pushToLoki("event", line)
}
