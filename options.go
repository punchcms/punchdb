package punchdb

import "time"

// Options configures the construction of a Store. Zero values are filled with
// sensible defaults by Normalize.
type Options struct {
	// CacheSizeMB is the memory (in MB) allocated to the read/write cache.
	// Default: 64
	CacheSizeMB int

	// MemTableSizeMB is the write buffer size (in MB). Default: 64
	MemTableSizeMB int

	// MaxOpenFiles is the maximum number of open file descriptors. Default: 64
	MaxOpenFiles int

	// CompactionThreads is the number of concurrent compaction goroutines.
	// 0 means CPU/2 (minimum 1). Default: 0
	CompactionThreads int

	// WALDir is a dedicated directory for the write-ahead log. If empty,
	// {path}/wal is used. Default: ""
	WALDir string

	// ReadOnly opens the database in read-only mode when true. Default: false
	ReadOnly bool

	// OnEvent is an optional callback for database events (background errors,
	// write stalls, compactions, WAL creation). If nil, events are ignored.
	// It is invoked from background goroutines and is NOT part of the hot path.
	OnEvent func(Event)

	// MetricsInterval is the interval for periodic metrics collection. When 0,
	// no background metrics goroutine is started. Recommended: 45s.
	// Default: 0 (off — a generic package must not spawn surprise goroutines).
	MetricsInterval time.Duration

	// OnMetrics is the periodic metrics callback. It is active only when
	// MetricsInterval > 0 AND this field is non-nil.
	OnMetrics func(MetricsSnapshot)
}

// DefaultOptions returns a configuration optimized for low-resource systems
// (1-2 vCPU, 2-4GB RAM). Event and metrics hooks are disabled by default;
// the caller opts in explicitly.
func DefaultOptions() Options {
	return Options{
		CacheSizeMB:       64,
		MemTableSizeMB:    64,
		MaxOpenFiles:      64,
		CompactionThreads: 0,
		WALDir:            "",
		ReadOnly:          false,
		MetricsInterval:   0,
	}
}

// Normalize fills zero values with defaults and enforces consistency. It is
// called automatically by Open and may also be called by the caller to
// validate a custom Options value.
func (o Options) Normalize() Options {
	d := DefaultOptions()
	if o.CacheSizeMB <= 0 {
		o.CacheSizeMB = d.CacheSizeMB
	}
	if o.MemTableSizeMB <= 0 {
		o.MemTableSizeMB = d.MemTableSizeMB
	}
	if o.MaxOpenFiles <= 0 {
		o.MaxOpenFiles = d.MaxOpenFiles
	}
	if o.CompactionThreads <= 0 {
		o.CompactionThreads = d.CompactionThreads
	}
	return o
}