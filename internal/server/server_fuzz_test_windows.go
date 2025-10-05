//go:build windows

package server

import "testing"

// addPlatformSpecificSeeds adds Windows-specific test seeds for fuzzing
func addPlatformSpecificSeeds(f *testing.F) {
	f.Add("process-name", "C:\\work\\dir")
	f.Add("../bad", "C:\\safe\\path")
}
