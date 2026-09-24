package interfaces

import (
	"context"

	"push_arch_bin_sync/internal/models/change"
)

// BinlogMonitor reads row-level changes off the MySQL binary log.
type BinlogMonitor interface {
	MonitorBinaryLogs(ctx context.Context) ([]change.TableChange, error)
	Close() error
}
