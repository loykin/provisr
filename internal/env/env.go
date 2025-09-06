package env

import (
	"os"
	"strings"
)

type Var map[string]string

type Env struct {
	Var Var // global variables (K->V)
	env Var // cached base from OS environment
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
	e.env = base
}

// Set sets a global variable K=V.
func (e *Env) Set(k, v string) {
	if e.Var == nil {
		e.Var = make(Var)
	}
	e.Var[k] = v
}

// Unset removes a global variable.
func (e *Env) Unset(k string) {
	if e.Var != nil {
		delete(e.Var, k)
	}
}

// Merge composes the final environment list applying order:
// base = OS env (or cached)
// then apply global e.Var overrides
// then apply perProc (slice of "K=V") overrides
// Returns the environment slice in "K=V" form, with ${VAR} expansion performed
// using the composed map (simple expansion, no recursion).
func (e *Env) Merge(perProc []string) []string {
	// start from OS or cached
	if e.env == nil {
		e.FromOS()
	}
	m := make(Var)
	for k, v := range e.env {
		m[k] = v
	}
	for k, v := range e.Var {
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
