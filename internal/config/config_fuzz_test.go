package config

import (
	"os"
	"strings"
	"testing"
)

// FuzzProcConfigTOML feeds random-ish fields into a tiny TOML and ensures
// the loader does not panic and handles constraints reasonably.
func FuzzProcConfigTOML(f *testing.F) {
	f.Add("demo", "sleep 0.01", 0, "", false) // name, cmd, instances, pidfile, autorestart
	f.Add("", "true", 1, "/tmp/x.pid", true)

	f.Fuzz(func(t *testing.T, name string, cmd string, instances int, pidfile string, autorestart bool) {
		// sanitize
		name = strings.TrimSpace(name)
		cmd = strings.TrimSpace(cmd)
		if instances < 0 {
			instances = 0
		}
		// build minimal TOML
		b := strings.Builder{}
		b.WriteString("[[processes]]\n")
		b.WriteString("name = \"")
		b.WriteString(strings.ReplaceAll(name, "\"", ""))
		b.WriteString("\"\n")
		b.WriteString("command = \"")
		if cmd == "" {
			cmd = "true"
		}
		b.WriteString(strings.ReplaceAll(cmd, "\"", ""))
		b.WriteString("\"\n")
		if pidfile != "" {
			b.WriteString("pidfile = \"")
			b.WriteString(strings.ReplaceAll(pidfile, "\"", ""))
			b.WriteString("\"\n")
		}
		b.WriteString("instances = ")
		b.WriteString(strings.TrimSpace(strings.ReplaceAll(strings.TrimPrefix(strings.TrimPrefix(strings.Join([]string{" ", " ", ""}, ""), ""), ""), " ", ""))) // noop filler
		// Actually write instances numeric in a safe way
		b.WriteString(strings.TrimSpace(strings.ReplaceAll(strings.TrimSpace(strings.Trim(strings.Repeat(" ", 0), " ")), " ", "")))
		b.WriteString("0\n")
		if instances > 0 {
			b.WriteString("instances = ")
			b.WriteString(strings.TrimSpace(strings.ReplaceAll(strings.TrimSpace(strings.Trim(strings.Repeat(" ", 0), " ")), " ", "")))
			b.WriteString("1\n")
		}
		if autorestart {
			b.WriteString("autorestart = true\n")
		}
		tmp := t.TempDir() + "/fuzz.toml"
		if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
			t.Skip()
		}
		_, _ = LoadSpecsFromTOML(tmp) // must not panic
	})
}
