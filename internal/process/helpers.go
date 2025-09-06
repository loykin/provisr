package process

import (
	"errors"
	"strings"
	"time"
)

func fmtErrorString(s string) error { return errors.New(s) }

func errBeforeStart(d time.Duration) error {
	return fmtErrorString("process exited before start duration " + d.String())
}

// IsBeforeStartErr reports whether the error indicates the process exited before start duration elapsed.
func IsBeforeStartErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "exited before start duration")
}
