package env

import (
	"strings"
	"testing"
)

// FuzzExpandMerge fuzzes Merge/expand with random inputs to ensure no panics and
// basic invariants around ${VAR} expansion.
func FuzzExpandMerge(f *testing.F) {
	// seeds (packed as bytes; newline-separated)
	f.Add([]byte("A=1\nB=${A}-x"), []byte("C=${B}-y"))
	f.Add([]byte("FOO=bar"), []byte("FOO=${FOO}"))
	f.Add([]byte("X=$Y"), []byte("Y=${X}")) // cyclic-like

	f.Fuzz(func(t *testing.T, globalB []byte, perB []byte) {
		// Decode slices from newline-separated bytes
		global := splitNZ(string(globalB))
		per := splitNZ(string(perB))
		if len(global) > 20 {
			global = global[:20]
		}
		if len(per) > 20 {
			per = per[:20]
		}

		e := New()
		for _, kv := range global {
			if i := strings.IndexByte(kv, '='); i >= 0 {
				e = e.WithSet(kv[:i], kv[i+1:])
			}
		}
		out := e.Merge(per)
		// Invariants:
		// 1) Out must be key=value items without empty keys and with '=' present.
		for _, kv := range out {
			if !strings.Contains(kv, "=") {
				t.Fatalf("bad pair: %q", kv)
			}
			if strings.HasPrefix(kv, "=") {
				t.Fatalf("empty key: %q", kv)
			}
		}
		// 2) Expansion should not introduce raw ${ sequences when inputs are simple ASCII without '$'.
		// Build a condition: if neither global nor per contain '$', output should not contain '${'.
		containsDollar := false
		for _, s := range append(append([]string{}, global...), per...) {
			if strings.ContainsRune(s, '$') {
				containsDollar = true
				break
			}
		}
		if !containsDollar {
			for _, kv := range out {
				if strings.Contains(kv, "${") {
					t.Fatalf("unexpected placeholder remains: %q", kv)
				}
			}
		}
	})
}

// splitNZ splits s by newlines and returns non-empty trimmed lines.
func splitNZ(s string) []string {
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			out = append(out, ln)
		}
	}
	return out
}
