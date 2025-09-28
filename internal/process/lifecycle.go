package process

import (
	"fmt"
	"strings"
	"time"
)

// LifecycleHooks defines hooks that run at different stages of process lifecycle
type LifecycleHooks struct {
	PreStart  []Hook `json:"pre_start" mapstructure:"pre_start"`   // Before process starts
	PostStart []Hook `json:"post_start" mapstructure:"post_start"` // After process starts successfully
	PreStop   []Hook `json:"pre_stop" mapstructure:"pre_stop"`     // Before process stops
	PostStop  []Hook `json:"post_stop" mapstructure:"post_stop"`   // After process stops
}

// Hook represents a single lifecycle hook command
type Hook struct {
	Name        string        `json:"name" mapstructure:"name"`                 // Hook name for identification
	Command     string        `json:"command" mapstructure:"command"`           // Command to execute
	WorkDir     string        `json:"work_dir" mapstructure:"work_dir"`         // Working directory (optional)
	Env         []string      `json:"env" mapstructure:"env"`                   // Additional environment variables
	Timeout     time.Duration `json:"timeout" mapstructure:"timeout"`           // Execution timeout (default: 30s)
	FailureMode FailureMode   `json:"failure_mode" mapstructure:"failure_mode"` // How to handle failures
	RunMode     RunMode       `json:"run_mode" mapstructure:"run_mode"`         // Blocking or async execution
}

// FailureMode defines how to handle hook execution failures
type FailureMode string

const (
	FailureModeIgnore FailureMode = "ignore" // Continue regardless of hook failure
	FailureModeFail   FailureMode = "fail"   // Fail the entire operation if hook fails
	FailureModeRetry  FailureMode = "retry"  // Retry the hook on failure
)

// RunMode defines how hooks are executed
type RunMode string

const (
	RunModeBlocking RunMode = "blocking" // Wait for hook completion before continuing
	RunModeAsync    RunMode = "async"    // Start hook and continue immediately
)

// Validate validates the lifecycle hooks configuration
func (lh *LifecycleHooks) Validate() error {
	// Check for duplicate hook names across all phases
	hookNames := make(map[string]string) // name -> phase

	phases := map[string][]Hook{
		"pre_start":  lh.PreStart,
		"post_start": lh.PostStart,
		"pre_stop":   lh.PreStop,
		"post_stop":  lh.PostStop,
	}

	for phase, hooks := range phases {
		for i, hook := range hooks {
			// Validate individual hook
			if err := hook.Validate(); err != nil {
				return fmt.Errorf("%s hook %d validation failed: %w", phase, i, err)
			}

			// Check for duplicate names
			if existingPhase, exists := hookNames[hook.Name]; exists {
				return fmt.Errorf("duplicate hook name %q found in %s and %s phases", hook.Name, existingPhase, phase)
			}
			hookNames[hook.Name] = phase
		}

		// Check phase-specific constraints
		if len(hooks) > 50 {
			return fmt.Errorf("%s phase has too many hooks (%d), maximum is 50", phase, len(hooks))
		}
	}

	// Validate logical constraints
	totalHooks := len(lh.PreStart) + len(lh.PostStart) + len(lh.PreStop) + len(lh.PostStop)
	if totalHooks > 100 {
		return fmt.Errorf("total hooks count %d exceeds maximum of 100", totalHooks)
	}

	// Warn about potentially problematic patterns
	if len(lh.PreStart) > 10 {
		// This is not an error, but could indicate complex setup
	}

	return nil
}

