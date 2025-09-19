package logger

import (
	"context"
	"io"
	"log/slog"
)

// ColorTextHandler wraps slog.TextHandler to add ANSI color codes for different log levels
type ColorTextHandler struct {
	*slog.TextHandler
	showTime bool
}

// NewColorTextHandler creates a new ColorTextHandler
func NewColorTextHandler(w io.Writer, opts *slog.HandlerOptions, showTime bool) *ColorTextHandler {
	return &ColorTextHandler{
		TextHandler: slog.NewTextHandler(w, opts),
		showTime:    showTime,
	}
}

// Handle implements slog.Handler
func (h *ColorTextHandler) Handle(ctx context.Context, r slog.Record) error {
	// Add color based on level
	var colorCode string
	switch r.Level {
	case slog.LevelDebug:
		colorCode = "\033[36m" // Cyan
	case slog.LevelInfo:
		colorCode = "\033[32m" // Green
	case slog.LevelWarn:
		colorCode = "\033[33m" // Yellow
	case slog.LevelError:
		colorCode = "\033[31m" // Red
	default:
		colorCode = "\033[0m" // Reset/default
	}

	// Modify the message to include color
	originalMsg := r.Message
	r.Message = colorCode + r.Level.String() + "\033[0m  " + originalMsg

	return h.TextHandler.Handle(ctx, r)
}
