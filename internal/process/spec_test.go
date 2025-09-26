package process

import (
	"runtime"
	"strings"
	"testing"

	"github.com/loykin/provisr/internal/logger"
)

func requireUnixSpec(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix-like shell")
	}
}

// Ensure that when the command string already includes an explicit
// shell invocation (e.g., "sh -c 'echo hi'"), we do not double-wrap
// it with another "/bin/sh -c" layer.
func TestBuildCommand_ExplicitShellNoDoubleWrap(t *testing.T) {
	requireUnixSpec(t)
	s := Spec{Name: "x", Command: "sh -c 'echo hi'"}
	cmd := s.BuildCommand()
	if len(cmd.Args) < 3 {
		t.Fatalf("unexpected argv: %#v", cmd.Args)
	}
	if cmd.Args[1] != "-c" {
		t.Fatalf("expected -c as second arg, got %#v", cmd.Args)
	}
	// The string after -c should be the original script, not another nested shell.
	if strings.HasPrefix(cmd.Args[2], "sh -c ") || strings.HasPrefix(cmd.Args[2], "/bin/sh -c ") {
		t.Fatalf("command was double-wrapped: %q", cmd.Args[2])
	}
}

// Sanity check: when metacharacters are present and no explicit shell prefix
// is provided, we should wrap with /bin/sh -c.
func TestBuildCommand_MetacharTriggersShell(t *testing.T) {
	requireUnixSpec(t)
	s := Spec{Name: "y", Command: "echo hi | wc -c"}
	cmd := s.BuildCommand()
	if len(cmd.Args) < 3 || cmd.Args[1] != "-c" {
		t.Fatalf("expected shell -c wrapping, got argv=%#v", cmd.Args)
	}
}

