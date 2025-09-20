package process

import (
	"strings"
	"testing"
	"time"
)

// FuzzBuildCommand tests command building with various inputs
func FuzzBuildCommand(f *testing.F) {
	// Seed with interesting cases
	f.Add("echo hello")
	f.Add("/bin/sh -c 'echo test'")
	f.Add("sh -c echo hi")
	f.Add("/usr/bin/env bash -c 'ls -la'")
	f.Add("")
	f.Add("command with spaces")
	f.Add("'single quoted'")
	f.Add(`"double quoted"`)
	f.Add("command\nwith\nnewlines")
	f.Add("command\twith\ttabs")

	f.Fuzz(func(t *testing.T, cmdStr string) {
		// Limit input size to prevent resource exhaustion
		if len(cmdStr) > 2000 {
			t.Skip("command too long")
		}

		spec := Spec{
			Name:    "test-process",
			Command: cmdStr,
		}

		// Test BuildCommand - should not panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("BuildCommand panicked: %v", r)
				}
			}()
			cmd := spec.BuildCommand()

			// Basic sanity checks on the result
			if cmd == nil {
				// Empty commands might return nil, which is acceptable
				if cmdStr != "" {
					t.Log("BuildCommand returned nil for non-empty command")
				}
				return
			}

			// Verify the command has some basic properties
			if cmd.Path == "" && len(cmd.Args) == 0 {
				t.Error("BuildCommand returned cmd with no path and no args")
			}

			// Check for obvious command injection attempts
			if containsSuspiciousPatterns(cmdStr) {
				// Just log for analysis, don't fail - the spec parsing should handle this
				t.Logf("suspicious pattern in command: %s", cmdStr)
			}
		}()
	})
}

// FuzzSpecValidation tests spec validation edge cases
func FuzzSpecValidation(f *testing.F) {
	// Seed with various spec configurations
	f.Add("proc1", "echo test", "/tmp", int64(1000), int64(500), int64(1000), true, int64(1))
	f.Add("", "", "", int64(0), int64(0), int64(0), false, int64(0))
	f.Add("../proc", "rm -rf /", "/", int64(-1), int64(-1), int64(-1), true, int64(100))

	f.Fuzz(func(t *testing.T, name, command, workdir string,
		startDur, retryInterval, restartInterval int64, autoRestart bool, instances int64) {

		// Limit input sizes
		if len(name) > 200 || len(command) > 1000 || len(workdir) > 500 {
			t.Skip("input too long")
		}

		// Convert int64 to reasonable ranges
		startDurTime := time.Duration(startDur%10000) * time.Millisecond
		retryIntervalTime := time.Duration(retryInterval%10000) * time.Millisecond
		restartIntervalTime := time.Duration(restartInterval%10000) * time.Millisecond
		instancesInt := int(instances % 10)
		if instancesInt < 0 {
			instancesInt = 1
		}

		spec := Spec{
			Name:            name,
			Command:         command,
			WorkDir:         workdir,
			StartDuration:   startDurTime,
			RetryInterval:   retryIntervalTime,
			RestartInterval: restartIntervalTime,
			AutoRestart:     autoRestart,
			Instances:       instancesInt,
		}

		// Test that creating a process with this spec doesn't crash
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("New panicked with spec %+v: %v", spec, r)
				}
			}()

			proc := New(spec)
			if proc == nil {
				t.Error("New returned nil process")
				return
			}

			// Test basic operations don't crash
			_ = proc.Snapshot()
			// Name might not be set until the process starts running
			// For fuzzing, we mainly care about no crashes

			// Test other safe operations
			proc.StopRequested()
		}()
	})
}

// FuzzParseExplicitShell tests shell parsing logic
func FuzzParseExplicitShell(f *testing.F) {
	// Seed with shell command patterns
	f.Add("sh -c echo hello")
	f.Add("/bin/sh -c 'ls -la'")
	f.Add("bash -c 'echo $HOME'")
	f.Add("/usr/bin/env sh -c test")
	f.Add("not-shell command")
	f.Add("sh-not-shell")
	f.Add("sh -c")
	f.Add("sh -c ''")

	f.Fuzz(func(t *testing.T, cmdStr string) {
		if len(cmdStr) > 500 {
			t.Skip("command too long")
		}

		// Test parseExplicitShell - should not panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("parseExplicitShell panicked: %v", r)
				}
			}()

			shellPath, afterCArg, matched := parseExplicitShell(cmdStr)

			// Basic validation of results
			if matched {
				if shellPath == "" {
					t.Error("matched but shellPath is empty")
				}
				// afterCArg can be empty (e.g., "sh -c ''")

				// Verify the parsing makes sense
				if !strings.Contains(cmdStr, "-c") {
					t.Errorf("matched shell pattern but no -c in command: %s", cmdStr)
				}
			} else {
				// If not matched, outputs should be empty
				if shellPath != "" || afterCArg != "" {
					t.Errorf("not matched but outputs not empty: shell=%s, after=%s", shellPath, afterCArg)
				}
			}

			// Test that the parsing is consistent
			if matched {
				// Re-parse should give same result
				shellPath2, afterCArg2, matched2 := parseExplicitShell(cmdStr)
				if shellPath != shellPath2 || afterCArg != afterCArg2 || matched != matched2 {
					t.Errorf("inconsistent parsing results")
				}
			}
		}()
	})
}

// Helper functions

func containsSuspiciousPatterns(cmd string) bool {
	suspicious := []string{
		";rm", ";del", "|rm", "|del",
		"&&rm", "&&del", "||rm", "||del",
		">`", ">|", "&>", "2>",
		"$(", "`", "${",
		"/../", "\\..\\",
	}

	cmdLower := strings.ToLower(cmd)
	for _, pattern := range suspicious {
		if strings.Contains(cmdLower, pattern) {
			return true
		}
	}

	return false
}
