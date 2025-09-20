# Manager Package

The `manager` package provides a simplified, race-condition-free process management system that replaces the complex Manager-Handler-Supervisor triple architecture with a clean two-layer design.

## Architecture Overview

### Before: Complex Triple Architecture
```
Manager
  ├── Handler (per process group)
  │   └── Supervisor (per process instance)
  │       └── Process (OS process wrapper)
  └── Complex locking hierarchy with race conditions
```

### After: Simplified Two-Layer Architecture
```
Manager (process container)
  └── ManagedProcess (state machine wrapper)
      └── process.Process (OS process)
```

## Core Components

### 1. Manager (`manager.go`)
The high-level process container that manages multiple process instances.

**Responsibilities:**
- Process lifecycle management (start, stop, status)
- Pattern-based operations (wildcards, multiple instances)
- Global environment variable management
- Store and history sink integration
- Background reconciliation

**Key Methods:**
- `Start(spec)` - Start a single process
- `StartN(spec)` - Start N instances of a process
- `Stop(name, timeout)` - Stop a specific process
- `StopAll(pattern, timeout)` - Stop processes matching pattern
- `Status(name)` - Get process status
- `StatusAll(pattern)` - Get status of processes matching pattern
- `Count(pattern)` - Count running processes matching pattern

### 2. ManagedProcess (`managed_process.go`)
A state machine-based wrapper that manages a single process instance.

**State Machine:**
```
Stopped → Starting → Running → Stopping → Stopped
    ↑                              ↓
    └──────── Auto-restart ←───────┘
```

**Responsibilities:**
- Process state management with atomic operations
- Race-condition-free state transitions
- Process monitoring and health checks
- Restart logic and failure handling
- Integration with external logging/monitoring

## Lock Hierarchy

Clear two-level locking prevents deadlocks:

1. **Manager Level** (`Manager.mu`):
   - Protects the processes map
   - Guards shared resources (store, sinks, env)
   - Controls reconciler lifecycle

2. **Process Level** (`ManagedProcess.mu`):
   - Protects individual process state
   - Guards process-specific resources
   - Never acquired while holding Manager.mu

## Concurrency Safety

### Race Condition Prevention
- **State Machine**: Single goroutine processes state changes
- **Atomic Counters**: Restart counts use `sync/atomic`
- **Channel Communication**: Lock-free command passing
- **Clear Hierarchy**: Prevents circular dependencies

### Thread Safety
- All public methods are thread-safe
- Internal state protected by appropriate mutexes
- No shared mutable state between components

## Process Lifecycle

### 1. Process Creation
```go
mgr := NewManager()
spec := process.Spec{
    Name:    "web-server",
    Command: "nginx -g 'daemon off;'",
    Env:     []string{"PORT=8080"},
}
err := mgr.Start(spec)
```

### 2. State Monitoring
```go
status, err := mgr.Status("web-server")
if status.Running {
    fmt.Printf("Process PID: %d", status.PID)
}
```

### 3. Pattern Operations
```go
// Start multiple instances
spec.Instances = 3
mgr.StartN(spec) // Creates web-server-1, web-server-2, web-server-3

// Stop all instances
mgr.StopAll("web-server*", 10*time.Second)

// Get status of all instances
statuses, _ := mgr.StatusAll("web-server*")
```

### 4. Graceful Shutdown
```go
mgr.Shutdown() // Stops reconciler and all processes
```

## Integration Points

### Environment Management
```go
mgr.SetGlobalEnv([]string{
    "NODE_ENV=production",
    "LOG_LEVEL=info",
})
```

### Persistence Layer
```go
store := sqlite.New("processes.db")
mgr.SetStore(store)
// Process events automatically persisted
```

### History Tracking
```go
sink := opensearch.New(config)
mgr.SetHistorySinks(sink)
// Process lifecycle events streamed to external systems
```

### Background Reconciliation
```go
mgr.StartReconciler(500 * time.Millisecond)
// Periodic state synchronization with store
```

## State Management

### Process States
- **Stopped**: Process not running, can be started
- **Starting**: Process is being launched, temporary state
- **Running**: Process is active and healthy
- **Stopping**: Process is being terminated, temporary state

