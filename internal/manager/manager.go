package manager

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/loykin/provisr/internal/env"
	"github.com/loykin/provisr/internal/history"
	"github.com/loykin/provisr/internal/metrics"
	"github.com/loykin/provisr/internal/process"
	"github.com/loykin/provisr/internal/store"
)

// Manager starts, stops, and monitors processes.
type Manager struct {
	mu        sync.RWMutex
	envM      *env.Env
	st        store.Store
	reconStop chan struct{}
	histSinks []history.Sink

	// unified per-process entry holding handler/supervisor and their cancels
	entries map[string]*procEntry
}

type procEntry struct {
	h       *handler
	hCancel context.CancelFunc
	s       *supervisor
	sCancel context.CancelFunc
}

func NewManager() *Manager {
	return &Manager{
		entries: make(map[string]*procEntry),
		envM:    env.New(),
	}
}

// SetHistorySinks configures external history sinks (OpenSearch, ClickHouse, etc.).
// Passing nil or no sinks clears the list.
func (m *Manager) SetHistorySinks(sinks ...history.Sink) {
	m.mu.Lock()
	m.histSinks = append([]history.Sink(nil), sinks...)
	m.mu.Unlock()
}

// SetStore configures a persistence store used to record process lifecycle events.
// It ensures the schema and stores the instance for subsequent writes.
func (m *Manager) SetStore(s store.Store) error {
	m.mu.Lock()
	m.st = s
	m.mu.Unlock()
	if s == nil {
		return nil
	}
	return s.EnsureSchema(context.Background())
}

func (m *Manager) recordStart(p *process.Process) {
	m.mu.Lock()
	st := m.st
	sinks := append([]history.Sink(nil), m.histSinks...)
	m.mu.Unlock()
	rs := p.Snapshot()
	rec := store.Record{
		Name:      rs.Name,
		PID:       rs.PID,
		StartedAt: rs.StartedAt,
	}
	if st != nil {
		_ = st.RecordStart(context.Background(), rec)
	}
	if len(sinks) > 0 {
		evt := history.Event{Type: history.EventStart, OccurredAt: time.Now().UTC(), Record: rec}
		for _, s := range sinks {
			_ = s.Send(context.Background(), evt)
		}
	}
}

func (m *Manager) recordStop(p *process.Process, exitErr error) {
	m.mu.Lock()
	st := m.st
	sinks := append([]history.Sink(nil), m.histSinks...)
	m.mu.Unlock()
	rs := p.Snapshot()
	uniq := store.UniqueKey(rs.PID, rs.StartedAt)
	if st != nil {
		_ = st.RecordStop(context.Background(), uniq, rs.StoppedAt, exitErr)
	}
	if len(sinks) > 0 {
		rec := store.Record{
			Name:      rs.Name,
			PID:       rs.PID,
			StartedAt: rs.StartedAt,
			StoppedAt: sql.NullTime{Time: rs.StoppedAt, Valid: !rs.StoppedAt.IsZero()},
			Running:   false,
		}
		if exitErr != nil {
			rec.ExitErr = sql.NullString{String: exitErr.Error(), Valid: true}
		}
		evt := history.Event{Type: history.EventStop, OccurredAt: time.Now().UTC(), Record: rec}
		for _, s := range sinks {
			_ = s.Send(context.Background(), evt)
		}
	}
}

// SetGlobalEnv sets global environment variables affecting all processes managed by this Manager.
// kvs must be in the form "KEY=VALUE".
func (m *Manager) SetGlobalEnv(kvs []string) {
	if m.envM == nil {
		m.envM = env.New()
	}
	e := m.envM
	for _, kv := range kvs {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			k := kv[:i]
			v := kv[i+1:]
			e = e.WithSet(k, v)
		}
	}
	m.envM = e
}

