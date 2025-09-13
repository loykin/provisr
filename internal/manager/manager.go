package manager

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/loykin/provisr/internal/env"
	"github.com/loykin/provisr/internal/history"
	"github.com/loykin/provisr/internal/metrics"
	"github.com/loykin/provisr/internal/process"
	"github.com/loykin/provisr/internal/store"
)

// Manager starts, stops, and monitors processes.
type Manager struct {
	mu        sync.Mutex
	procs     map[string]*entry
	envM      *env.Env
	st        store.Store
	reconStop chan struct{}
	histSinks []history.Sink
}

type entry struct {
	r    *process.Process
	spec process.Spec
}

// monitor waits for process exit and auto-restarts if configured.
func (m *Manager) monitor(e *entry) {
	for {
		cmd := e.r.CopyCmd()
		if cmd == nil {
			e.r.MonitoringStop()
			return
		}
		// Wait for process to exit
		err := cmd.Wait()
		// notify waiters and mark status
		e.r.CloseWaitDone()
		e.r.MarkExited(err)
		// persist stop in store (best-effort)
		m.recordStop(e, err)
		stopping := e.r.StopRequested()
		autoRestart := e.spec.AutoRestart
		interval2 := e.spec.RestartInterval
		if interval2 <= 0 {
			interval2 = 1 * time.Second
		}
		if stopping || !autoRestart {
			e.r.MonitoringStop()
			return
		}
		// attempt restart
		time.Sleep(interval2)
		_ = e.r.IncRestarts()
		metrics.IncRestart(e.spec.Name)
		_ = m.Start(e.spec)
		// loop continues to wait on new cmd
	}
}

func NewManager() *Manager { return &Manager{procs: make(map[string]*entry), envM: env.New()} }

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

// SetStoreHistoryEnabled toggles append-only history recording inside the configured store.
func (m *Manager) SetStoreHistoryEnabled(enabled bool) {
	// Deprecated: history persistence is handled via history sinks now.
	// This method is kept for backward compatibility and is a no-op.
}

