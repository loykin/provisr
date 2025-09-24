//go:build windows

package process

import (
	"os/exec"
	"syscall"
)

// Windows creation flags
const (
	CREATE_NEW_PROCESS_GROUP = 0x00000200
	DETACHED_PROCESS         = 0x00000008
)

// configureSysProcAttr sets platform-specific attributes for Windows.
// For signal handling, we create a new process group. When Detached is true,
// we additionally set DETACHED_PROCESS so the child does not inherit the
// parent's console and is fully detached.
func configureSysProcAttr(cmd *exec.Cmd, spec Spec) {
	attrs := &syscall.SysProcAttr{}
	flags := uint32(CREATE_NEW_PROCESS_GROUP)
	if spec.Detached {
		flags |= DETACHED_PROCESS
	}
	attrs.CreationFlags = flags
	cmd.SysProcAttr = attrs
}