func (m *Manager) Start(spec process.Spec) error {
	// Inject merged env into spec before passing to handler
	spec.Env = m.mergedEnvFor(spec)
	m.ensureHandler(spec)
	h := m.getHandler(spec.Name)
	if h == nil {
		return fmt.Errorf("failed to ensure handler for %s", spec.Name)
	}
	attempts, interval := retryParams(spec)
	var lastErr error
	for i := 0; i <= attempts; i++ {
		reply := make(chan error, 1)
		h.ctrl <- CtrlMsg{Type: CtrlStart, Spec: spec, Reply: reply}
		err := <-reply
		if err == nil {
			// ensure supervisor is running for this process; observability handled by supervisor
			m.ensureSupervisor(spec.Name)
			return nil
		}
		lastErr = err
		if i < attempts {
			if !process.IsBeforeStartErr(err) {
				time.Sleep(interval)
			}
		}
	}
	return lastErr
}

// Stop stops a running process. If already stopped, it's a no-op.
func (m *Manager) Stop(name string, wait time.Duration) error {
	h := m.getHandler(name)
	if h == nil {
		return fmt.Errorf("unknown process: %s", name)
	}
	reply := make(chan error, 1)
	h.ctrl <- CtrlMsg{Type: CtrlStop, Wait: wait, Reply: reply}
	err := <-reply
	// stop supervisor if running
	m.mu.Lock()
	if e := m.entries[name]; e != nil {
		if e.sCancel != nil {
			e.sCancel()
			e.sCancel = nil
		}
		e.s = nil
	}
	m.mu.Unlock()
	return err
}

// Status returns current status including detector check.
func (m *Manager) Status(name string) (process.Status, error) {
	h := m.getHandler(name)
	if h == nil {
		return process.Status{}, fmt.Errorf("unknown process: %s", name)
	}
	return h.Status(), nil
}

// StartN starts Spec.Instances instances by suffixing names with -1..-N.
func (m *Manager) StartN(spec process.Spec) error {
	n := spec.Instances
	if n <= 1 {
		return m.Start(spec)
	}
	for i := 1; i <= n; i++ {
		inst := spec
		inst.Name = fmt.Sprintf("%s-%d", spec.Name, i)
		if err := m.Start(inst); err != nil {
			return err
		}
	}
	// update gauge for base name
	if c, err := m.Count(spec.Name); err == nil {
		metrics.SetRunningInstances(spec.Name, c)
	}
	return nil
}

