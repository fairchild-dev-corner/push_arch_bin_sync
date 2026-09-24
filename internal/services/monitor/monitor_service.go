package monitor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"push_arch_bin_sync/internal/interfaces"
	"push_arch_bin_sync/internal/metrics"
	"push_arch_bin_sync/internal/models/change"
	models "push_arch_bin_sync/internal/models/config"
	"push_arch_bin_sync/internal/models/table"
	"push_arch_bin_sync/internal/utils"
)

var separator = strings.Repeat("=", 80)

// Stats tracks running monitoring statistics.
type Stats struct {
	TotalEvents      int64
	TotalInserts     int64
	TotalUpdates     int64
	TotalDeletes     int64
	SuccessfulWrites int64
	FailedWrites     int64
	UpdatesByTable   map[string]int64
	LastCheckTime    time.Time
	LastSuccessTime  time.Time
	LastErrorMessage string
}

// Monitor orchestrates reading binlog changes and replicating them to a Replicator.
type Monitor struct {
	config     *models.MonitorConfig
	binlog     interfaces.BinlogMonitor
	replicator interfaces.Replicator
	stats      *Stats
}

func NewMonitor(config *models.MonitorConfig, binlog interfaces.BinlogMonitor, replicator interfaces.Replicator) *Monitor {
	return &Monitor{
		config:     config,
		binlog:     binlog,
		replicator: replicator,
		stats: &Stats{
			UpdatesByTable: make(map[string]int64),
		},
	}
}

// applyChanges replicates changes to the destination database, updating write stats.
func (m *Monitor) applyChanges(changes []change.TableChange) bool {
	if len(changes) == 0 {
		return true
	}

	if err := m.replicator.ApplyBatch(changes); err != nil {
		m.stats.FailedWrites++
		m.stats.LastErrorMessage = err.Error()
		metrics.WritesTotal.WithLabelValues("failure").Inc()
		utils.LogError("Failed to replicate %d changes to %s@%s:%d: %v", len(changes), m.config.DestDBName, m.config.DestDBHost, m.config.DestDBPort, err)
		return false
	}

	utils.LogSync("✅ Replicated %d changes to %s@%s:%d", len(changes), m.config.DestDBName, m.config.DestDBHost, m.config.DestDBPort)
	m.stats.SuccessfulWrites++
	m.stats.LastSuccessTime = time.Now()
	metrics.WritesTotal.WithLabelValues("success").Inc()
	metrics.LastSuccessTimestamp.Set(float64(m.stats.LastSuccessTime.Unix()))
	return true
}

// formatRecordID renders a composite primary key as "col=val, col2=val2".
func formatRecordID(id map[string]interface{}) string {
	if len(id) == 0 {
		return "?"
	}

	keys := make([]string, 0, len(id))
	for k := range id {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s=%v", k, id[k])
	}
	return strings.Join(parts, ", ")
}

// PrintStatistics prints monitoring statistics.
func (m *Monitor) PrintStatistics() {
	log.Println("\n" + separator)
	log.Println("BINARY LOG MONITORING STATISTICS")
	log.Println(separator)
	log.Printf("Total Events Processed: %d", m.stats.TotalEvents)
	log.Printf("Inserts: %d", m.stats.TotalInserts)
	log.Printf("Updates: %d", m.stats.TotalUpdates)
	log.Printf("Deletes: %d", m.stats.TotalDeletes)
	log.Printf("\nDestination Writes:")
	log.Printf("Successful: %d", m.stats.SuccessfulWrites)
	log.Printf("Failed: %d", m.stats.FailedWrites)
	log.Printf("\nTiming:")
	log.Printf("  Last Check: %v", m.stats.LastCheckTime)
	log.Printf("  Last Success: %v", m.stats.LastSuccessTime)

	if len(m.stats.UpdatesByTable) > 0 {
		log.Println("\n📈 Changes by Table (15 Tables Monitored):")
		for tableName, count := range m.stats.UpdatesByTable {
			if count > 0 {
				log.Printf("  %-20s: %d changes", tableName, count)
			}
		}
	}

	log.Println(separator + "\n")
}

// Run starts continuous monitoring.
func (m *Monitor) Run() {
	log.Println("\n" + separator)
	log.Println("PUSH ARCH BIN SYNC BINARY LOG MONITOR")
	log.Println(separator)
	log.Printf("*Database Host: %s:%d", m.config.DBHost, m.config.DBPort)
	log.Printf("*Database Name: %s", m.config.DBName)
	log.Printf("*Monitoring Method: MySQL Binary Logs")
	log.Println("\nTables Being Monitored (15 Total):")
	for i, t := range table.AllTables {
		log.Printf("  %d. %-20s - %s", i+1, t.Name, t.Description)
	}
	log.Printf("\nSecurity:")
	log.Printf("• Transport: MySQL over TLS (pinned CA cert)")
	log.Printf("• Certificate: Pinned & Verified")
	log.Printf("\nDestination: MySQL (mirrored tables) at %s:%d", m.config.DestDBHost, m.config.DestDBPort)
	log.Println(separator + "\n")

	// Stats ticker - print every 5 minutes
	statsTicker := time.NewTicker(5 * time.Minute)
	defer statsTicker.Stop()

	checkCount := 0
	for {
		checkCount++
		m.stats.LastCheckTime = time.Now()
		metrics.LastCheckTimestamp.Set(float64(m.stats.LastCheckTime.Unix()))

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		changes, err := m.binlog.MonitorBinaryLogs(ctx)
		cancel()

		if err != nil {
			utils.LogError("Binary log read error: %v", err)
			m.stats.LastErrorMessage = err.Error()
			m.stats.FailedWrites++
			metrics.BinlogReadErrorsTotal.Inc()
			time.Sleep(5 * time.Second)
			continue
		}

		if len(changes) > 0 {
			log.Printf("\n[%s] Check - Detected %d changes",
				time.Now().Format("15:04:05"), len(changes))

			for _, c := range changes {
				m.stats.TotalEvents++
				m.stats.UpdatesByTable[c.TableName]++
				metrics.EventsTotal.WithLabelValues(c.Operation, c.TableName).Inc()

				recordInfo := formatRecordID(c.RecordID)
				switch c.Operation {
				case "INSERT":
					m.stats.TotalInserts++
				case "UPDATE":
					m.stats.TotalUpdates++
				case "DELETE":
					m.stats.TotalDeletes++
				}
				log.Printf("%s: %-20s (%s)", c.Operation, c.TableName, recordInfo)
				utils.PushEvent(c.Operation, c.TableName, recordInfo)
			}
			log.Printf("\n Replicating %d changes to %s@%s:%d...", len(changes), m.config.DestDBName, m.config.DestDBHost, m.config.DestDBPort)
			m.applyChanges(changes)

		} else {
			log.Printf("[%s] Checking - Monitoring active (no changes)", time.Now().Format("15:04:05"))
		}

		select {
		case <-statsTicker.C:
			m.PrintStatistics()
		default:
		}

		time.Sleep(1 * time.Second)
	}
}

// Close closes the underlying binlog monitor and replicator.
func (m *Monitor) Close() error {
	return errors.Join(m.binlog.Close(), m.replicator.Close())
}
