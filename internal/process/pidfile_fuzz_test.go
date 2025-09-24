package process

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzReadPIDFileWithMeta(f *testing.F) {
	f.Add("123\n{}\n{\"start_unix\":1}\n")
	f.Add("0\n")
	f.Add("not-a-pid\n{}\n")
	f.Fuzz(func(t *testing.T, content string) {
		dir := t.TempDir()
		pf := filepath.Join(dir, "fuzz.pid")
		_ = os.WriteFile(pf, []byte(content), 0o600)
		_, _, _, _ = ReadPIDFileWithMeta(pf) // Should never panic
	})
}
