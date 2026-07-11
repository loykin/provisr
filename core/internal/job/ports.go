package job

import (
	"time"

	"github.com/loykin/provisr/core/internal/process"
	"github.com/loykin/provisr/core/observability"
)

// ProcessRunner is the minimum process-management capability required by a
// Job. The concrete process manager is an adapter for this port.
type ProcessRunner interface {
	RegisterN(process.Spec) error
	Stop(string, time.Duration) error
	Status(string) (process.Status, error)
	Unregister(string, time.Duration) error
	Observe(observability.Event)
}