// Validate validates a single hook configuration
func (h *Hook) Validate() error {
	// Name is required for identification
	name := strings.TrimSpace(h.Name)
	if name == "" {
		return fmt.Errorf("hook name is required")
	}

	// Name should not contain special characters that could cause issues
	if strings.ContainsAny(name, " \t\n\r/\\<>:\"|?*") {
		return fmt.Errorf("hook %q: name contains invalid characters (spaces, tabs, path separators, or special chars)", name)
	}

	// Command is required
	if strings.TrimSpace(h.Command) == "" {
		return fmt.Errorf("hook %q requires command", name)
	}

	// Command should not be excessively long
	if len(h.Command) > 10000 {
		return fmt.Errorf("hook %q: command too long (max 10000 characters)", name)
	}

	// Validate failure mode
	switch h.FailureMode {
	case "", FailureModeIgnore, FailureModeFail, FailureModeRetry:
		// Valid
	default:
		return fmt.Errorf("hook %q: invalid failure_mode %q, must be one of: ignore, fail, retry", name, h.FailureMode)
	}

	// Validate run mode
	switch h.RunMode {
	case "", RunModeBlocking, RunModeAsync:
		// Valid
	default:
		return fmt.Errorf("hook %q: invalid run_mode %q, must be one of: blocking, async", name, h.RunMode)
	}

	// Timeout should be positive if specified
	if h.Timeout < 0 {
		return fmt.Errorf("hook %q: timeout cannot be negative", name)
	}

	// Timeout should not be excessively long (max 1 hour)
	if h.Timeout > time.Hour {
		return fmt.Errorf("hook %q: timeout too long (max 1 hour)", name)
	}

	// Validate working directory if specified
	if h.WorkDir != "" {
		workDir := strings.TrimSpace(h.WorkDir)
		if workDir == "" {
			return fmt.Errorf("hook %q: work_dir cannot be empty string or whitespace", name)
		}
		// Check for potentially dangerous paths
		if strings.Contains(workDir, "..") {
			return fmt.Errorf("hook %q: work_dir cannot contain '..' path traversal", name)
		}
	}

	// Validate environment variables
	for i, env := range h.Env {
		if !strings.Contains(env, "=") {
			return fmt.Errorf("hook %q: env[%d] %q is invalid, must be in KEY=VALUE format", name, i, env)
		}

		parts := strings.SplitN(env, "=", 2)
		key := strings.TrimSpace(parts[0])
		if key == "" {
			return fmt.Errorf("hook %q: env[%d] has empty key", name, i)
		}

		// Check for reserved environment variables
		if strings.HasPrefix(key, "PROVISR_") {
			return fmt.Errorf("hook %q: env[%d] key %q is reserved (PROVISR_ prefix)", name, i, key)
		}
	}

	return nil
}

// GetDefaults applies default values to hook configuration
func (h *Hook) GetDefaults() {
	if h.FailureMode == "" {
		h.FailureMode = FailureModeFail // Default to failing on hook errors
	}

	if h.RunMode == "" {
		h.RunMode = RunModeBlocking // Default to blocking execution
	}

	if h.Timeout == 0 {
		h.Timeout = 30 * time.Second // Default 30 second timeout
	}
}

// HasAnyHooks returns true if there are any hooks defined
func (lh *LifecycleHooks) HasAnyHooks() bool {
	return len(lh.PreStart) > 0 || len(lh.PostStart) > 0 || len(lh.PreStop) > 0 || len(lh.PostStop) > 0
}

// GetHooksForPhase returns hooks for a specific lifecycle phase
func (lh *LifecycleHooks) GetHooksForPhase(phase LifecyclePhase) []Hook {
	switch phase {
	case PhasePreStart:
		return lh.PreStart
	case PhasePostStart:
		return lh.PostStart
	case PhasePreStop:
		return lh.PreStop
	case PhasePostStop:
		return lh.PostStop
	default:
		return nil
	}
}

// LifecyclePhase represents different phases of process lifecycle
type LifecyclePhase string

const (
	PhasePreStart  LifecyclePhase = "pre_start"
	PhasePostStart LifecyclePhase = "post_start"
	PhasePreStop   LifecyclePhase = "pre_stop"
	PhasePostStop  LifecyclePhase = "post_stop"
)

// String returns the string representation of the lifecycle phase
func (p LifecyclePhase) String() string {
	return string(p)
}

// DeepCopy creates a deep copy of LifecycleHooks
func (lh *LifecycleHooks) DeepCopy() LifecycleHooks {
	if lh == nil {
		return LifecycleHooks{}
	}

	hooks := LifecycleHooks{
		PreStart:  copyHooks(lh.PreStart),
		PostStart: copyHooks(lh.PostStart),
		PreStop:   copyHooks(lh.PreStop),
		PostStop:  copyHooks(lh.PostStop),
	}

	return hooks
}

// copyHooks creates a deep copy of a slice of hooks
func copyHooks(hooks []Hook) []Hook {
	if hooks == nil {
		return nil
	}

	copied := make([]Hook, len(hooks))
	for i, hook := range hooks {
		copied[i] = hook.DeepCopy()
	}

	return copied
}

// DeepCopy creates a deep copy of a Hook
func (h *Hook) DeepCopy() Hook {
	hook := Hook{
		Name:        h.Name,
		Command:     h.Command,
		WorkDir:     h.WorkDir,
		Timeout:     h.Timeout,
		FailureMode: h.FailureMode,
		RunMode:     h.RunMode,
	}

	// Copy environment variables slice
	if h.Env != nil {
		hook.Env = append([]string(nil), h.Env...)
	}

	return hook
}