// StopAll stops all instances with the base name.
func (m *Manager) StopAll(base string, wait time.Duration) error {
	m.mu.RLock()
	names := make([]string, 0, len(m.entries))
	for name := range m.entries {
		if name == base || strings.HasPrefix(name, base+"-") {
			names = append(names, name)
		}
	}
	m.mu.RUnlock()
	var firstErr error
	for _, name := range names {
		if err := m.Stop(name, wait); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if c, err := m.Count(base); err == nil {
		metrics.SetRunningInstances(base, c)
	}
	return firstErr
}

// StatusAll returns statuses for all instances matching the base name.
func (m *Manager) StatusAll(base string) ([]process.Status, error) {
	m.mu.RLock()
	names := make([]string, 0, len(m.entries))
	for name := range m.entries {
		if name == base || strings.HasPrefix(name, base+"-") {
			names = append(names, name)
		}
	}
	m.mu.RUnlock()
	res := make([]process.Status, 0, len(names))
	for _, n := range names {
		st, err := m.Status(n)
		if err != nil {
			return nil, err
		}
		res = append(res, st)
	}
	return res, nil
}

// StatusMatch returns statuses for all process names that match the wildcard pattern.
// Supported wildcard: '*' matches any substring (including empty). Multiple '*' are allowed.
func (m *Manager) StatusMatch(pattern string) ([]process.Status, error) {
	m.mu.RLock()
	names := make([]string, 0, len(m.entries))
	for name := range m.entries {
		if wildcardMatch(name, pattern) {
			names = append(names, name)
		}
	}
	m.mu.RUnlock()
	res := make([]process.Status, 0, len(names))
	for _, n := range names {
		st, err := m.Status(n)
		if err != nil {
			return nil, err
		}
		res = append(res, st)
	}
	return res, nil
}

// StopMatch stops all processes with names that match the wildcard pattern.
// Returns the first error encountered, if any.
func (m *Manager) StopMatch(pattern string, wait time.Duration) error {
	m.mu.RLock()
	names := make([]string, 0, len(m.entries))
	for name := range m.entries {
		if wildcardMatch(name, pattern) {
			names = append(names, name)
		}
	}
	m.mu.RUnlock()
	var firstErr error
	for _, name := range names {
		if err := m.Stop(name, wait); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Count returns number of running instances for the base name.
func (m *Manager) Count(base string) (int, error) {
	sts, err := m.StatusAll(base)
	if err != nil {
		return 0, err
	}
	c := 0
	for _, st := range sts {
		if st.Running {
			c++
		}
	}
	return c, nil
}

// --- helpers extracted to reduce cyclomatic complexity in Start() ---

// getHandler returns the handler for a process name.
func (m *Manager) getHandler(name string) *handler {
	m.mu.RLock()
	e := m.entries[name]
	m.mu.RUnlock()
	if e != nil {
		return e.h
	}
	return nil
}

// ensureHandler creates and runs a handler for the given spec name if missing.
// It also updates the handler's spec if it already exists.
func (m *Manager) ensureHandler(spec process.Spec) *handler {
	m.mu.Lock()
	e := m.entries[spec.Name]
	if e == nil {
		// create new handler with injected env merge and history callbacks
		h := newHandler(spec, m.mergedEnvFor, m.recordStart, m.recordStop)
		ctx, cancel := context.WithCancel(context.Background())
		e = &procEntry{h: h, hCancel: cancel}
		m.entries[spec.Name] = e
		go h.run(ctx)
	} else {
		// update spec via control channel synchronously
		reply := make(chan error, 1)
		e.h.ctrl <- CtrlMsg{Type: CtrlUpdateSpec, Spec: spec, Reply: reply}
		<-reply
	}
	m.mu.Unlock()
	return e.h
}

// wildcardMatch matches name against a pattern with '*' wildcard (glob-like, case-sensitive).
// It returns true if the sequence of non-* segments appear in order in name.
func wildcardMatch(name, pattern string) bool {
	if pattern == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	// fast path: no '*'
	if !strings.Contains(pattern, "*") {
		return name == pattern
	}
	parts := strings.Split(pattern, "*")
	// Handle anchors based on leading/trailing '*'
	idx := 0
	// Leading part must match prefix if pattern doesn't start with '*'
	if parts[0] != "" {
		if !strings.HasPrefix(name, parts[0]) {
			return false
		}
		idx = len(parts[0])
	}
	// Middle parts must occur in order
	for i := 1; i < len(parts)-1; i++ {
		p := parts[i]
		if p == "" {
			continue
		}
		j := strings.Index(name[idx:], p)
		if j < 0 {
			return false
		}
		idx += j + len(p)
	}
	// Trailing part must match suffix if pattern doesn't end with '*'
	last := parts[len(parts)-1]
	if last != "" {
		return strings.HasSuffix(name, last) && idx <= len(name)-len(last)
	}
	return true
}

// retryParams computes attempts and interval from the spec.
func retryParams(spec process.Spec) (int, time.Duration) {
	attempts := spec.RetryCount
	if attempts < 0 {
		attempts = 0
	}
	interval := spec.RetryInterval
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	return attempts, interval
}

// mergedEnvFor merges manager globals with per-process env.
func (m *Manager) mergedEnvFor(spec process.Spec) []string {
	if m.envM != nil {
		return m.envM.Merge(spec.Env)
	}
	return nil
}

// ReconcileOnce checks current managed processes against reality and updates the store.
// It also attempts to auto-start processes that should be running but are not by sending control messages.
func (m *Manager) ReconcileOnce() {
	m.mu.Lock()
	st := m.st
	handlers := make([]*handler, 0, len(m.entries))
	for _, e := range m.entries {
		if e != nil && e.h != nil {
			handlers = append(handlers, e.h)
		}
	}
	m.mu.Unlock()
	if len(handlers) == 0 && st == nil {
		return
	}
	ctx := context.Background()
	for _, h := range handlers {
		stSnap := h.Status()
		rs := h.Snapshot()
		if st != nil {
			rec := store.Record{
				Name:      rs.Name,
				PID:       rs.PID,
				StartedAt: rs.StartedAt,
				Running:   stSnap.Running,
			}
			if !stSnap.Running && !rs.StoppedAt.IsZero() {
				rec.StoppedAt = sql.NullTime{Time: rs.StoppedAt, Valid: true}
			}
			if rs.ExitErr != nil {
				rec.ExitErr = sql.NullString{String: rs.ExitErr.Error(), Valid: true}
			}
			_ = st.UpsertStatus(ctx, rec)
			// If we still think it was running (internal) but detector says not, record a stop.
			if !stSnap.Running && rs.Running {
				uniq := store.UniqueKey(rs.PID, rs.StartedAt)
				_ = st.RecordStop(ctx, uniq, time.Now().UTC(), fmt.Errorf("lost (reconciler)"))
			}
		}
		// Auto-start safety net: only when no supervisor is present (supervisor owns policies/starts)
		spec := h.Spec()
		if !stSnap.Running && spec.AutoRestart && !h.StopRequested() && m.getSupervisor(spec.Name) == nil {
			reply := make(chan error, 1)
			h.ctrl <- CtrlMsg{Type: CtrlStart, Spec: spec, Reply: reply}
			if err := <-reply; err == nil {
				// Ensure supervisor exists to monitor subsequent exits; observability handled there
				m.ensureSupervisor(spec.Name)
			}
		}
	}
}

// StartReconciler starts a background loop that periodically calls ReconcileOnce.
func (m *Manager) StartReconciler(interval time.Duration) {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	m.mu.Lock()
	if m.reconStop != nil {
		m.mu.Unlock()
		return // already running
	}
	stop := make(chan struct{})
	m.reconStop = stop
	m.mu.Unlock()
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				m.ReconcileOnce()
			case <-stop:
				return
			}
		}
	}()
}