### State Transitions
All state changes go through the internal state machine:
```go
func (mp *ManagedProcess) handleStart(spec process.Spec) error {
    // Atomic state transition: Stopped → Starting
    mp.setState(StateStarting)
    
    // Launch process
    if err := mp.proc.Start(); err != nil {
        mp.setState(StateStopped)
        return err
    }
    
    // Atomic state transition: Starting → Running
    mp.setState(StateRunning)
    return nil
}
```

### Status Synchronization
Process status combines multiple sources:
```go
func (mp *ManagedProcess) Status() process.Status {
    status := mp.proc.Snapshot()           // OS process state
    alive, detector := mp.proc.DetectAlive() // Health check
    
    status.Running = alive && mp.state == StateRunning // Combined state
    status.Restarts = int(atomic.LoadInt64(&mp.restarts)) // Metrics
    return status
}
```

## Error Handling

### Graceful Degradation
- Process failures don't affect other processes
- Manager continues operating if individual processes fail
- Automatic cleanup of failed process resources

### Retry Logic
- Configurable restart policies
- Exponential backoff for repeated failures
- Maximum retry limits to prevent infinite loops

### Error Propagation
```go
// Start errors are immediately returned
if err := mgr.Start(spec); err != nil {
    // Handle start failure
}

// Stop errors may indicate already-terminated process
if err := mgr.Stop("process", 5*time.Second); err != nil {
    // Process may have already exited
}
```

## Performance Characteristics

### Memory Efficiency
- Minimal goroutine overhead (one per ManagedProcess)
- Shared environment variables across processes
- Pooled resources for common operations

### Scalability
- O(1) process lookup by name
- O(n) pattern matching operations
- Bounded memory usage per process

### Latency
- Sub-millisecond status checks
- Fast process startup/shutdown
- Minimal lock contention

## Testing Strategy

### Unit Tests
- **Manager**: High-level operations and integration
- **ManagedProcess**: State machine and lifecycle
- **Concurrent**: Race condition and stress testing

### Test Coverage Areas
- Process lifecycle (start, run, stop)
- Concurrent operations (multiple goroutines)
- Error conditions (failures, timeouts)
- State transitions (all valid paths)
- Pattern matching (wildcards, edge cases)

### Mock Infrastructure
- MockStore for persistence testing
- MockHistorySink for event tracking
- Deterministic timing for state tests

## Migration Guide

### From Triple Architecture
1. Replace `Handler` references with `Manager`
2. Remove `Supervisor` usage - functionality moved to `ManagedProcess`
3. Update lock usage - simpler hierarchy
4. Migrate custom state logic to new state machine

### API Compatibility
Most existing APIs are preserved:
- `Start()`, `Stop()`, `Status()` unchanged
- Pattern operations (`StopMatch`, `StatusMatch`) available
- Environment and store integration compatible

### Breaking Changes
- Internal handler/supervisor APIs removed
- Some timing behaviors may differ (improved performance)
- Error messages may have changed

## Best Practices

### Process Design
- Use descriptive process names
- Set appropriate timeouts for stop operations
- Configure environment variables at the manager level
- Implement proper health checks in your processes

### Resource Management
- Call `Shutdown()` for graceful cleanup
- Use pattern operations for bulk management
- Monitor process restart counts for health
- Configure appropriate reconciliation intervals

### Error Handling
- Always check errors from start operations
- Handle stop errors gracefully (process may exit naturally)
- Implement retry logic in your application if needed
- Use status checks to verify process health

## Troubleshooting

### Common Issues
1. **Process won't start**: Check command syntax and permissions
2. **Process exits immediately**: Verify environment and working directory
3. **Stop timeouts**: Process may not handle signals properly
4. **High restart count**: Process is crashing repeatedly

### Debugging Tools
```go
// Enable detailed logging
status := mgr.Status("problematic-process")
fmt.Printf("State: running=%v, restarts=%d, pid=%d\n", 
    status.Running, status.Restarts, status.PID)

// Check all processes
statuses, _ := mgr.StatusAll("*")
for _, s := range statuses {
    fmt.Printf("%s: %v\n", s.Name, s.Running)
}
```

### Performance Monitoring
- Monitor restart counts for process stability
- Track start/stop latencies for performance
- Use reconciler logs for state synchronization issues
- Monitor resource usage of managed processes

---

*This architecture provides a clean, safe, and efficient process management system with clear separation of concerns and robust error handling.*