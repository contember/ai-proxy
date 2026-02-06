package llm_resolver

import (
	"sync"
	"time"

	"go.uber.org/zap/zapcore"
)

// LogEntry represents a single captured log entry.
type LogEntry struct {
	Time    time.Time
	Level   string
	Message string
	Fields  map[string]interface{}
}

// LogBuffer is a thread-safe ring buffer for log entries.
type LogBuffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	cap     int
}

// NewLogBuffer creates a new LogBuffer with the given capacity.
func NewLogBuffer(capacity int) *LogBuffer {
	return &LogBuffer{
		entries: make([]LogEntry, 0, capacity),
		cap:     capacity,
	}
}

// Add appends a log entry, evicting the oldest when full.
func (b *LogBuffer) Add(entry LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.entries) >= b.cap {
		b.entries = b.entries[1:]
	}
	b.entries = append(b.entries, entry)
}

// Entries returns a copy of all entries in chronological order.
func (b *LogBuffer) Entries() []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]LogEntry, len(b.entries))
	copy(out, b.entries)
	return out
}

// bufferCore is a zapcore.Core that writes log entries to a LogBuffer.
type bufferCore struct {
	buf    *LogBuffer
	fields []zapcore.Field
}

// NewBufferCore creates a new zapcore.Core that captures entries into the buffer.
func NewBufferCore(buf *LogBuffer) zapcore.Core {
	return &bufferCore{buf: buf}
}

func (c *bufferCore) Enabled(level zapcore.Level) bool {
	return level >= zapcore.InfoLevel
}

func (c *bufferCore) With(fields []zapcore.Field) zapcore.Core {
	clone := &bufferCore{
		buf:    c.buf,
		fields: make([]zapcore.Field, 0, len(c.fields)+len(fields)),
	}
	clone.fields = append(clone.fields, c.fields...)
	clone.fields = append(clone.fields, fields...)
	return clone
}

func (c *bufferCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return ce.AddCore(entry, c)
	}
	return ce
}

func (c *bufferCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	allFields := make([]zapcore.Field, 0, len(c.fields)+len(fields))
	allFields = append(allFields, c.fields...)
	allFields = append(allFields, fields...)

	fieldMap := make(map[string]interface{}, len(allFields))
	enc := zapcore.NewMapObjectEncoder()
	for _, f := range allFields {
		f.AddTo(enc)
	}
	for k, v := range enc.Fields {
		fieldMap[k] = v
	}

	c.buf.Add(LogEntry{
		Time:    entry.Time,
		Level:   entry.Level.String(),
		Message: entry.Message,
		Fields:  fieldMap,
	})
	return nil
}

func (c *bufferCore) Sync() error {
	return nil
}
