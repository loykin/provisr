package detector

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzPIDFileDetectorContent ensures PIDFileDetector.Alive does not panic
// on arbitrary file contents and various sizes.
func FuzzPIDFileDetectorContent(f *testing.F) {
	// Seed with valid and invalid examples
	f.Add([]byte("123\n"))
	f.Add([]byte("not-a-number"))
	f.Add([]byte("\n\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		pf := filepath.Join(dir, "pid.pid")
		_ = os.WriteFile(pf, data, 0o644)
		d := PIDFileDetector{PIDFile: pf}
		_, _ = d.Alive() // must not panic
	})
}
