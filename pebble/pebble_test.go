package pebble

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/punchcms/punchdb"
)

// openTestStore opens an isolated Pebble store in a temporary directory and
// registers cleanup. Each test gets its own database.
func openTestStore(t *testing.T, opts punchdb.Options) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(dir, opts)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// openBenchStore opens an isolated Pebble store for benchmarks.
func openBenchStore(b *testing.B) *Store {
	b.Helper()
	dir := b.TempDir()
	s, err := Open(dir, punchdb.DefaultOptions())
	if err != nil {
		b.Fatalf("open store: %v", err)
	}
	b.Cleanup(func() { s.Close() })
	return s
}

// -----------------------------------------------------------------------------
// CRUD
// -----------------------------------------------------------------------------

func TestSetGet(t *testing.T) {
	s := openTestStore(t, punchdb.DefaultOptions())
	ctx := context.Background()

	key := []byte("punch:d:post:1")
	val := []byte("hello world")

	if err := s.Set(ctx, key, val); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := s.Get(ctx, key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !bytes.Equal(got, val) {
		t.Fatalf("get = %q, want %q", got, val)
	}
}

func TestGetReturnsSafeCopy(t *testing.T) {
	s := openTestStore(t, punchdb.DefaultOptions())
	ctx := context.Background()

	key := []byte("k")
	if err := s.Set(ctx, key, []byte("original")); err != nil {
		t.Fatalf("set: %v", err)
	}

	// Mutating the returned slice must not affect the stored value.
	got, _ := s.Get(ctx, key)
	got[0] = 'X'

	again, _ := s.Get(ctx, key)
	if string(again) != "original" {
		t.Fatalf("stored value mutated through returned slice: got %q", again)
	}
}

func TestGetNotFound(t *testing.T) {
	s := openTestStore(t, punchdb.DefaultOptions())
	_, err := s.Get(context.Background(), []byte("missing"))
	if !errors.Is(err, punchdb.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	s := openTestStore(t, punchdb.DefaultOptions())
	ctx := context.Background()

	key := []byte("k")
	if err := s.Set(ctx, key, []byte("v")); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, key); !errors.Is(err, punchdb.ErrNotFound) {
		t.Fatalf("after delete, err = %v, want ErrNotFound", err)
	}
}

func TestDeleteMissingKeyIsIdempotent(t *testing.T) {
	s := openTestStore(t, punchdb.DefaultOptions())
	if err := s.Delete(context.Background(), []byte("never-existed")); err != nil {
		t.Fatalf("delete missing key should not error, got %v", err)
	}
}

// -----------------------------------------------------------------------------
// Batch
// -----------------------------------------------------------------------------

func TestBatchCommit(t *testing.T) {
	s := openTestStore(t, punchdb.DefaultOptions())
	ctx := context.Background()

	b := s.NewBatch()
	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("key:%d", i))
		val := []byte(fmt.Sprintf("val:%d", i))
		if err := b.Set(key, val); err != nil {
			t.Fatalf("batch set: %v", err)
		}
	}
	if err := b.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	b.Close()

	for i := 0; i < 10; i++ {
		got, err := s.Get(ctx, []byte(fmt.Sprintf("key:%d", i)))
		if err != nil {
			t.Fatalf("get key:%d: %v", i, err)
		}
		if want := fmt.Sprintf("val:%d", i); string(got) != want {
			t.Fatalf("key:%d = %q, want %q", i, got, want)
		}
	}
}

func TestBatchCount(t *testing.T) {
	s := openTestStore(t, punchdb.DefaultOptions())
	b := s.NewBatch()
	defer b.Close()

	b.Set([]byte("a"), []byte("1"))
	b.Set([]byte("b"), []byte("2"))
	b.Delete([]byte("c"))

	if b.Count() != 3 {
		t.Fatalf("count = %d, want 3", b.Count())
	}
}

