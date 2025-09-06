package env

import (
	"os"
	"strings"
	"sync"
)

type Var map[string]string

// Env is an immutable environment descriptor with optional lazy base snapshot.
// It is safe for concurrent use. Mutating operations return a new *Env.
// Internally, base and globals must not be mutated after construction.
// A mutex guards lazy initialization of base only.

type Env struct {
	base    Var // immutable snapshot of OS env (set once lazily)
	globals Var // immutable map of global overrides (owned by this Env)
	mu      sync.RWMutex
}

func New() *Env { return &Env{} }

// copyMap returns a shallow copy of m (nil-safe).
func copyMap(m Var) Var {
	if m == nil {
		return nil
	}
	cp := make(Var, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

// ensureBase lazily snapshots the current process environment into e.base exactly once.
func (e *Env) ensureBase() Var {
	e.mu.RLock()
	b := e.base
	e.mu.RUnlock()
	if b != nil {
		return b
	}
	// Build snapshot locally.
	local := make(Var)
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			k := kv[:i]
			v := kv[i+1:]
			if k == "" {
				continue
			}
			local[k] = v
		}
	}
	// Publish snapshot once.
	e.mu.Lock()
	if e.base == nil { // another goroutine may have set it
		e.base = local
	}
	b = e.base
	e.mu.Unlock()
	return b
}

// WithSet returns a new Env with global variable k=v set.
func (e *Env) WithSet(k, v string) *Env {
	if k == "" {
		return e // ignore empty keys
	}
	// Snapshot base pointer; immutable once set.
	b := e.ensureBase()
	ng := copyMap(e.globals)
	if ng == nil {
		ng = make(Var)
	}
	ng[k] = v
	return &Env{base: b, globals: ng}
}

// WithUnset returns a new Env without global variable k.
func (e *Env) WithUnset(k string) *Env {
	b := e.ensureBase()
	ng := copyMap(e.globals)
	if ng != nil {
		delete(ng, k)
	}
	return &Env{base: b, globals: ng}
}

// Merge composes final environment applying order:
// base (OS snapshot) -> globals -> perProc overrides. Returns a fresh []string.
func (e *Env) Merge(perProc []string) []string {
	base := e.ensureBase()
	m := make(Var, len(base)+len(e.globals)+len(perProc))
	// Copy base
	for k, v := range base {
		m[k] = v
	}
	// Apply globals
	for k, v := range e.globals {
		if k == "" {
			continue
		}
		m[k] = v
	}
	// Apply per-process overrides
	for _, kv := range perProc {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			k := kv[:i]
			v := kv[i+1:]
			if k == "" {
				continue
			}
			m[k] = v
		}
	}
	// Expand ${VAR}
	expanded := make(Var, len(m))
	for k, v := range m {
		expanded[k] = expand(v, m)
	}
	// Build slice result (fresh)
	out := make([]string, 0, len(expanded))
	for k, v := range expanded {
		if k == "" {
			continue
		}
		out = append(out, k+"="+v)
	}
	return out
}

func expand(s string, m Var) string {
	res := s
	for k, v := range m {
		res = strings.ReplaceAll(res, "${"+k+"}", v)
	}
	return res
}
