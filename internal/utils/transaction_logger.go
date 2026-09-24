package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

var transactionLogger *log.Logger

// InitTransactionLogger creates the transaction_logs/ directory (relative to
// the working directory) if needed and configures PushEvent to persist a
// copy of every detected row change to a timestamped
// transaction_logs/transactions_<date_timestamp>.logs file, one per run.
// This is deliberately file-only (no stdout) - the per-row INSERT/UPDATE/
// DELETE line is already printed to the console separately in
// monitor_service.go's Run() loop; writing it here too would duplicate it.
func InitTransactionLogger() error {
	dir := "transaction_logs"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create transaction_logs directory: %w", err)
	}

	filename := fmt.Sprintf("transactions_%s.logs", time.Now().Format("20060102_150405"))
	f, err := os.OpenFile(filepath.Join(dir, filename), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open transaction log file: %w", err)
	}

	transactionLogger = log.New(f, "", log.LstdFlags)
	return nil
}

// logTransaction writes one pre-formatted change record to the current run's
// transaction_logs file. A no-op if InitTransactionLogger wasn't called -
// unlike LogError/LogSync, there's no fallback to the standard logger here,
// since PushEvent (the only caller) already has the console line covered.
func logTransaction(line string) {
	if transactionLogger == nil {
		return
	}
	transactionLogger.Println(line)
}
