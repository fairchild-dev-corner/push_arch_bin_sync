package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

var syncLogger *log.Logger

// InitSyncLogger creates the sync_logs/ directory (relative to the working
// directory) if needed and configures LogSync to write to both stdout and
// a timestamped sync_logs/sync_logs_<date_timestamp>.logs file, one per run.
func InitSyncLogger() error {
	dir := "sync_logs"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create sync_logs directory: %w", err)
	}

	filename := fmt.Sprintf("sync_logs_%s.logs", time.Now().Format("20060102_150405"))
	f, err := os.OpenFile(filepath.Join(dir, filename), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open sync log file: %w", err)
	}

	syncLogger = log.New(io.MultiWriter(os.Stdout, f), "", log.LstdFlags)
	return nil
}

// LogSync writes a formatted successful-sync message to stdout and the
// current run's sync_logs/sync_logs_<date_timestamp>.logs file.
// If InitSyncLogger hasn't been called, it falls back to the standard logger.
func LogSync(format string, args ...interface{}) {
	line := fmt.Sprintf(format, args...)
	if syncLogger == nil {
		log.Println(line)
	} else {
		syncLogger.Println(line)
	}
	pushToLoki("sync", line)
}
