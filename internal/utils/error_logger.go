package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

var errorLogger *log.Logger

// InitErrorLogger creates the err_logs/ directory (relative to the working
// directory) if needed and configures LogError to write to both stdout and
// a timestamped err_logs/error_<date_timestamp>.logs file, one per run.
func InitErrorLogger() error {
	dir := "err_logs"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create err_logs directory: %w", err)
	}

	filename := fmt.Sprintf("error_%s.logs", time.Now().Format("20060102_150405"))
	f, err := os.OpenFile(filepath.Join(dir, filename), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open error log file: %w", err)
	}

	errorLogger = log.New(io.MultiWriter(os.Stdout, f), "", log.LstdFlags)
	return nil
}

// LogError writes a formatted error message to stdout and the current run's
// err_logs/error_<date_timestamp>.logs file.
// If InitErrorLogger hasn't been called, it falls back to the standard logger.
func LogError(format string, args ...interface{}) {
	line := fmt.Sprintf(format, args...)
	if errorLogger == nil {
		log.Println(line)
	} else {
		errorLogger.Println(line)
	}
	pushToLoki("error", line)
}
