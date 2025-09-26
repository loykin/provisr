package process_group

import (
	"fmt"
	"time"

	"github.com/loykin/provisr/internal/manager"
	"github.com/loykin/provisr/internal/process"
)

// GroupSpec defines a group of processes to be managed together.
// Each member is a full process.Spec; Instances is honored per member.
// Name is a logical group identifier used for diagnostics only.
type GroupSpec struct {
	Name    string
	Members []process.Spec
}

// Group provides start/stop/status operations over a set of processes
// using an underlying process.Manager.

type Group struct {
	mgr *manager.Manager
}

func New(mgr *manager.Manager) *Group { return &Group{mgr: mgr} }

// Start starts all members. If any start fails, it stops any members that
// have already been started in this call and returns the error.
func (g *Group) Start(gs GroupSpec) error {
	started := make([]process.Spec, 0, len(gs.Members))
	for _, m := range gs.Members {
		var err error
		if m.Instances > 1 {
			err = g.mgr.RegisterN(m)
		} else {
			err = g.mgr.Register(m)
		}
		if err != nil {
			// rollback: stop previously started members
			for i := len(started) - 1; i >= 0; i-- {
				_ = g.mgr.StopAll(started[i].Name, 2*time.Second)
			}
			return fmt.Errorf("group %s start failed on %s: %w", gs.Name, m.Name, err)
		}
		started = append(started, m)
	}
	return nil
}

// Stop stops all members regardless of their state, best-effort.
// Returns the first error encountered.
func (g *Group) Stop(gs GroupSpec, wait time.Duration) error {
	var firstErr error
	for _, m := range gs.Members {
		if err := g.mgr.StopAll(m.Name, wait); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Status returns a map of member base name to its instance statuses.
func (g *Group) Status(gs GroupSpec) (map[string][]process.Status, error) {
	res := make(map[string][]process.Status, len(gs.Members))
	for _, m := range gs.Members {
		sts, err := g.mgr.StatusAll(m.Name)
		if err != nil {
			return nil, err
		}
		res[m.Name] = sts
	}
	return res, nil
}
