//go:build !windows

package server

import "testing"

// addPlatformSpecificSeeds adds Unix-specific test seeds for fuzzing
func addPlatformSpecificSeeds(f *testing.F) {
	f.Add("process-name", "/work/dir")
	f.Add("../bad", "/safe/path")
}
