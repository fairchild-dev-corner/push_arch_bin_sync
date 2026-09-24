package metrics

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// EventsTotal counts binlog row events processed, by operation and table.
var EventsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "binlog_sync_events_total",
	Help: "Total binlog row events processed, by operation and table.",
}, []string{"operation", "table"})

// WritesTotal counts destination-write batches, by result ("success" or "failure").
var WritesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "binlog_sync_destination_writes_total",
	Help: "Total batches applied to the destination database, by result.",
}, []string{"result"})

// BinlogReadErrorsTotal counts errors reading from the source binlog.
var BinlogReadErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "binlog_sync_read_errors_total",
	Help: "Total binlog read errors from the source.",
})

// LastSuccessTimestamp is the Unix time of the last successful replication
// to the destination. Prometheus staleness on the scrape target itself
// (the "up" metric) already signals the process being down; this signals
// the process being up but stuck (e.g. destination unreachable).
var LastSuccessTimestamp = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "binlog_sync_last_success_timestamp_seconds",
	Help: "Unix timestamp of the last successful replication to the destination.",
})

// LastCheckTimestamp is the Unix time of the last binlog poll cycle.
var LastCheckTimestamp = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "binlog_sync_last_check_timestamp_seconds",
	Help: "Unix timestamp of the last binlog poll cycle.",
})

// Serve starts the Prometheus /metrics HTTP endpoint on addr (e.g. ":9308")
// in the background. A failure here is logged but doesn't crash the
// daemon - metrics are observability, not core functionality.
func Serve(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Printf("metrics server stopped: %v", err)
		}
	}()

	log.Printf("Metrics available at %s/metrics", addr)
}
