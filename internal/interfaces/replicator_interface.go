package interfaces

import "push_arch_bin_sync/internal/models/change"

// Replicator applies one poll cycle's worth of detected changes to a
// destination store, atomically where possible.
type Replicator interface {
	ApplyBatch(changes []change.TableChange) error
	Close() error
}