func TestBatchDelete(t *testing.T) {
	s := openTestStore(t, punchdb.DefaultOptions())
	ctx := context.Background()

	if err := s.Set(ctx, []byte("x"), []byte("1")); err != nil {
		t.Fatalf("set: %v", err)
	}

	b := s.NewBatch()
	b.Delete([]byte("x"))
	b.Set([]byte("y"), []byte("2"))
	if err := b.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	b.Close()

	if _, err := s.Get(ctx, []byte("x")); !errors.Is(err, punchdb.ErrNotFound) {
		t.Fatalf("x should be deleted, got err %v", err)
	}
	if _, err := s.Get(ctx, []byte("y")); err != nil {
		t.Fatalf("y should exist: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Scan & PrefixScan
// -----------------------------------------------------------------------------

func TestScanRange(t *testing.T) {
	s := openTestStore(t, punchdb.DefaultOptions())
	ctx := context.Background()

	for _, k := range []string{"a", "b", "c", "d", "e"} {
		if err := s.Set(ctx, []byte(k), []byte(k+"-val")); err != nil {
			t.Fatalf("set %s: %v", k, err)
		}
	}

	// Half-open range [b, d) must yield b and c only.
	var got []string
	err := s.Scan(ctx, []byte("b"), []byte("d"), func(key, value []byte) error {
		got = append(got, string(key))
		return nil
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if want := []string{"b", "c"}; !slices.Equal(got, want) {
		t.Fatalf("scan = %v, want %v", got, want)
	}
}

func TestScanStopIteration(t *testing.T) {
	s := openTestStore(t, punchdb.DefaultOptions())
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("k%02d", i))
		if err := s.Set(ctx, key, []byte("v")); err != nil {
			t.Fatalf("set: %v", err)
		}
	}

	count := 0
	err := s.Scan(ctx, []byte("k"), []byte("k\xFF"), func(key, value []byte) error {
		count++
		if count == 3 {
			return punchdb.ErrStopIteration
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan should stop cleanly, got err %v", err)
	}
	if count != 3 {
		t.Fatalf("count = %d, want 3 (early stop)", count)
	}
}

// TestPrefixScan verifies prefix matching AND tenant/type isolation, which is
// the foundation of the multi-tenant key layout ({tenant}:d:{type}:{id}).
func TestPrefixScan(t *testing.T) {
	s := openTestStore(t, punchdb.DefaultOptions())
	ctx := context.Background()

	keys := []string{
		"punch:d:post:1",
		"punch:d:post:2",
		"punch:d:page:1",    // same tenant, different type — must be excluded
		"punch:d:postfix:1", // shares "punch:d:post" but not "punch:d:post:" — excluded
		"other:d:post:1",    // different tenant — must be excluded
	}
	for _, k := range keys {
		if err := s.Set(ctx, []byte(k), []byte("v")); err != nil {
			t.Fatalf("set %s: %v", k, err)
		}
	}

	var got []string
	err := s.PrefixScan(ctx, []byte("punch:d:post:"), func(key, value []byte) error {
		got = append(got, string(key))
		return nil
	})
	if err != nil {
		t.Fatalf("prefix scan: %v", err)
	}
	if want := []string{"punch:d:post:1", "punch:d:post:2"}; !slices.Equal(got, want) {
		t.Fatalf("prefix scan = %v, want %v", got, want)
	}
}

// -----------------------------------------------------------------------------
// Lifecycle
// -----------------------------------------------------------------------------

func TestCloseIdempotent(t *testing.T) {
	s, err := Open(t.TempDir(), punchdb.DefaultOptions())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second close should not error: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Events & Metrics
// -----------------------------------------------------------------------------

func TestEventListenerNilWithoutCallback(t *testing.T) {
	s := &Store{opts: punchdb.DefaultOptions()}
	if s.eventListener() != nil {
		t.Fatal("eventListener should be nil when OnEvent is unset")
	}
}

func TestEventListenerSetWithCallback(t *testing.T) {
	opts := punchdb.DefaultOptions()
	opts.OnEvent = func(punchdb.Event) {}
	s := &Store{opts: opts}
	if s.eventListener() == nil {
		t.Fatal("eventListener should be non-nil when OnEvent is set")
	}
}

func TestOnEventReceivesWALCreated(t *testing.T) {
	events := make(chan punchdb.Event, 16)
	opts := punchdb.DefaultOptions()
	opts.OnEvent = func(e punchdb.Event) { events <- e }

	s, err := Open(t.TempDir(), opts)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	// A WAL is created synchronously during Open.
	select {
	case e := <-events:
		if e.Type != punchdb.EventWALCreated {
			t.Fatalf("first event = %s, want WAL_CREATED", e.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WALCreated event")
	}
}

func TestMetricsSnapshotFields(t *testing.T) {
	s := openTestStore(t, punchdb.DefaultOptions())
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		key := []byte(fmt.Sprintf("k%d", i))
		if err := s.Set(ctx, key, bytes.Repeat([]byte("x"), 256)); err != nil {
			t.Fatalf("set: %v", err)
		}
	}

	snap := s.snapshot()
	if snap.CacheHitRatePercent < 0 || snap.CacheHitRatePercent > 100 {
		t.Fatalf("cache hit rate out of range: %f", snap.CacheHitRatePercent)
	}
	if snap.TotalFiles < 0 || snap.TotalSizeMB < 0 || snap.WALSizeMB < 0 {
		t.Fatalf("negative metric value: %+v", snap)
	}
}

func TestOnMetricsDelivered(t *testing.T) {
	received := make(chan punchdb.MetricsSnapshot, 1)
	opts := punchdb.DefaultOptions()
	opts.MetricsInterval = 20 * time.Millisecond
	opts.OnMetrics = func(m punchdb.MetricsSnapshot) {
		select {
		case received <- m:
		default:
		}
	}

	s, err := Open(t.TempDir(), opts)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	select {
	case <-received:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for metrics snapshot")
	}
}

// -----------------------------------------------------------------------------
// Checkpoint (backup foundation)
// -----------------------------------------------------------------------------

func TestCheckpoint(t *testing.T) {
	s := openTestStore(t, punchdb.DefaultOptions())
	ctx := context.Background()

	if err := s.Set(ctx, []byte("k"), []byte("v")); err != nil {
		t.Fatalf("set: %v", err)
	}

	cpDir := filepath.Join(t.TempDir(), "checkpoint")
	if err := s.Checkpoint(cpDir); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}

	entries, err := os.ReadDir(cpDir)
	if err != nil {
		t.Fatalf("read checkpoint dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("checkpoint dir is empty")
	}
}

// TestCheckpointRestoresData opens the checkpoint as a separate read-only
// store and verifies the data survived — the core guarantee of the backup
// system.
func TestCheckpointRestoresData(t *testing.T) {
	s := openTestStore(t, punchdb.DefaultOptions())
	ctx := context.Background()

	if err := s.Set(ctx, []byte("k"), []byte("v")); err != nil {
		t.Fatalf("set: %v", err)
	}

	cpDir := filepath.Join(t.TempDir(), "checkpoint")
	if err := s.Checkpoint(cpDir); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}

	// Open the checkpoint read-only. WALDir is intentionally left empty so
	// Pebble follows the checkpoint's own manifest layout.
	restored, err := Open(cpDir, punchdb.Options{ReadOnly: true})
	if err != nil {
		t.Fatalf("open checkpoint: %v", err)
	}
	defer restored.Close()

	got, err := restored.Get(ctx, []byte("k"))
	if err != nil {
		t.Fatalf("get from checkpoint: %v", err)
	}
	if string(got) != "v" {
		t.Fatalf("restored = %q, want %q", got, "v")
	}
}

// -----------------------------------------------------------------------------
// Benchmarks (zero-allocation claims)
// -----------------------------------------------------------------------------

func BenchmarkSet(b *testing.B) {
	s := openBenchStore(b)
	ctx := context.Background()
	val := []byte("benchmark-value")
	key := make([]byte, 0, 32)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key = key[:0]
		key = append(key, "bench:"...)
		key = strconv.AppendInt(key, int64(i), 10)
		if err := s.Set(ctx, key, val); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGet(b *testing.B) {
	s := openBenchStore(b)
	ctx := context.Background()

	key := []byte("bench:key")
	val := bytes.Repeat([]byte("x"), 128)
	if err := s.Set(ctx, key, val); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.Get(ctx, key); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPrefixScan simulates a paginated read: scan a prefix and stop after
// 10 records via ErrStopIteration.
func BenchmarkPrefixScan(b *testing.B) {
	s := openBenchStore(b)
	ctx := context.Background()

	for i := 0; i < 1000; i++ {
		key := []byte(fmt.Sprintf("punch:d:post:%04d", i))
		if err := s.Set(ctx, key, []byte("v")); err != nil {
			b.Fatal(err)
		}
	}
	prefix := []byte("punch:d:post:")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n := 0
		s.PrefixScan(ctx, prefix, func(key, value []byte) error {
			n++
			if n >= 10 {
				return punchdb.ErrStopIteration
			}
			return nil
		})
	}
}
