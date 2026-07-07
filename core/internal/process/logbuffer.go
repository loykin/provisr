package process

import (
	"bytes"
	"io"
	"sync"
)

// defaultLogBufferCapacity bounds memory use per process: only the most
// recent N lines are kept, oldest evicted first.
const defaultLogBufferCapacity = 500

// LogLine is a single captured line of stdout/stderr output, exposed to
// the live-tail polling API.
type LogLine struct {
	Offset uint64 `json:"offset"`
	Stream string `json:"stream"` // "stdout" or "stderr"
	Text   string `json:"text"`
}

// logRingBuffer is a fixed-capacity, thread-safe ring buffer of captured
// stdout/stderr lines. It exists so the live-tail API works regardless of
// whether file-based logging is configured for a process.
type logRingBuffer struct {
	mu       sync.Mutex
	lines    []LogLine
	capacity int
	nextOff  uint64
}

func newLogRingBuffer(capacity int) *logRingBuffer {
	if capacity <= 0 {
		capacity = defaultLogBufferCapacity
	}
	return &logRingBuffer{capacity: capacity}
}

func (b *logRingBuffer) append(stream, text string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = append(b.lines, LogLine{Offset: b.nextOff, Stream: stream, Text: text})
	b.nextOff++
	if len(b.lines) > b.capacity {
		b.lines = b.lines[len(b.lines)-b.capacity:]
	}
}

// since returns buffered lines with Offset >= since (oldest first, capped
// at limit if positive) plus the offset to pass as `since` on the next
// poll. If since is older than everything still buffered (evicted), all
// currently buffered lines are returned — the caller can't get lines that
// no longer exist, but won't error either.
func (b *logRingBuffer) since(since uint64, limit int) ([]LogLine, uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	start := 0
	for i, l := range b.lines {
		if l.Offset >= since {
			start = i
			break
		}
		start = i + 1
	}

	lines := b.lines[start:]
	next := b.nextOff
	if limit > 0 && len(lines) > limit {
		lines = lines[:limit]
		// Truncated: the next poll must resume right after the last line
		// actually returned, not at the buffer's true head — otherwise the
		// lines between the two would be skipped forever.
		next = lines[len(lines)-1].Offset + 1
	}

	out := make([]LogLine, len(lines))
	copy(out, lines)
	return out, next
}

// lineTeeWriter splits a byte stream into lines, appending each complete
// line to a logRingBuffer as it arrives, then passes the raw bytes through
// unchanged to an optional underlying writer (e.g. file-based logging).
type lineTeeWriter struct {
	buf    *logRingBuffer
	stream string
	next   []byte
	passTo io.Writer
}

func newLineTeeWriter(buf *logRingBuffer, stream string, passTo io.Writer) *lineTeeWriter {
	return &lineTeeWriter{buf: buf, stream: stream, passTo: passTo}
}

func (w *lineTeeWriter) Write(p []byte) (int, error) {
	w.next = append(w.next, p...)
	for {
		idx := bytes.IndexByte(w.next, '\n')
		if idx < 0 {
			break
		}
		line := string(bytes.TrimRight(w.next[:idx], "\r"))
		w.buf.append(w.stream, line)
		w.next = w.next[idx+1:]
	}

	if w.passTo != nil {
		return w.passTo.Write(p)
	}
	return len(p), nil
}
