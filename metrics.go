package punchdb

// MetricsSnapshot holds a point-in-time summary of database metrics. It is
// collected periodically when MetricsInterval > 0 and delivered to the
// OnMetrics callback. All values are pre-computed; formatting is left to the
// caller.
type MetricsSnapshot struct {
	// DiskMB is the total disk usage in megabytes.
	DiskMB int64
	// CacheHitRatePercent is the block cache hit ratio (0-100).
	CacheHitRatePercent float64
	// TotalFiles is the total number of SST files across all levels.
	TotalFiles int64
	// TotalSizeMB is the total data size across all levels in megabytes.
	TotalSizeMB int64
	// CompactionCount is the total number of completed compactions.
	CompactionCount int64
	// CompactionsInProgress is the number of compactions currently running.
	CompactionsInProgress int64
	// WALSizeMB is the write-ahead log size in megabytes.
	WALSizeMB int64
}