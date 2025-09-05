package process

import (
	"fmt"
	"syscall"
	"time"
)

// tryReap performs a non-blocking wait on the child process to detect if it exited.
// Returns true if the process has exited and status was updated.
func tryReap(p *proc) bool {
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	var ws syscall.WaitStatus
	pid, err := syscall.Wait4(p.cmd.Process.Pid, &ws, syscall.WNOHANG, nil)
	if err != nil || pid == 0 {
		return false
	}
	// mark as stopped
	p.status.Running = false
	p.status.StoppedAt = time.Now()
	if ws.Exited() && ws.ExitStatus() == 0 {
		p.status.ExitErr = nil
	} else {
		p.status.ExitErr = fmt.Errorf("exit status: %v", ws)
	}
	return true
}
