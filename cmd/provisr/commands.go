package main

import (
	"strings"

	"github.com/loykin/provisr"
)

type command struct {
	mgr *provisr.Manager
}

// isExpectedShutdownError checks if the error is expected during shutdown
func isExpectedShutdownError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	// Check for common shutdown signals and patterns
	return errStr == "signal: terminated" ||
		errStr == "signal: killed" ||
		errStr == "signal: interrupt" ||
		errStr == "exit status 1" || // Common exit code
		errStr == "exit status 130" || // Ctrl+C
		errStr == "exit status 143" || // SIGTERM
		// Also handle wrapped errors from stop process
		errStr == "failed to stop process: signal: terminated" ||
		errStr == "failed to stop process: signal: killed" ||
		errStr == "failed to stop process: signal: interrupt" ||
		// Handle API error responses that contain shutdown signals
		errStr == "API error: signal: terminated" ||
		errStr == "API error: signal: killed" ||
		errStr == "API error: signal: interrupt" ||
		// Handle nested API error responses
		errStr == "API error: failed to stop process: signal: terminated" ||
		errStr == "API error: failed to stop process: signal: killed" ||
		errStr == "API error: failed to stop process: signal: interrupt" ||
		// Check if error string contains shutdown signals (more flexible)
		strings.Contains(errStr, "signal: terminated") ||
		strings.Contains(errStr, "signal: killed") ||
		strings.Contains(errStr, "signal: interrupt")
}
