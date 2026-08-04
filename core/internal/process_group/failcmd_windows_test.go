//go:build windows

package process_group

// failCommand returns a command that starts and exits with a non-zero status
// almost immediately. Unlike "false" (which has no native Windows binary, so
// cmd.exe has to search PATH and fail to find it — a search that can take
// long enough on some CI runners to race past EnforceStartDuration's poll
// window and be mistaken for a process that stayed up), "exit /b 1" is a
// cmd.exe builtin with no external executable to look up.
func failCommand() string {
	return "cmd /c exit /b 1"
}
