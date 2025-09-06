package env

import (
	"os"
	"strings"
	"sync"
)

type Var map[string]string

type Env struct {
	Var Var // global variables (K->V)
	env Var // cached base from OS environment
	// protect env cache and Var from concurrent access
	mu sync.RWMutex
}

func New() *Env {
	return &Env{
		Var: make(Var),
	}
}

// FromOS caches the current process environment as the base.
func (e *Env) FromOS() {
	base := make(Var)
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			k := kv[:i]
			v := kv[i+1:]
			if k == "" {
				continue
			}
			base[k] = v
		}
	}
	e.mu.Lock()
	e.env = base
	e.mu.Unlock()
}

// Set sets a global variable K=V.
func (e *Env) Set(k, v string) {
	e.mu.Lock()
	if e.Var == nil {
		e.Var = make(Var)
	}
	e.Var[k] = v
	e.mu.Unlock()
}

// Unset removes a global variable.
func (e *Env) Unset(k string) {
	e.mu.Lock()
	if e.Var != nil {
		delete(e.Var, k)
	}
	e.mu.Unlock()
}

// Merge composes the final environment list applying order:
// base = OS env (or cached)
// then apply global e.Var overrides
// then apply perProc (slice of "K=V") overrides
// Returns the environment slice in "K=V" form, with ${VAR} expansion performed
// using the composed map (simple expansion, no recursion).
func (e *Env) Merge(perProc []string) []string {
	// Read or initialize cache under lock
	e.mu.RLock()
	envCache := e.env
	globals := e.Var
	e.mu.RUnlock()
	if envCache == nil {
		// initialize cache once
		e.FromOS()
		e.mu.RLock()
		envCache = e.env
		globals = e.Var
		e.mu.RUnlock()
	}
	m := make(Var)
	for k, v := range envCache {
		m[k] = v
	}
	for k, v := range globals {
		if k == "" {
			continue
		}
		m[k] = v
	}
	for _, kv := range perProc {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			k := kv[:i]
			v := kv[i+1:]
			if k == "" { // skip malformed entries with empty key
				continue
			}
			m[k] = v
		}
	}
	// expand ${VAR}
	expanded := make(Var, len(m))
	for k, v := range m {
		expanded[k] = expand(v, m)
	}
	// build slice
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
	// simple ${VAR} expansion; iterate over keys present
	for k, v := range m {
		res = strings.ReplaceAll(res, "${"+k+"}", v)
	}
	return res
}
