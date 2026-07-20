package punchdb

// EventType categorizes a database event delivered via the OnEvent callback.
type EventType uint8

const (
	// EventBackgroundError indicates a critical background error.
	EventBackgroundError EventType = iota
	// EventWriteStallBegin indicates that writes have temporarily stalled.
	EventWriteStallBegin
	// EventWriteStallEnd indicates that a write stall has ended.
	EventWriteStallEnd
	// EventCompactionBegin indicates that a compaction has started.
	EventCompactionBegin
	// EventCompactionEnd indicates that a compaction has completed.
	EventCompactionEnd
	// EventWALCreated indicates that a new WAL file was created.
	EventWALCreated
)

// eventNames is an O(1) lookup table for EventType names.
var eventNames = [6]string{
	"BACKGROUND_ERROR",
	"WRITE_STALL_BEGIN",
	"WRITE_STALL_END",
	"COMPACTION_BEGIN",
	"COMPACTION_END",
	"WAL_CREATED",
}

// String returns the human-readable name of the event type.
func (t EventType) String() string {
	if t < EventType(len(eventNames)) {
		return eventNames[t]
	}
	return "UNKNOWN"
}

// Event represents a structured database event delivered to the OnEvent
// callback. It is NOT part of the hot path; it is emitted from background
// goroutines, so forwarding it to a logger is safe.
type Event struct {
	// Type is the event category.
	Type EventType
	// Message is a short human-readable description.
	Message string
	// Fields holds alternating key-value pairs (compatible with loggerj fields).
	Fields []string
}