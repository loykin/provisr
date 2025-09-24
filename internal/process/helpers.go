package process

import (
	"errors"
	"time"
)

func fmtErrorString(s string) error { return errors.New(s) }

func errBeforeStart(d time.Duration) error {
	return fmtErrorString("process exited before start duration " + d.String())
}