func TestSpec_Validate(t *testing.T) {
	tests := []struct {
		name        string
		spec        Spec
		expectErr   bool
		errContains string
	}{
		{
			name: "valid spec",
			spec: Spec{
				Name:    "test-process",
				Command: "echo hello",
			},
			expectErr: false,
		},
		{
			name: "empty name",
			spec: Spec{
				Name:    "",
				Command: "echo hello",
			},
			expectErr:   true,
			errContains: "process requires name",
		},
		{
			name: "whitespace only name",
			spec: Spec{
				Name:    "   ",
				Command: "echo hello",
			},
			expectErr:   true,
			errContains: "process requires name",
		},
		{
			name: "empty command",
			spec: Spec{
				Name:    "test-process",
				Command: "",
			},
			expectErr:   true,
			errContains: "requires command",
		},
		{
			name: "whitespace only command",
			spec: Spec{
				Name:    "test-process",
				Command: "   ",
			},
			expectErr:   true,
			errContains: "requires command",
		},
		{
			name: "detached with file logging should fail",
			spec: Spec{
				Name:     "test-process",
				Command:  "echo hello",
				Detached: true,
				Log: logger.Config{
					File: logger.FileConfig{
						Dir: "/tmp/logs",
					},
				},
			},
			expectErr:   true,
			errContains: "detached=true cannot be combined with log outputs",
		},
		{
			name: "detached with stdout path should fail",
			spec: Spec{
				Name:     "test-process",
				Command:  "echo hello",
				Detached: true,
				Log: logger.Config{
					File: logger.FileConfig{
						StdoutPath: "/tmp/stdout.log",
					},
				},
			},
			expectErr:   true,
			errContains: "detached=true cannot be combined with log outputs",
		},
		{
			name: "detached with stderr path should fail",
			spec: Spec{
				Name:     "test-process",
				Command:  "echo hello",
				Detached: true,
				Log: logger.Config{
					File: logger.FileConfig{
						StderrPath: "/tmp/stderr.log",
					},
				},
			},
			expectErr:   true,
			errContains: "detached=true cannot be combined with log outputs",
		},
		{
			name: "detached without file logging should pass",
			spec: Spec{
				Name:     "test-process",
				Command:  "echo hello",
				Detached: true,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestSpec_DeepCopy(t *testing.T) {
	original := &Spec{
		Name:    "test-process",
		Command: "echo hello",
		Env:     []string{"VAR1=value1", "VAR2=value2"},
		DetectorConfigs: []DetectorConfig{
			{Type: "pidfile", Path: "/tmp/test.pid"},
			{Type: "command", Command: "pgrep test"},
		},
		Log: logger.Config{
			File: logger.FileConfig{
				Dir: "/tmp/logs",
			},
		},
	}

	deepCopy := original.DeepCopy()

	if deepCopy == nil {
		t.Fatal("DeepCopy returned nil")
	}

	if deepCopy == original {
		t.Error("DeepCopy returned the same instance")
	}

	if deepCopy.Name != original.Name {
		t.Errorf("Name not copied correctly: got %q, want %q", deepCopy.Name, original.Name)
	}

	if deepCopy.Command != original.Command {
		t.Errorf("Command not copied correctly: got %q, want %q", deepCopy.Command, original.Command)
	}

	if len(deepCopy.Env) != len(original.Env) {
		t.Errorf("Env length mismatch: got %d, want %d", len(deepCopy.Env), len(original.Env))
	}

	deepCopy.Env[0] = "MODIFIED=value"
	if original.Env[0] == "MODIFIED=value" {
		t.Error("Modifying copy.Env affected original")
	}

	if len(deepCopy.DetectorConfigs) != len(original.DetectorConfigs) {
		t.Errorf("DetectorConfigs length mismatch: got %d, want %d", len(deepCopy.DetectorConfigs), len(original.DetectorConfigs))
	}

	deepCopy.DetectorConfigs[0].Type = "modified"
	if original.DetectorConfigs[0].Type == "modified" {
		t.Error("Modifying copy.DetectorConfigs affected original")
	}
}

func TestSpec_DeepCopy_Nil(t *testing.T) {
	var spec *Spec
	deepCopy := spec.DeepCopy()
	if deepCopy != nil {
		t.Errorf("DeepCopy of nil should return nil, got %v", deepCopy)
	}
}

func TestBuildCommand_EmptyCommand(t *testing.T) {
	spec := Spec{
		Name:    "test",
		Command: "",
	}
	cmd := spec.BuildCommand()

	if cmd.Path != "/bin/true" {
		t.Errorf("expected /bin/true for empty command, got %q", cmd.Path)
	}
}

func TestBuildCommand_SimpleCommand(t *testing.T) {
	spec := Spec{
		Name:    "test",
		Command: "ls -la",
	}
	cmd := spec.BuildCommand()

	if !(cmd.Path == "ls" || strings.HasSuffix(cmd.Path, "/ls")) {
		t.Errorf("expected ls or a path ending with /ls, got %q", cmd.Path)
	}

	expected := []string{"ls", "-la"}
	if len(cmd.Args) != len(expected) {
		t.Errorf("expected args %v, got %v", expected, cmd.Args)
	}

	for i, arg := range expected {
		if i >= len(cmd.Args) || cmd.Args[i] != arg {
			t.Errorf("expected arg[%d] = %q, got %q", i, arg, cmd.Args[i])
		}
	}
}

func TestParseExplicitShell(t *testing.T) {
	tests := []struct {
		name           string
		cmdStr         string
		expectedShell  string
		expectedAfter  string
		expectedResult bool
	}{
		{
			name:           "sh -c with single quotes",
			cmdStr:         "sh -c 'echo hello'",
			expectedShell:  "sh",
			expectedAfter:  "echo hello",
			expectedResult: true,
		},
		{
			name:           "sh -c with double quotes",
			cmdStr:         `sh -c "echo hello"`,
			expectedShell:  "sh",
			expectedAfter:  "echo hello",
			expectedResult: true,
		},
		{
			name:           "/bin/sh -c",
			cmdStr:         "/bin/sh -c 'echo hello'",
			expectedShell:  "/bin/sh",
			expectedAfter:  "echo hello",
			expectedResult: true,
		},
		{
			name:           "/usr/bin/sh -c",
			cmdStr:         "/usr/bin/sh -c 'echo hello'",
			expectedShell:  "/usr/bin/sh",
			expectedAfter:  "echo hello",
			expectedResult: true,
		},
		{
			name:           "no quotes",
			cmdStr:         "sh -c echo hello",
			expectedShell:  "sh",
			expectedAfter:  "echo hello",
			expectedResult: true,
		},
		{
			name:           "not shell command",
			cmdStr:         "echo hello",
			expectedShell:  "",
			expectedAfter:  "",
			expectedResult: false,
		},
		{
			name:           "whitespace prefix",
			cmdStr:         "  \tsh -c 'echo hello'",
			expectedShell:  "sh",
			expectedAfter:  "echo hello",
			expectedResult: true,
		},
		{
			name:           "partial match",
			cmdStr:         "bash -c 'echo hello'",
			expectedShell:  "",
			expectedAfter:  "",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shell, after, result := parseExplicitShell(tt.cmdStr)

			if result != tt.expectedResult {
				t.Errorf("expected result %v, got %v", tt.expectedResult, result)
			}

			if shell != tt.expectedShell {
				t.Errorf("expected shell %q, got %q", tt.expectedShell, shell)
			}

			if after != tt.expectedAfter {
				t.Errorf("expected after %q, got %q", tt.expectedAfter, after)
			}
		})
	}
}
