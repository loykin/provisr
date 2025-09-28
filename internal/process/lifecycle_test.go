package process

import (
	"strings"
	"testing"
	"time"
)

func TestHook_Validate(t *testing.T) {
	tests := []struct {
		name    string
		hook    Hook
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid hook",
			hook: Hook{
				Name:        "setup-db",
				Command:     "echo 'setting up database'",
				WorkDir:     "/app",
				Env:         []string{"DB_HOST=localhost"},
				Timeout:     30 * time.Second,
				FailureMode: FailureModeFail,
				RunMode:     RunModeBlocking,
			},
			wantErr: false,
		},
		{
			name: "empty name",
			hook: Hook{
				Command: "echo test",
			},
			wantErr: true,
			errMsg:  "hook name is required",
		},
		{
			name: "invalid name with spaces",
			hook: Hook{
				Name:    "setup db",
				Command: "echo test",
			},
			wantErr: true,
			errMsg:  "name contains invalid characters",
		},
		{
			name: "invalid name with path separator",
			hook: Hook{
				Name:    "setup/db",
				Command: "echo test",
			},
			wantErr: true,
			errMsg:  "name contains invalid characters",
		},
		{
			name: "empty command",
			hook: Hook{
				Name: "test",
			},
			wantErr: true,
			errMsg:  "requires command",
		},
		{
			name: "command too long",
			hook: Hook{
				Name:    "test",
				Command: strings.Repeat("a", 10001),
			},
			wantErr: true,
			errMsg:  "command too long",
		},
		{
			name: "invalid failure mode",
			hook: Hook{
				Name:        "test",
				Command:     "echo test",
				FailureMode: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid failure_mode",
		},
		{
			name: "invalid run mode",
			hook: Hook{
				Name:    "test",
				Command: "echo test",
				RunMode: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid run_mode",
		},
		{
			name: "negative timeout",
			hook: Hook{
				Name:    "test",
				Command: "echo test",
				Timeout: -1 * time.Second,
			},
			wantErr: true,
			errMsg:  "timeout cannot be negative",
		},
		{
			name: "timeout too long",
			hook: Hook{
				Name:    "test",
				Command: "echo test",
				Timeout: 2 * time.Hour,
			},
			wantErr: true,
			errMsg:  "timeout too long",
		},
		{
			name: "work_dir with path traversal",
			hook: Hook{
				Name:    "test",
				Command: "echo test",
				WorkDir: "../etc",
			},
			wantErr: true,
			errMsg:  "work_dir cannot contain '..' path traversal",
		},
		{
			name: "empty work_dir",
			hook: Hook{
				Name:    "test",
				Command: "echo test",
				WorkDir: "   ",
			},
			wantErr: true,
			errMsg:  "work_dir cannot be empty string or whitespace",
		},
		{
			name: "invalid env format",
			hook: Hook{
				Name:    "test",
				Command: "echo test",
				Env:     []string{"INVALID_ENV"},
			},
			wantErr: true,
			errMsg:  "must be in KEY=VALUE format",
		},
		{
			name: "empty env key",
			hook: Hook{
				Name:    "test",
				Command: "echo test",
				Env:     []string{"=value"},
			},
			wantErr: true,
			errMsg:  "has empty key",
		},
		{
			name: "reserved env key",
			hook: Hook{
				Name:    "test",
				Command: "echo test",
				Env:     []string{"PROVISR_SECRET=value"},
			},
			wantErr: true,
			errMsg:  "is reserved (PROVISR_ prefix)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.hook.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Hook.Validate() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Hook.Validate() error = %v, want to contain %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Hook.Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestHook_GetDefaults(t *testing.T) {
	hook := &Hook{
		Name:    "test",
		Command: "echo test",
	}

	hook.GetDefaults()

	if hook.FailureMode != FailureModeFail {
		t.Errorf("GetDefaults() FailureMode = %v, want %v", hook.FailureMode, FailureModeFail)
	}
	if hook.RunMode != RunModeBlocking {
		t.Errorf("GetDefaults() RunMode = %v, want %v", hook.RunMode, RunModeBlocking)
	}
	if hook.Timeout != 30*time.Second {
		t.Errorf("GetDefaults() Timeout = %v, want %v", hook.Timeout, 30*time.Second)
	}
}

func TestLifecycleHooks_Validate(t *testing.T) {
	tests := []struct {
		name    string
		hooks   LifecycleHooks
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid hooks",
			hooks: LifecycleHooks{
				PreStart: []Hook{
					{Name: "setup", Command: "echo setup"},
				},
				PostStart: []Hook{
					{Name: "notify", Command: "echo notify"},
				},
			},
			wantErr: false,
		},
		{
			name:    "empty hooks",
			hooks:   LifecycleHooks{},
			wantErr: false,
		},
		{
			name: "duplicate hook names",
			hooks: LifecycleHooks{
				PreStart: []Hook{
					{Name: "setup", Command: "echo setup1"},
				},
				PostStart: []Hook{
					{Name: "setup", Command: "echo setup2"},
				},
			},
			wantErr: true,
			errMsg:  "duplicate hook name",
		},
		{
			name: "invalid hook in pre_start",
			hooks: LifecycleHooks{
				PreStart: []Hook{
					{Name: "", Command: "echo test"},
				},
			},
			wantErr: true,
			errMsg:  "pre_start hook 0 validation failed",
		},
		{
			name: "too many hooks in one phase",
			hooks: LifecycleHooks{
				PreStart: make([]Hook, 51),
			},
			wantErr: true,
			errMsg:  "pre_start phase has too many hooks",
		},
	}

	// Initialize hooks for "too many hooks" test
	for i := 0; i < 51; i++ {
		tests[4].hooks.PreStart[i] = Hook{
			Name:    "hook" + string(rune('a'+i%26)) + string(rune('0'+i/26)),
			Command: "echo test",
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.hooks.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("LifecycleHooks.Validate() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("LifecycleHooks.Validate() error = %v, want to contain %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("LifecycleHooks.Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestLifecycleHooks_HasAnyHooks(t *testing.T) {
	tests := []struct {
		name  string
		hooks LifecycleHooks
		want  bool
	}{
		{
			name:  "no hooks",
			hooks: LifecycleHooks{},
			want:  false,
		},
		{
			name: "has pre_start hooks",
			hooks: LifecycleHooks{
				PreStart: []Hook{{Name: "test", Command: "echo test"}},
			},
			want: true,
		},
		{
			name: "has post_stop hooks",
			hooks: LifecycleHooks{
				PostStop: []Hook{{Name: "cleanup", Command: "echo cleanup"}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.hooks.HasAnyHooks(); got != tt.want {
				t.Errorf("LifecycleHooks.HasAnyHooks() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLifecycleHooks_GetHooksForPhase(t *testing.T) {
	hooks := LifecycleHooks{
		PreStart:  []Hook{{Name: "pre1", Command: "echo pre1"}},
		PostStart: []Hook{{Name: "post1", Command: "echo post1"}},
		PreStop:   []Hook{{Name: "prestop1", Command: "echo prestop1"}},
		PostStop:  []Hook{{Name: "poststop1", Command: "echo poststop1"}},
	}

	tests := []struct {
		phase    LifecyclePhase
		expected int
		hookName string
	}{
		{PhasePreStart, 1, "pre1"},
		{PhasePostStart, 1, "post1"},
		{PhasePreStop, 1, "prestop1"},
		{PhasePostStop, 1, "poststop1"},
		{"invalid", 0, ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			result := hooks.GetHooksForPhase(tt.phase)
			if len(result) != tt.expected {
				t.Errorf("GetHooksForPhase(%v) returned %d hooks, want %d", tt.phase, len(result), tt.expected)
			}
			if tt.expected > 0 && result[0].Name != tt.hookName {
				t.Errorf("GetHooksForPhase(%v) returned hook name %v, want %v", tt.phase, result[0].Name, tt.hookName)
			}
		})
	}
}

func TestLifecycleHooks_DeepCopy(t *testing.T) {
	original := LifecycleHooks{
		PreStart: []Hook{
			{
				Name:    "setup",
				Command: "echo setup",
				Env:     []string{"KEY=value"},
			},
		},
		PostStart: []Hook{
			{
				Name:    "notify",
				Command: "echo notify",
			},
		},
	}

	copied := original.DeepCopy()

	// Verify the copy is independent
	copied.PreStart[0].Name = "modified"
	copied.PreStart[0].Env[0] = "MODIFIED=value"

	if original.PreStart[0].Name == "modified" {
		t.Error("DeepCopy() did not create independent copy of Name")
	}
	if original.PreStart[0].Env[0] == "MODIFIED=value" {
		t.Error("DeepCopy() did not create independent copy of Env slice")
	}

	// Verify structure is preserved
	if len(copied.PreStart) != 1 {
		t.Errorf("DeepCopy() PreStart length = %d, want 1", len(copied.PreStart))
	}
	if len(copied.PostStart) != 1 {
		t.Errorf("DeepCopy() PostStart length = %d, want 1", len(copied.PostStart))
	}
}

func TestHook_DeepCopy(t *testing.T) {
	original := Hook{
		Name:        "test",
		Command:     "echo test",
		WorkDir:     "/app",
		Env:         []string{"KEY1=value1", "KEY2=value2"},
		Timeout:     30 * time.Second,
		FailureMode: FailureModeFail,
		RunMode:     RunModeBlocking,
	}

	copied := original.DeepCopy()

	// Modify the copy
	copied.Name = "modified"
	copied.Env[0] = "MODIFIED=value"

	// Verify original is unchanged
	if original.Name == "modified" {
		t.Error("DeepCopy() did not create independent copy of Name")
	}
	if original.Env[0] == "MODIFIED=value" {
		t.Error("DeepCopy() did not create independent copy of Env slice")
	}

	// Verify all fields are copied correctly
	if copied.Command != original.Command {
		t.Error("DeepCopy() did not copy Command correctly")
	}
	if copied.WorkDir != original.WorkDir {
		t.Error("DeepCopy() did not copy WorkDir correctly")
	}
	if copied.Timeout != original.Timeout {
		t.Error("DeepCopy() did not copy Timeout correctly")
	}
	if copied.FailureMode != original.FailureMode {
		t.Error("DeepCopy() did not copy FailureMode correctly")
	}
	if copied.RunMode != original.RunMode {
		t.Error("DeepCopy() did not copy RunMode correctly")
	}
}

func TestLifecyclePhase_String(t *testing.T) {
	tests := []struct {
		phase    LifecyclePhase
		expected string
	}{
		{PhasePreStart, "pre_start"},
		{PhasePostStart, "post_start"},
		{PhasePreStop, "pre_stop"},
		{PhasePostStop, "post_stop"},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			if got := tt.phase.String(); got != tt.expected {
				t.Errorf("LifecyclePhase.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}
