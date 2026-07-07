package process

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogRingBuffer_SinceAndEviction(t *testing.T) {
	b := newLogRingBuffer(3)
	for _, line := range []string{"a", "b", "c", "d", "e"} {
		b.append("stdout", line)
	}

	// capacity 3: only the last 3 lines (c, d, e) should remain.
	all, next := b.since(0, 0)
	if len(all) != 3 {
		t.Fatalf("expected 3 buffered lines after eviction, got %d", len(all))
	}
	texts := []string{all[0].Text, all[1].Text, all[2].Text}
	if strings.Join(texts, ",") != "c,d,e" {
		t.Fatalf("expected [c d e], got %v", texts)
	}
	if next != 5 {
		t.Fatalf("expected next offset 5, got %d", next)
	}

	// since an offset older than everything buffered still returns
	// everything currently available, not an error.
	fromZero, _ := b.since(0, 0)
	if len(fromZero) != 3 {
		t.Fatalf("expected 3 lines when since=0 after eviction, got %d", len(fromZero))
	}

	// since the last seen offset returns only newer lines.
	newer, _ := b.since(all[len(all)-1].Offset+1, 0)
	if len(newer) != 0 {
		t.Fatalf("expected 0 new lines, got %d", len(newer))
	}
}

func TestLogRingBuffer_LimitCaps(t *testing.T) {
	b := newLogRingBuffer(10)
	for i := 0; i < 5; i++ {
		b.append("stdout", "line")
	}
	lines, _ := b.since(0, 2)
	if len(lines) != 2 {
		t.Fatalf("expected limit to cap results at 2, got %d", len(lines))
	}
}

// TestLogRingBuffer_TruncatedNextResumesAfterLastReturnedLine guards against
// a real bug: when limit truncates the result, `next` must point right
// after the last line actually returned, not the buffer's true head —
// otherwise a second poll with since=next would silently skip every line
// between the two, forever.
func TestLogRingBuffer_TruncatedNextResumesAfterLastReturnedLine(t *testing.T) {
	b := newLogRingBuffer(100)
	for i := 0; i < 10; i++ {
		b.append("stdout", "line")
	}

	first, next := b.since(0, 3)
	if len(first) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(first))
	}
	if next != 3 {
		t.Fatalf("expected next=3 (right after offset 2), got %d", next)
	}

	second, next2 := b.since(next, 3)
	if len(second) != 3 || second[0].Offset != 3 {
		t.Fatalf("expected next poll to resume at offset 3, got %+v", second)
	}
	if next2 != 6 {
		t.Fatalf("expected next=6, got %d", next2)
	}
}

func TestLineTeeWriter_SplitsLinesAndPassesThrough(t *testing.T) {
	buf := newLogRingBuffer(10)
	var passed bytes.Buffer
	w := newLineTeeWriter(buf, "stdout", &passed)

	// A single Write call spanning multiple lines, plus a partial line
	// held over to the next Write call.
	_, err := w.Write([]byte("hello\nworld\r\npartial"))
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	lines, _ := buf.since(0, 0)
	if len(lines) != 2 {
		t.Fatalf("expected 2 complete lines buffered, got %d: %+v", len(lines), lines)
	}
	if lines[0].Text != "hello" || lines[1].Text != "world" {
		t.Fatalf("unexpected line contents: %+v", lines)
	}

	if _, err := w.Write([]byte(" line\n")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	lines, _ = buf.since(0, 0)
	if len(lines) != 3 || lines[2].Text != "partial line" {
		t.Fatalf("expected partial line to be completed across writes, got %+v", lines)
	}

	if passed.String() != "hello\nworld\r\npartial line\n" {
		t.Fatalf("expected raw bytes passed through unchanged, got %q", passed.String())
	}
}

func TestLineTeeWriter_NilPassThroughIsSafe(t *testing.T) {
	buf := newLogRingBuffer(10)
	w := newLineTeeWriter(buf, "stderr", nil)
	if _, err := w.Write([]byte("oops\n")); err != nil {
		t.Fatalf("Write() with nil passTo should not error: %v", err)
	}
	lines, _ := buf.since(0, 0)
	if len(lines) != 1 || lines[0].Stream != "stderr" {
		t.Fatalf("unexpected buffered lines: %+v", lines)
	}
}