func (m *Manager) recordStart(e *entry) {
	m.mu.Lock()
	st := m.st
	sinks := append([]history.Sink(nil), m.histSinks...)
	m.mu.Unlock()
	rs := e.r.Snapshot()
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

func (m *Manager) recordStop(e *entry, exitErr error) {
	m.mu.Lock()
	st := m.st
	sinks := append([]history.Sink(nil), m.histSinks...)
	m.mu.Unlock()
	rs := e.r.Snapshot()
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

func (m *Manager) get(name string) *entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.procs[name]
}

func (m *Manager) Start(spec process.Spec) error {
	e := m.getOrCreateEntry(spec)
	// Fast path: if already alive, no-op
	if m.fastAlive(e) {
		return nil
	}
	attempts, interval := retryParams(spec)
	var lastErr error
	for i := 0; i <= attempts; i++ {
		if err := m.tryStartOnce(e, spec); err != nil {
			lastErr = err
			if i < attempts {
				time.Sleep(interval)
				continue
			}
			return lastErr
		}
		// persist start in store (best-effort)
		m.recordStart(e)
		if err := m.postStart(e, spec); err != nil {
			lastErr = err
			if i < attempts {
				if !process.IsBeforeStartErr(err) {
					time.Sleep(interval)
				}
				continue
			}
			return lastErr
		}
		return nil
	}
	return lastErr
}

// Stop stops a running process. If already stopped, it's a no-op.
func (m *Manager) Stop(name string, wait time.Duration) error {
	e := m.get(name)
	if e == nil {
		return fmt.Errorf("unknown process: %s", name)
	}
	alive, _ := e.r.DetectAlive()
	if !alive {
		return nil
	}
	e.r.SetStopRequested(true)
	cmd := e.r.CopyCmd()
	if cmd != nil && cmd.Process != nil {
		pid := cmd.Process.Pid
		_ = syscall.Kill(-pid, syscall.SIGTERM)
		waitCh := e.r.WaitDoneChan()
		select {
		case <-waitCh:
			// monitor observed exit
		case <-time.After(wait):
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			time.Sleep(10 * time.Millisecond)
		}
	}
	e.r.CloseWriters()
	metrics.IncStop(name)
	rs := e.r.Snapshot()
	return rs.ExitErr
}

// Status returns current status including detector check.
func (m *Manager) Status(name string) (process.Status, error) {
	e := m.get(name)
	if e == nil {
		return process.Status{}, fmt.Errorf("unknown process: %s", name)
	}
	alive, by := e.r.DetectAlive()
	rs := e.r.Snapshot()
	st := process.Status{
		Name:       rs.Name,
		Running:    alive,
		PID:        rs.PID,
		StartedAt:  rs.StartedAt,
		StoppedAt:  rs.StoppedAt,
		ExitErr:    rs.ExitErr,
		DetectedBy: by,
		Restarts:   rs.Restarts,
	}
	return st, nil
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
	m.mu.Lock()
	names := make([]string, 0, len(m.procs))
	for name := range m.procs {
		if name == base || strings.HasPrefix(name, base+"-") {
			names = append(names, name)
		}
	}
	m.mu.Unlock()
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
	m.mu.Lock()
	names := make([]string, 0, len(m.procs))
	for name := range m.procs {
		if name == base || strings.HasPrefix(name, base+"-") {
			names = append(names, name)
		}
	}
	m.mu.Unlock()
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
	m.mu.Lock()
	names := make([]string, 0, len(m.procs))
	for name := range m.procs {
		if wildcardMatch(name, pattern) {
			names = append(names, name)
		}
	}
	m.mu.Unlock()
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
	m.mu.Lock()
	names := make([]string, 0, len(m.procs))
	for name := range m.procs {
		if wildcardMatch(name, pattern) {
			names = append(names, name)
		}
	}
	m.mu.Unlock()
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
// getOrCreateEntry returns an entry for the spec name, creating it if missing.
func (m *Manager) getOrCreateEntry(spec process.Spec) *entry {
	m.mu.Lock()
	e, exists := m.procs[spec.Name]
	if !exists {
		r := process.New(spec)
		e = &entry{r: r, spec: spec}
		m.procs[spec.Name] = e
	} else {
		e.spec = spec
		// ensure process has the updated spec too
		e.r.UpdateSpec(spec)
	}
	m.mu.Unlock()
	return e
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

// fastAlive returns true if the process is already reported alive.
func (m *Manager) fastAlive(e *entry) bool {
	alive, _ := e.r.DetectAlive()
	return alive
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

// tryStartOnce configures and tries to start the process once.
func (m *Manager) tryStartOnce(e *entry, spec process.Spec) error {
	mergedEnv := m.mergedEnvFor(spec)
	cmd := e.r.ConfigureCmd(mergedEnv)
	return e.r.TryStart(cmd)
}

// postStart runs metrics, starts monitoring, and enforces start duration.
// Returns error if the process exits before start duration.
func (m *Manager) postStart(e *entry, spec process.Spec) error {
	metrics.IncStart(spec.Name)
	if e.r.MonitoringStartIfNeeded() {
		go m.monitor(e)
	}
	if err := e.r.EnforceStartDuration(spec.StartDuration); err != nil {
		e.r.RemovePIDFile()
		e.r.MarkExited(err)
		return err
	}
	if spec.StartDuration > 0 {
		metrics.ObserveStartDuration(spec.Name, spec.StartDuration.Seconds())
	}
	return nil
}

// ReconcileOnce checks current managed processes against reality and updates the store.
// It also attempts to auto-restart processes that should be running but are not.
func (m *Manager) ReconcileOnce() {
	m.mu.Lock()
	st := m.st
	entries := make([]*entry, 0, len(m.procs))
	for _, e := range m.procs {
		entries = append(entries, e)
	}
	m.mu.Unlock()
	if len(entries) == 0 && st == nil {
		return
	}
	ctx := context.Background()
	for _, e := range entries {
		alive, _ := e.r.DetectAlive()
		rs := e.r.Snapshot()
		if st != nil {
			rec := store.Record{
				Name:      e.spec.Name,
				PID:       rs.PID,
				StartedAt: rs.StartedAt,
				Running:   alive,
			}
			if !alive && !rs.StoppedAt.IsZero() {
				rec.StoppedAt = sql.NullTime{Time: rs.StoppedAt, Valid: true}
			}
			if rs.ExitErr != nil {
				rec.ExitErr = sql.NullString{String: rs.ExitErr.Error(), Valid: true}
			}
			_ = st.UpsertStatus(ctx, rec)
			// If we still think it was running but it's not, ensure a stop record exists.
			if !alive && rs.Running {
				uniq := store.UniqueKey(rs.PID, rs.StartedAt)
				_ = st.RecordStop(ctx, uniq, time.Now().UTC(), fmt.Errorf("lost (reconciler)"))
			}
		}
		// Auto-restart policy enforcement (best-effort)
		if !alive && e.spec.AutoRestart && !e.r.StopRequested() {
			_ = m.Start(e.spec)
		}
	}
}

// StartReconciler starts a background loop that periodically calls ReconcileOnce.
func (m *Manager) StartReconciler(interval time.Duration) {
	if interval <= 0 {
		interval = 2 * time.Second
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
