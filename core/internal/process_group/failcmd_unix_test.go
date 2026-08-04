//go:build !windows

package process_group

// failCommand returns a command that starts and exits with a non-zero status
// almost immediately.
func failCommand() string {
	return "false"
}
