package runtime

import (
	"context"
	"time"
)

// Clock makes time-dependent control propagation deterministic in tests.
type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}

// FlagReader, PolicyReader, and AuditWriter are intentionally narrow seams. Their
// domain-specific request and response contracts arrive in later implementation tasks.
type FlagReader interface {
	Snapshot(context.Context) (string, error)
}

type PolicyReader interface {
	Version(context.Context) (string, error)
}

type AuditWriter interface {
	Append(context.Context, string) error
}

type Dependencies struct {
	Audit  AuditWriter
	Clock  Clock
	Flags  FlagReader
	Policy PolicyReader
}

func (d Dependencies) WithDefaults() Dependencies {
	if d.Clock == nil {
		d.Clock = SystemClock{}
	}
	return d
}