// StopReconciler stops the background reconcile loop if running.
func (m *Manager) StopReconciler() {
	m.mu.Lock()
	ch := m.reconStop
	m.reconStop = nil
	m.mu.Unlock()
	if ch != nil {
		close(ch)
	}
}

// Shutdown stops reconciler and gracefully shuts down all handlers by sending CtrlShutdown
// and canceling their contexts to avoid goroutine leaks.
func (m *Manager) Shutdown() {
	// stop reconciler first to avoid new auto-starts during shutdown
	m.StopReconciler()
	m.mu.Lock()
	entries := make(map[string]*procEntry, len(m.entries))
	for name, e := range m.entries {
		entries[name] = e
	}
	m.mu.Unlock()
	// cancel all supervisors first
	for _, e := range entries {
		if e != nil && e.sCancel != nil {
			e.sCancel()
			e.sCancel = nil
		}
	}
	// then send shutdown to each handler and cancel its context
	var wg sync.WaitGroup
	for _, e := range entries {
		if e == nil || e.h == nil {
			continue
		}
		reply := make(chan error, 1)
		select {
		case e.h.ctrl <- CtrlMsg{Type: CtrlShutdown, Reply: reply}:
			// sent
		default:
			// if channel is full, still attempt cancel to unblock run
		}
		if e.hCancel != nil {
			e.hCancel()
		}
		wg.Add(1)
		go func(r <-chan error) {
			defer wg.Done()
			select {
			case <-r:
			case <-time.After(2 * time.Second):
				// timeout; best-effort
			}
		}(reply)
	}
	wg.Wait()
}

// getSupervisor returns the supervisor for a process name.
func (m *Manager) getSupervisor(name string) *supervisor {
	m.mu.RLock()
	var s *supervisor
	if e := m.entries[name]; e != nil {
		s = e.s
	}
	m.mu.RUnlock()
	return s
}

// ensureSupervisor creates and runs a supervisor for the given process name if missing.
func (m *Manager) ensureSupervisor(name string) *supervisor {
	m.mu.Lock()
	e := m.entries[name]
	if e == nil {
		m.mu.Unlock()
		return nil
	}
	s := e.s
	if s == nil && e.h != nil {
		ctx, cancel := context.WithCancel(context.Background())
		s = newSupervisor(ctx, e.h, m.recordStart, m.recordStop)
		e.s = s
		e.sCancel = cancel
		go s.Run()
	}
	m.mu.Unlock()
	return s
}
