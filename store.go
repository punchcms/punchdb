// Package punchdb defines a high-performance, domain-agnostic key-value store
// contract. It transports raw bytes only and has no knowledge of tenants,
// models, indexes, or relations — those concerns belong to higher layers
// (e.g., an ORM).
package punchdb

import (
	"context"
	"errors"
)

// ErrNotFound is returned when the requested key does not exist.
var ErrNotFound = errors.New("punchdb: key not found")

// ErrStopIteration is returned from a Scan callback to terminate the iteration
// early and cleanly. It is not treated as an error; it signals a successful
// early exit (e.g., "page limit reached" in cursor-based pagination).
var ErrStopIteration = errors.New("punchdb: stop iteration")

// Store is the base contract for all key-value backend implementations.
// All methods are safe for concurrent use.
type Store interface {
	// Get returns the value associated with the given key, or ErrNotFound.
	// The returned slice is a safe copy the caller may retain indefinitely.
	Get(ctx context.Context, key []byte) ([]byte, error)

	// Set writes a key-value pair using NoSync (WAL provides durability).
	Set(ctx context.Context, key, value []byte) error

	// Delete removes a key. It is idempotent: deleting a missing key is not
	// an error.
	Delete(ctx context.Context, key []byte) error

	// Scan iterates over all keys in the half-open range [start, end) in
	// sorted order. It is the foundation of cursor-based pagination (O(1)
	// seek). If the callback returns ErrStopIteration, the scan stops cleanly.
	Scan(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error

	// PrefixScan iterates over all keys sharing the given prefix. It is a
	// convenience wrapper around Scan where end = prefix + 0xFF.
	PrefixScan(ctx context.Context, prefix []byte, fn func(key, value []byte) error) error

	// NewBatch starts a new batch for atomic multi-key writes. A Batch is NOT
	// thread-safe; each goroutine must create its own.
	NewBatch() Batch

	// Close releases the database and all associated resources. It is
	// idempotent.
	Close() error
}

// Batch accumulates write operations and applies them atomically on Commit,
// guaranteeing that no partial state is ever persisted.
type Batch interface {
	// Set adds a write operation to the batch.
	Set(key, value []byte) error
	// Delete adds a delete operation to the batch.
	Delete(key []byte) error
	// Commit atomically applies all accumulated operations.
	Commit() error
	// Count returns the number of operations currently in the batch.
	Count() int
	// Close releases the batch resources.
	Close() error
}
