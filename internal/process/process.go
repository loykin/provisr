package process

import (
	"io"
	"os/exec"
	"sync"

	"github.com/loykin/provisr/internal/detector"
)

type proc struct {
	spec      Spec
	cmd       *exec.Cmd
	status    Status
	mu        sync.Mutex
	stopping  bool // true when Stop has been requested; suppress autorestart
	restarts  int
	outCloser io.WriteCloser
	errCloser io.WriteCloser
}

func (p *proc) detectors() []detector.Detector {
	dets := make([]detector.Detector, 0, len(p.spec.Detectors)+1)
	if p.spec.PIDFile != "" {
		dets = append(dets, detector.PIDFileDetector{PIDFile: p.spec.PIDFile})
	}
	// Note: we avoid using a plain PID-based detector tied to p.cmd to prevent PID reuse false positives.
	dets = append(dets, p.spec.Detectors...)
	return dets
}

func (p *proc) detectAlive() (bool, string) {
	// If we already marked it stopped, trust that state.
	if !p.status.Running {
		return false, ""
	}
	// If we started the command and it hasn't exited yet (non-blocking check), consider it running.
	if p.cmd != nil {
		if p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited() {
			return false, ""
		}
		// Non-blocking reap to detect natural exit
		if tryReap(p) {
			return false, ""
		}
		return true, "exec:pid"
	}
	for _, d := range p.detectors() {
		ok, _ := d.Alive()
		if ok {
			return true, d.Describe()
		}
	}
	return false, ""
}
